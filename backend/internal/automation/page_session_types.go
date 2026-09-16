package automation

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultPageSessionIdle    = 5 * time.Minute
	pageSessionReadyTimeout   = 90 * time.Second
	pageSessionMaxLine        = 8 << 20
	pageSessionRunnerFileName = "runner_page_session.cjs"
)

type PageCommand struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

type PageCommandRequest struct {
	ProfileID string
	Selector  map[string]any
	Commands  []PageCommand

	LaunchBaseURL    string
	LaunchAuthHeader string
	LaunchAuthValue  string
	ArtifactDir      string
	Timeout          time.Duration
	IdleTimeout      time.Duration
}

type PageCommandResult struct {
	ProfileID string           `json:"profileId"`
	OK        bool             `json:"ok"`
	Results   []PageStepResult `json:"results"`
	Session   map[string]any   `json:"session,omitempty"`
	Reused    bool             `json:"reused"`
	Error     string           `json:"error,omitempty"`
}

type PageStepResult struct {
	Action string         `json:"action"`
	OK     bool           `json:"ok"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type sessionProcess interface {
	io.Writer
	Stdout() io.Reader
	Kill() error
}

type nodeSessionProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
}

func (p *nodeSessionProcess) Write(data []byte) (int, error) { return p.stdin.Write(data) }
func (p *nodeSessionProcess) Stdout() io.Reader              { return p.stdout }
func (p *nodeSessionProcess) Kill() error {
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	return stopTaskProcess(p.cmd)
}

type pageSession struct {
	profileID string
	proc      sessionProcess
	reader    *bufio.Reader
	session   map[string]any

	mu       sync.Mutex
	nextID   int64
	lastUsed time.Time
	closed   bool
}

type sessionEnvelope struct {
	Type    string         `json:"type,omitempty"`
	ID      int64          `json:"id,omitempty"`
	OK      bool           `json:"ok,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   string         `json:"error,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	Session map[string]any `json:"session,omitempty"`
}

type pageSessionPayload struct {
	RuntimeDir       string         `json:"runtimeDir"`
	Selector         map[string]any `json:"selector,omitempty"`
	LaunchBaseURL    string         `json:"launchBaseUrl"`
	LaunchAuthHeader string         `json:"launchAuthHeader,omitempty"`
	LaunchAuthValue  string         `json:"launchAuthValue,omitempty"`
	ArtifactDir      string         `json:"artifactDir,omitempty"`
	DefaultTimeoutMs int64          `json:"defaultTimeoutMs,omitempty"`
	ConnectTimeoutMs int64          `json:"connectTimeoutMs,omitempty"`
}

func marshalPageCommand(id int64, command PageCommand) ([]byte, error) {
	payload := struct {
		ID     int64          `json:"id"`
		Action string         `json:"action"`
		Args   map[string]any `json:"args,omitempty"`
	}{ID: id, Action: command.Action, Args: command.Args}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
