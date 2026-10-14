package backend

import (
	"ant-chrome/backend/internal/logger"
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	browserStderrTailMaxLines = 200
	browserStderrTailMaxBytes = 64 * 1024
)

type browserProcessExitResult struct {
	Err        error
	StderrTail string
	ExitCode   *int
}

type browserProcessMonitor struct {
	cmd          *exec.Cmd
	stderr       io.ReadCloser
	stderrWriter *os.File
	stderrTail   *tailTextBuffer
	stderrInit   chan struct{}
	stderrDone   chan struct{}
	waitDone     chan struct{}

	mu           sync.Mutex
	result       browserProcessExitResult
	debugPort    int
	profileID    string
	runID        string
	stopIntent   string
	stopMethod   string
	kernelBinary string
}

func newBrowserProcessMonitor(cmd *exec.Cmd) (*browserProcessMonitor, error) {
	if cmd == nil {
		return nil, fmt.Errorf("browser command is nil")
	}

	stderr, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	cmd.Stderr = writer
	return &browserProcessMonitor{
		cmd:          cmd,
		stderr:       stderr,
		stderrWriter: writer,
		stderrTail:   newTailTextBuffer(browserStderrTailMaxLines, browserStderrTailMaxBytes),
		stderrInit:   make(chan struct{}),
		stderrDone:   make(chan struct{}),
		waitDone:     make(chan struct{}),
	}, nil
}

func newDetachedBrowserProcessMonitor() *browserProcessMonitor {
	waitDone := make(chan struct{})
	close(waitDone)
	return &browserProcessMonitor{waitDone: waitDone}
}

func (m *browserProcessMonitor) Start() {
	if m.stderrWriter != nil {
		_ = m.stderrWriter.Close()
	}
	go m.captureStderr()
	<-m.stderrInit
	go m.waitForExit()
}

func (m *browserProcessMonitor) Done() <-chan struct{} {
	return m.waitDone
}

func (m *browserProcessMonitor) HasExited() bool {
	select {
	case <-m.waitDone:
		return true
	default:
		return false
	}
}

func (m *browserProcessMonitor) Result() browserProcessExitResult {
	<-m.waitDone

	m.mu.Lock()
	defer m.mu.Unlock()
	result := m.result
	if m.stderrTail != nil {
		result.StderrTail = m.stderrTail.String()
	}
	return result
}

func (m *browserProcessMonitor) Wait() error {
	return m.Result().Err
}

func (m *browserProcessMonitor) DebugPort() (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.debugPort <= 0 {
		return 0, false
	}
	return m.debugPort, true
}

func (m *browserProcessMonitor) SetDebugPort(port int) {
	if port <= 0 {
		return
	}

	m.mu.Lock()
	if m.debugPort <= 0 {
		m.debugPort = port
	}
	m.mu.Unlock()
}

func (m *browserProcessMonitor) captureStderr() {
	defer close(m.stderrDone)

	if m.stderr == nil {
		close(m.stderrInit)
		return
	}
	defer m.stderr.Close()

	scanner := bufio.NewScanner(m.stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	close(m.stderrInit)
	for scanner.Scan() {
		line := scanner.Text()
		m.stderrTail.Append(logger.RedactDiagnosticText(line))
		if strings.Contains(line, "ERROR:") || strings.Contains(line, "FATAL:") || strings.Contains(line, "crash") {
			m.mu.Lock()
			id, run := m.profileID, m.runID
			m.mu.Unlock()
			logger.New("Chromium").Warn("内核 stderr 异常", logger.F("profile_id", id), logger.F("run_id", run), logger.F("pid", m.cmd.Process.Pid), logger.F("event", "kernel.stderr"), logger.F("stderr", logger.RedactDiagnosticText(line)))
		}
		if port, ok := parseBrowserDebugPortFromStderrLine(line); ok {
			m.SetDebugPort(port)
		}
	}
	if err := scanner.Err(); err != nil && !shouldIgnoreBrowserStderrReadError(err) {
		m.stderrTail.Append(fmt.Sprintf("[stderr read error] %v", err))
	}
}

func (m *browserProcessMonitor) closeCapture() {
	if m.stderrWriter != nil {
		m.stderrWriter.Close()
	}
	if m.stderr != nil {
		m.stderr.Close()
	}
}

func shouldIgnoreBrowserStderrReadError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF || err == os.ErrClosed {
		return true
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "file already closed") ||
		strings.Contains(message, "handle is invalid")
}

func (m *browserProcessMonitor) waitForExit() {
	err := m.cmd.Wait()

	m.mu.Lock()
	m.result = browserProcessExitResult{
		Err:        err,
		StderrTail: m.stderrTail.String(),
	}
	if m.cmd.ProcessState != nil {
		code := m.cmd.ProcessState.ExitCode()
		m.result.ExitCode = &code
	}
	m.mu.Unlock()
	close(m.waitDone)
}

type tailTextBuffer struct {
	maxLines int
	maxBytes int

	mu         sync.Mutex
	lines      []string
	totalBytes int
}

func newTailTextBuffer(maxLines int, maxBytes int) *tailTextBuffer {
	if maxLines <= 0 {
		maxLines = 1
	}
	if maxBytes <= 0 {
		maxBytes = 1024
	}

	return &tailTextBuffer{
		maxLines: maxLines,
		maxBytes: maxBytes,
	}
}

func (b *tailTextBuffer) Append(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	if len(trimmed) > b.maxBytes {
		trimmed = trimmed[len(trimmed)-b.maxBytes:]
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, trimmed)
	b.totalBytes += len(trimmed) + 1
	for len(b.lines) > b.maxLines || b.totalBytes > b.maxBytes {
		if len(b.lines) == 0 {
			b.totalBytes = 0
			break
		}
		b.totalBytes -= len(b.lines[0]) + 1
		b.lines = b.lines[1:]
	}
}

func (b *tailTextBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.lines, "\n")
}

func parseBrowserDebugPortFromStderrLine(line string) (int, bool) {
	const marker = "DevTools listening on "

	idx := strings.Index(line, marker)
	if idx < 0 {
		return 0, false
	}

	rawURL := strings.TrimSpace(line[idx+len(marker):])
	if rawURL == "" {
		return 0, false
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, false
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

func (m *browserProcessMonitor) initialPID() int {
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}
