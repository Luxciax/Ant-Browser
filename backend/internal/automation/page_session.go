package automation

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) RunPageCommands(ctx context.Context, req PageCommandRequest) (PageCommandResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	profileID := strings.TrimSpace(req.ProfileID)
	if profileID == "" {
		return PageCommandResult{}, fmt.Errorf("profileId is required")
	}
	if len(req.Commands) == 0 {
		return PageCommandResult{}, fmt.Errorf("at least one page command is required")
	}
	if strings.TrimSpace(req.LaunchBaseURL) == "" {
		return PageCommandResult{}, fmt.Errorf("launchBaseUrl is required")
	}
	state := m.CurrentState()
	if !state.Ready {
		return PageCommandResult{}, fmt.Errorf("automation runtime is not ready")
	}

	session, reused, err := m.acquirePageSession(state, req)
	if err != nil {
		return PageCommandResult{}, err
	}
	result := PageCommandResult{
		ProfileID: profileID,
		OK:        true,
		Reused:    reused,
		Session:   session.session,
		Results:   make([]PageStepResult, 0, len(req.Commands)),
	}
	for index, command := range req.Commands {
		if err := ctx.Err(); err != nil {
			result.OK = false
			result.Error = err.Error()
			return result, nil
		}
		step, callErr := session.call(command, req.Timeout)
		if callErr != nil {
			m.dropPageSession(profileID, session)
			if index == 0 && reused {
				fresh, _, spawnErr := m.acquirePageSession(state, req)
				if spawnErr != nil {
					result.OK = false
					result.Error = "page session could not be rebuilt: " + spawnErr.Error()
					return result, nil
				}
				session = fresh
				result.Reused = false
				result.Session = fresh.session
				step, callErr = session.call(command, req.Timeout)
			}
			if callErr != nil {
				m.dropPageSession(profileID, session)
				result.OK = false
				result.Error = callErr.Error()
				return result, nil
			}
		}
		result.Results = append(result.Results, step)
		if !step.OK {
			result.OK = false
			result.Error = step.Error
			break
		}
	}
	return result, nil
}

func (m *Manager) ClosePageSession(profileID string) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}
	m.pageMu.Lock()
	session := m.pageSessions[profileID]
	delete(m.pageSessions, profileID)
	m.pageMu.Unlock()
	if session != nil {
		session.close()
	}
}

func (m *Manager) closeAllPageSessions() {
	m.pageMu.Lock()
	sessions := make([]*pageSession, 0, len(m.pageSessions))
	for _, session := range m.pageSessions {
		sessions = append(sessions, session)
	}
	m.pageSessions = make(map[string]*pageSession)
	m.pageMu.Unlock()
	for _, session := range sessions {
		if session != nil {
			session.close()
		}
	}
}

func (m *Manager) PageSessionProfiles() []string {
	m.pageMu.Lock()
	defer m.pageMu.Unlock()
	items := make([]string, 0, len(m.pageSessions))
	for profileID := range m.pageSessions {
		items = append(items, profileID)
	}
	return items
}

func (m *Manager) acquirePageSession(state RuntimeState, req PageCommandRequest) (*pageSession, bool, error) {
	profileID := strings.TrimSpace(req.ProfileID)
	m.pageMu.Lock()
	if existing := m.pageSessions[profileID]; existing != nil && !existing.isClosed() {
		m.pageMu.Unlock()
		return existing, true, nil
	}
	m.pageMu.Unlock()

	session, err := m.spawnPageSession(state, req)
	if err != nil {
		return nil, false, err
	}
	m.pageMu.Lock()
	if existing := m.pageSessions[profileID]; existing != nil && !existing.isClosed() {
		m.pageMu.Unlock()
		session.close()
		return existing, true, nil
	}
	m.pageSessions[profileID] = session
	m.pageMu.Unlock()
	m.ensurePageSessionReaper(req.IdleTimeout)
	return session, false, nil
}

func (m *Manager) dropPageSession(profileID string, session *pageSession) {
	m.pageMu.Lock()
	if current := m.pageSessions[profileID]; current == session {
		delete(m.pageSessions, profileID)
	}
	m.pageMu.Unlock()
	if session != nil {
		session.close()
	}
}

func (m *Manager) spawnPageSession(state RuntimeState, req PageCommandRequest) (*pageSession, error) {
	payload := pageSessionPayload{
		RuntimeDir:       state.RuntimeDir,
		Selector:         req.Selector,
		LaunchBaseURL:    strings.TrimSpace(req.LaunchBaseURL),
		LaunchAuthHeader: strings.TrimSpace(req.LaunchAuthHeader),
		LaunchAuthValue:  strings.TrimSpace(req.LaunchAuthValue),
		ArtifactDir:      strings.TrimSpace(req.ArtifactDir),
		ConnectTimeoutMs: pageSessionReadyTimeout.Milliseconds(),
	}
	if req.Timeout > 0 {
		payload.DefaultTimeoutMs = req.Timeout.Milliseconds()
	}
	payloadPath, err := m.writePageSessionPayload(payload)
	if err != nil {
		return nil, err
	}
	defer os.Remove(payloadPath)

	runnerPath := filepath.Join(state.RuntimeDir, pageSessionRunnerFileName)
	if _, err := os.Stat(runnerPath); err != nil {
		return nil, fmt.Errorf("page session runner is unavailable: %w", err)
	}
	cmd := exec.Command(state.NodePath, runnerPath, payloadPath)
	cmd.Dir = state.RuntimeDir
	prepareTaskCommand(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create page session stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create page session stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start page session: %w", err)
	}
	session := &pageSession{
		profileID: strings.TrimSpace(req.ProfileID),
		proc:      &nodeSessionProcess{cmd: cmd, stdin: stdin, stdout: stdout},
		lastUsed:  time.Now(),
		reader:    bufio.NewReaderSize(stdout, 64<<10),
	}
	go func() { _ = cmd.Wait() }()
	if err := session.awaitReady(pageSessionReadyTimeout); err != nil {
		session.close()
		return nil, err
	}
	return session, nil
}

func (m *Manager) writePageSessionPayload(payload pageSessionPayload) (string, error) {
	tempDir := filepath.Join(m.runtimeRoot(), "tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("create page session temp dir: %w", err)
	}
	file, err := os.CreateTemp(tempDir, "page-session-*.json")
	if err != nil {
		return "", fmt.Errorf("create page session payload: %w", err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(payload); err != nil {
		return "", fmt.Errorf("write page session payload: %w", err)
	}
	return file.Name(), nil
}

func (m *Manager) ensurePageSessionReaper(idleTimeout time.Duration) {
	if idleTimeout <= 0 {
		idleTimeout = defaultPageSessionIdle
	}
	m.pageMu.Lock()
	if m.pageReaperOn {
		m.pageMu.Unlock()
		return
	}
	m.pageReaperOn = true
	m.pageMu.Unlock()
	go m.reapIdlePageSessions(idleTimeout)
}

func (m *Manager) reapIdlePageSessions(idleTimeout time.Duration) {
	interval := idleTimeout / 4
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.pageMu.Lock()
		expired := make([]*pageSession, 0)
		for profileID, session := range m.pageSessions {
			if session == nil || session.isClosed() || now.Sub(session.touchedAt()) > idleTimeout {
				expired = append(expired, session)
				delete(m.pageSessions, profileID)
			}
		}
		remaining := len(m.pageSessions)
		if remaining == 0 {
			m.pageReaperOn = false
		}
		m.pageMu.Unlock()
		for _, session := range expired {
			if session != nil {
				session.close()
			}
		}
		if remaining == 0 {
			return
		}
	}
}
