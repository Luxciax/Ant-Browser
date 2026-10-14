package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type runtimeMessage struct {
	ID        int             `json:"id"`
	Method    string          `json:"method"`
	SessionID string          `json:"sessionId"`
	Params    json.RawMessage `json:"params"`
	Error     *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ObserveRuntime owns a separate CDP session, not the passive diagnostics
// connection. It attaches existing/new pages and child frames. It never grants
// geolocation permission, injects JS, or changes the proxy/seed/hardware.
func ObserveRuntime(ctx context.Context, port int, id Identity, emit func(string, map[string]any)) {
	if id.Geo == nil && id.Timezone == "" && id.Locale == "" {
		return
	}
	for ctx.Err() == nil {
		err := observeRuntimeConnection(ctx, port, id, emit)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			emit("identity.runtime_disconnected", map[string]any{"error": err.Error()})
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func observeRuntimeConnection(ctx context.Context, port int, id Identity, emit func(string, map[string]any)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/version", port), nil)
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
		WebSocketURL string `json:"webSocketDebuggerUrl"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&version)
	resp.Body.Close()
	if err != nil {
		return err
	}
	u, err := url.Parse(version.WebSocketURL)
	if err != nil || u.Host != fmt.Sprintf("127.0.0.1:%d", port) || u.Scheme != "ws" {
		return fmt.Errorf("invalid local identity CDP endpoint")
	}
	conn, _, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).DialContext(ctx, version.WebSocketURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)
	conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(45 * time.Second)) })
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	next := 0
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
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
	pending := map[int]string{}
	send := func(method string, params any, session string) error {
		next++
		pending[next] = method
		conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		return conn.WriteJSON(map[string]any{"id": next, "method": method, "params": params, "sessionId": session})
	}
	attach := map[string]any{"autoAttach": true, "flatten": true, "waitForDebuggerOnStart": true}
	if err := send("Target.setAutoAttach", attach, ""); err != nil {
		return err
	}
	for {
		var m runtimeMessage
		if err := conn.ReadJSON(&m); err != nil {
			return err
		}
		if m.ID != 0 {
			method := pending[m.ID]
			delete(pending, m.ID)
			if m.Error != nil {
				emit("identity.override_failed", map[string]any{"method": method, "session_id": m.SessionID, "error_code": m.Error.Code, "error": m.Error.Message})
			}
			continue
		}
		if m.Method != "Target.attachedToTarget" {
			continue
		}
		var p struct {
			SessionID string `json:"sessionId"`
			Waiting   bool   `json:"waitingForDebugger"`
			Target    struct {
				ID   string `json:"targetId"`
				Type string `json:"type"`
			} `json:"targetInfo"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return err
		}
		if p.SessionID == "" {
			continue
		}
		if p.Target.Type == "page" || p.Target.Type == "iframe" || p.Target.Type == "worker" || p.Target.Type == "shared_worker" || p.Target.Type == "service_worker" {
			if (p.Target.Type == "page" || p.Target.Type == "iframe") && id.Geo != nil && ValidGeo(*id.Geo) {
				if err := send("Emulation.setGeolocationOverride", map[string]any{"latitude": id.Geo.Latitude, "longitude": id.Geo.Longitude, "accuracy": id.Geo.Accuracy}, p.SessionID); err != nil {
					return err
				}
			}
			if id.Timezone != "" && id.Timezone != "system" {
				if err := send("Emulation.setTimezoneOverride", map[string]any{"timezoneId": id.Timezone}, p.SessionID); err != nil {
					return err
				}
			}
			if id.Locale != "" {
				if err := send("Emulation.setLocaleOverride", map[string]any{"locale": id.Locale}, p.SessionID); err != nil {
					return err
				}
			}
			if err := send("Target.setAutoAttach", attach, p.SessionID); err != nil {
				return err
			}
		}
		// Resume EVERY target, including workers and unsupported targets. A failed
		// override must never leave a page waiting forever for the debugger.
		if p.Waiting {
			if err := send("Runtime.runIfWaitingForDebugger", map[string]any{}, p.SessionID); err != nil {
				return err
			}
		}
		emit("identity.target_overrides_requested", map[string]any{"target_id": p.Target.ID, "target_type": p.Target.Type})
	}
}
