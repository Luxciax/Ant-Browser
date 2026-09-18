package automation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *pageSession) call(command PageCommand, timeout time.Duration) (PageStepResult, error) {
	action := strings.TrimSpace(command.Action)
	if action == "" {
		return PageStepResult{}, fmt.Errorf("page action is required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return PageStepResult{}, fmt.Errorf("page session is closed")
	}
	s.nextID++
	id := s.nextID
	line, err := marshalPageCommand(id, PageCommand{Action: action, Args: command.Args})
	if err != nil {
		return PageStepResult{}, fmt.Errorf("encode page command: %w", err)
	}
	if _, err := s.proc.Write(line); err != nil {
		s.markClosed()
		return PageStepResult{}, fmt.Errorf("page session disconnected: %w", err)
	}

	envelope, err := s.readEnvelope(id, timeout+10*time.Second)
	if err != nil {
		s.markClosed()
		return PageStepResult{}, err
	}
	s.lastUsed = time.Now()
	return PageStepResult{Action: action, OK: envelope.OK, Result: envelope.Result, Error: envelope.Error}, nil
}

func (s *pageSession) awaitReady(timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	envelope, err := s.readEnvelope(0, timeout)
	if err != nil {
		return err
	}
	if envelope.Type != "ready" {
		return fmt.Errorf("page session startup failed: %s", describeEnvelope(envelope))
	}
	s.session = envelope.Session
	s.lastUsed = time.Now()
	return nil
}

func (s *pageSession) readEnvelope(wantID int64, timeout time.Duration) (sessionEnvelope, error) {
	type readResult struct {
		envelope sessionEnvelope
		err      error
	}
	done := make(chan readResult, 1)
	go func() {
		for {
			line, err := s.readLine()
			if err != nil {
				done <- readResult{err: err}
				return
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			var envelope sessionEnvelope
			if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
				done <- readResult{err: fmt.Errorf("decode page session response: %w", err)}
				return
			}
			if envelope.Type == "closed" {
				done <- readResult{err: fmt.Errorf("page session closed: %s", strings.TrimSpace(envelope.Reason))}
				return
			}
			if wantID > 0 && envelope.ID != wantID {
				continue
			}
			done <- readResult{envelope: envelope}
			return
		}
	}()

	select {
	case result := <-done:
		return result.envelope, result.err
	case <-time.After(timeout):
		return sessionEnvelope{}, fmt.Errorf("page session response timed out after %s", timeout)
	}
}

func (s *pageSession) readLine() (string, error) {
	var builder strings.Builder
	for {
		chunk, isPrefix, err := s.reader.ReadLine()
		if err != nil {
			return "", fmt.Errorf("page session disconnected: %w", err)
		}
		builder.Write(chunk)
		if builder.Len() > pageSessionMaxLine {
			return "", fmt.Errorf("page session response exceeds %d bytes", pageSessionMaxLine)
		}
		if !isPrefix {
			return builder.String(), nil
		}
	}
}

func describeEnvelope(envelope sessionEnvelope) string {
	if message := strings.TrimSpace(envelope.Error); message != "" {
		return message
	}
	if reason := strings.TrimSpace(envelope.Reason); reason != "" {
		return reason
	}
	if envelope.Type != "" {
		return "unexpected message type " + envelope.Type
	}
	return "unknown response"
}

func (s *pageSession) markClosed() {
	if s.closed {
		return
	}
	s.closed = true
	if s.proc != nil {
		_ = s.proc.Kill()
	}
}

func (s *pageSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markClosed()
}

func (s *pageSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *pageSession) touchedAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastUsed
}

func normalizePageSessionIdleTimeout(idleTimeout time.Duration) time.Duration {
	if idleTimeout <= 0 {
		return defaultPageSessionIdle
	}
	return idleTimeout
}

func (s *pageSession) setIdleTimeout(idleTimeout time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.idleTimeout = normalizePageSessionIdleTimeout(idleTimeout)
	s.mu.Unlock()
}

func (s *pageSession) idleTimeoutValue() time.Duration {
	if s == nil {
		return defaultPageSessionIdle
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return normalizePageSessionIdleTimeout(s.idleTimeout)
}

func (s *pageSession) expiredAt(now time.Time) bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return true
	}
	idleTimeout := normalizePageSessionIdleTimeout(s.idleTimeout)
	return !s.lastUsed.IsZero() && now.Sub(s.lastUsed) > idleTimeout
}
