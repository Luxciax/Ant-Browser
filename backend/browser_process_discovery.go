package backend

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type browserUserDataProcess struct {
	PID         int    `json:"pid"`
	DebugPort   int    `json:"debugPort"`
	CommandLine string `json:"commandLine"`
}

type browserRuntimeDetection struct {
	PID        int
	DebugPort  int
	DebugReady bool
}

var errBrowserStartHandledByRecoveredRuntime = errors.New("browser start handled by recovered runtime")

var findBrowserUserDataProcesses = findBrowserUserDataProcessesOS
var terminateBrowserUserDataProcess = terminateBrowserUserDataProcessOS

var remoteDebuggingPortPattern = regexp.MustCompile(`(?i)--remote-debugging-port=(\d+)`)

func parseRemoteDebuggingPort(commandLine string) int {
	matches := remoteDebuggingPortPattern.FindStringSubmatch(commandLine)
	if len(matches) < 2 {
		return 0
	}
	port, err := strconv.Atoi(matches[1])
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func detectBrowserRuntimeByUserDataDir(userDataDir string) (browserRuntimeDetection, bool) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return browserRuntimeDetection{}, false
	}

	if detection, ok := detectBrowserRuntimeByActivePort(userDataDir); ok {
		return detection, true
	}

	processes, err := findBrowserUserDataProcesses(userDataDir)
	if err != nil || len(processes) == 0 {
		return browserRuntimeDetection{}, false
	}

	for _, process := range processes {
		debugPort := process.DebugPort
		if debugPort <= 0 {
			debugPort = parseRemoteDebuggingPort(process.CommandLine)
		}
		if debugPort > 0 {
			if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
				return browserRuntimeDetection{PID: process.PID, DebugPort: debugPort, DebugReady: true}, true
			}
		}
	}

	first := processes[0]
	return browserRuntimeDetection{PID: first.PID, DebugPort: first.DebugPort}, true
}

func detectBrowserRuntimeByActivePort(userDataDir string) (browserRuntimeDetection, bool) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return browserRuntimeDetection{}, false
	}

	data, err := os.ReadFile(filepath.Join(userDataDir, "DevToolsActivePort"))
	if err != nil {
		return browserRuntimeDetection{}, false
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return browserRuntimeDetection{}, false
	}
	debugPort, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	expected := strings.TrimSpace(lines[1])
	if err != nil || debugPort <= 0 || debugPort > 65535 || !strings.HasPrefix(expected, "/devtools/browser/") {
		return browserRuntimeDetection{}, false
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := fetchBrowserDebugJSON(&http.Client{Timeout: browserDebugProbeTimeout}, debugPort, "/json/version", &version); err != nil {
		return browserRuntimeDetection{}, false
	}
	endpoint, err := url.Parse(version.WebSocketDebuggerURL)
	if err == nil && endpoint.Scheme == "ws" && endpoint.Path == expected && endpoint.Port() == strconv.Itoa(debugPort) && (endpoint.Hostname() == "127.0.0.1" || endpoint.Hostname() == "localhost" || endpoint.Hostname() == "::1") {
		return browserRuntimeDetection{DebugPort: debugPort, DebugReady: true}, true
	}

	return browserRuntimeDetection{}, false
}

// terminateBrowserUserDataOrphans 结束同一用户数据目录下无法被 CDP 接管的残留浏览器进程。
// 仅当主浏览器进程（无 --type= 参数）不携带调试端口时才清理，
// 避免误杀正在启动或受管实例的子进程；taskkill /T 会连同子进程一起结束。
func terminateBrowserUserDataOrphans(userDataDir string, timeout time.Duration) (bool, error) {
	processes, err := findBrowserUserDataProcesses(userDataDir)
	if err != nil {
		return false, err
	}
	mainPID := 0
	hasManaged := false
	for _, process := range processes {
		if process.PID <= 0 || isBrowserChildProcessCommandLine(process.CommandLine) {
			continue
		}
		debugPort := process.DebugPort
		if debugPort <= 0 {
			debugPort = parseRemoteDebuggingPort(process.CommandLine)
		}
		if debugPort > 0 {
			hasManaged = true
			continue
		}
		mainPID = process.PID
	}
	if hasManaged || mainPID <= 0 {
		return false, nil
	}
	if err := terminateBrowserUserDataProcess(mainPID, timeout); err != nil {
		return false, err
	}
	return true, nil
}

func isBrowserChildProcessCommandLine(commandLine string) bool {
	return strings.Contains(commandLine, "--type=")
}

func terminateBrowserProcessesByUserDataDir(userDataDir string, timeout time.Duration) (bool, error) {
	processes, err := findBrowserUserDataProcesses(userDataDir)
	if err != nil {
		return false, err
	}
	if len(processes) == 0 {
		return false, nil
	}

	var errs []error
	terminated := false
	for _, process := range processes {
		if process.PID <= 0 || process.PID == os.Getpid() {
			continue
		}
		terminated = true
		if err := terminateBrowserUserDataProcess(process.PID, timeout); err != nil {
			errs = append(errs, fmt.Errorf("pid %d: %w", process.PID, err))
		}
	}
	return terminated, errors.Join(errs...)
}
