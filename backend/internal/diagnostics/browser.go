// Package diagnostics observes existing browser traffic; it never submits
// probes, collects payloads, or navigates/reloads user tabs.
package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Event struct {
	Level  string
	Name   string
	Fields map[string]any
}

type message struct {
	ID        int             `json:"id"`
	Method    string          `json:"method"`
	SessionID string          `json:"sessionId"`
	Params    json.RawMessage `json:"params"`
	Result    json.RawMessage `json:"result"`
	Error     json.RawMessage `json:"error"`
}

type request struct {
	Domain, Kind string
	Started      time.Time
	Reported     bool
}
type session struct {
	TargetID string
	Requests map[string]request
}

// Observe reconnects without mutating the browser. Cancellation closes the
// websocket immediately even if the renderer/network service has hung.
func Observe(ctx context.Context, port int, emit func(Event)) {
	failures := 0
	last := map[string]time.Time{}
	repeated := map[string]int{}
	output := emit
	emit = func(e Event) {
		if strings.HasPrefix(e.Name, "network.") {
			key := fmt.Sprint(e.Name, e.Fields["target_id"], e.Fields["domain"], e.Fields["net_error"], e.Fields["http_status"])
			if time.Since(last[key]) < 5*time.Second {
				repeated[key]++
				return
			}
			e.Fields["repeated_since_previous"] = repeated[key]
			if len(last) > 512 {
				last = map[string]time.Time{}
				repeated = map[string]int{}
			}
			last[key] = time.Now()
			delete(repeated, key)
		}
		output(e)
	}
	for ctx.Err() == nil {
		err := observeConnection(ctx, port, emit)
		if ctx.Err() != nil {
			return
		}
		failures++
		if failures == 1 || failures%5 == 0 {
			emit(Event{"WARN", "diagnostics.disconnected", map[string]any{"reason": safeTransportError(err), "attempt": failures}})
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func safeTransportError(err error) string {
	if err == nil {
		return "connection closed"
	}
	// Keep actionable transport details without URLs, credentials or page data.
	var closed *websocket.CloseError
	if errors.As(err, &closed) {
		return fmt.Sprintf("websocket closed: code=%d", closed.Code)
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return "debugger transport timeout"
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return fmt.Sprintf("%s/%s: %T", op.Op, op.Net, op.Err)
	}
	return fmt.Sprintf("%T", err)
}

func observeConnection(ctx context.Context, port int, emit func(Event)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/json/version", port), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	var version struct {
		Browser      string `json:"Browser"`
		WebSocketURL string `json:"webSocketDebuggerUrl"`
	}
	err = json.NewDecoder(resp.Body).Decode(&version)
	resp.Body.Close()
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(version.WebSocketURL)
	if err != nil || endpoint.Hostname() != "127.0.0.1" || endpoint.Port() != fmt.Sprint(port) {
		return fmt.Errorf("unexpected debugger endpoint")
	}
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second, Proxy: nil}
	conn, _, err := dialer.DialContext(ctx, version.WebSocketURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetReadLimit(1024 * 1024)
	closed := make(chan struct{})
	defer close(closed)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-closed:
		}
	}()
	_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(45 * time.Second)) })
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-closed:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)) != nil {
					conn.Close()
					return
				}
			}
		}
	}()
	commands := map[int]struct{ Method, SessionID string }{}
	nextID := 0
	send := func(method string, params any, sessionID string) error {
		nextID++
		commands[nextID] = struct{ Method, SessionID string }{method, sessionID}
		if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
			return err
		}
		return conn.WriteJSON(map[string]any{"id": nextID, "method": method, "params": params, "sessionId": sessionID})
	}
	if err = send("Target.setDiscoverTargets", map[string]any{"discover": true}, ""); err != nil {
		return err
	}
	emit(Event{"INFO", "diagnostics.connected", map[string]any{"browser_version": version.Browser, "debug_port": port}})
	sessions := map[string]*session{}
	attached := map[string]bool{}
	incoming := make(chan message, 1)
	readErrors := make(chan error, 1)
	go func() {
		for {
			var m message
			if e := conn.ReadJSON(&m); e != nil {
				readErrors <- e
				return
			}
			select {
			case incoming <- m:
			case <-closed:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		var m message
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err = <-readErrors:
			return err
		case <-ticker.C:
			for _, s := range sessions {
				for id, r := range s.Requests {
					if !r.Reported && time.Since(r.Started) > 30*time.Second && r.Kind != "EventSource" && r.Kind != "WebSocket" && r.Kind != "Media" {
						emit(Event{"WARN", "network.pending", map[string]any{"target_id": s.TargetID, "domain": r.Domain, "resource_type": r.Kind, "duration_ms": time.Since(r.Started).Milliseconds(), "note": "尚未收到响应头；不能据此认定断网"}})
						r.Reported = true
						s.Requests[id] = r
					}
				}
			}
			continue
		case m = <-incoming:
		}
		if m.ID != 0 {
			cmd := commands[m.ID]
			delete(commands, m.ID)
			if len(m.Error) > 0 && string(m.Error) != "null" {
				emit(Event{"WARN", "diagnostics.command_failed", map[string]any{"command_id": m.ID, "method": cmd.Method}})
			} else if cmd.Method == "Network.enable" {
				s := sessions[cmd.SessionID]
				if s != nil {
					emit(Event{"INFO", "diagnostics.network_ready", map[string]any{"target_id": s.TargetID}})
				}
			}
			continue
		}
		switch m.Method {
		case "Target.targetCreated":
			var p struct {
				Info struct {
					ID   string `json:"targetId"`
					Type string `json:"type"`
				} `json:"targetInfo"`
			}
			if json.Unmarshal(m.Params, &p) != nil {
				continue
			}
			if p.Info.Type == "page" && !attached[p.Info.ID] {
				attached[p.Info.ID] = true
				if err = send("Target.attachToTarget", map[string]any{"targetId": p.Info.ID, "flatten": true}, ""); err != nil {
					return err
				}
			}
		case "Target.attachedToTarget":
			var p struct {
				SessionID string `json:"sessionId"`
				Info      struct {
					ID string `json:"targetId"`
				} `json:"targetInfo"`
			}
			if json.Unmarshal(m.Params, &p) != nil {
				continue
			}
			sessions[p.SessionID] = &session{p.Info.ID, map[string]request{}}
			// No request/response bodies are retained or requested.
			if err = send("Network.enable", map[string]any{"maxPostDataSize": 0, "maxResourceBufferSize": 0, "maxTotalBufferSize": 0}, p.SessionID); err != nil {
				return err
			}
			if err = send("Inspector.enable", map[string]any{}, p.SessionID); err != nil {
				return err
			}
		case "Target.targetDestroyed":
			var p struct {
				ID string `json:"targetId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			delete(attached, p.ID)
			for id, s := range sessions {
				if s.TargetID == p.ID {
					delete(sessions, id)
				}
			}
		case "Target.detachedFromTarget":
			var p struct {
				SessionID string `json:"sessionId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			delete(sessions, p.SessionID)
		default:
			handleEvent(m, sessions, emit, time.Now())
		}
	}
}

func handleEvent(m message, sessions map[string]*session, emit func(Event), now time.Time) {
	s := sessions[m.SessionID]
	if m.Method == "Target.targetCrashed" || m.Method == "Inspector.targetCrashed" {
		var p struct {
			TargetID  string `json:"targetId"`
			Status    string `json:"status"`
			ErrorCode int    `json:"errorCode"`
		}
		_ = json.Unmarshal(m.Params, &p)
		if s != nil {
			p.TargetID = s.TargetID
		}
		emit(Event{"ERROR", "renderer.crashed", map[string]any{"target_id": p.TargetID, "status": p.Status, "error_code": p.ErrorCode}})
		return
	}
	if s == nil {
		return
	}
	var p struct {
		RequestID     string `json:"requestId"`
		Type          string `json:"type"`
		ErrorText     string `json:"errorText"`
		Canceled      bool   `json:"canceled"`
		BlockedReason string `json:"blockedReason"`
		Request       struct {
			URL string `json:"url"`
		} `json:"request"`
		Response struct {
			URL      string `json:"url"`
			Status   int    `json:"status"`
			Protocol string `json:"protocol"`
		} `json:"response"`
	}
	if json.Unmarshal(m.Params, &p) != nil {
		return
	}
	key := p.RequestID
	switch m.Method {
	case "Network.requestWillBeSent":
		domain := Domain(p.Request.URL)
		if domain != "" && len(s.Requests) < 512 {
			s.Requests[key] = request{Domain: domain, Kind: p.Type, Started: now}
		}
	case "Network.responseReceived":
		r := s.Requests[key]
		delete(s.Requests, key) // headers prove a response; don't call an SSE stream a hang.
		duration := int64(0)
		if !r.Started.IsZero() {
			duration = now.Sub(r.Started).Milliseconds()
		}
		if p.Response.Status >= 400 || duration >= 3000 {
			domain := Domain(p.Response.URL)
			if domain == "" {
				domain = r.Domain
			}
			emit(Event{"WARN", "network.response", map[string]any{"target_id": s.TargetID, "domain": domain, "resource_type": p.Type, "http_status": p.Response.Status, "duration_ms": duration, "protocol": p.Response.Protocol}})
		}
	case "Network.loadingFinished":
		delete(s.Requests, key)
	case "Network.loadingFailed":
		r := s.Requests[key]
		delete(s.Requests, key)
		level := "ERROR"
		if p.Canceled {
			level = "INFO"
		}
		// Chromium net error identifiers are sufficient; arbitrary error strings
		// could contain addresses, credentials or user content.
		code := "unknown"
		if strings.HasPrefix(p.ErrorText, "net::ERR_") {
			code = strings.Fields(p.ErrorText)[0]
		}
		duration := int64(0)
		if !r.Started.IsZero() {
			duration = now.Sub(r.Started).Milliseconds()
		}
		emit(Event{level, "network.failed", map[string]any{"target_id": s.TargetID, "domain": r.Domain, "resource_type": p.Type, "net_error": code, "canceled": p.Canceled, "blocked_reason": p.BlockedReason, "duration_ms": duration}})
	}
}

func Domain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.Hostname()
}
