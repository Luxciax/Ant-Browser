package identity

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRuntimeCoversPagesFramesWorkersAndResumesAfterFailure(t *testing.T) {
	var server *httptest.Server
	var mu sync.Mutex
	commands := map[string][]string{}
	complete := make(chan struct{})
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			fmt.Fprintf(w, `{"webSocketDebuggerUrl":%q}`, "ws"+server.URL[4:]+"/ws")
			return
		}
		up := websocket.Upgrader{}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var first map[string]any
		if conn.ReadJSON(&first) != nil {
			return
		}
		conn.WriteJSON(map[string]any{"id": first["id"], "result": map[string]any{}})
		for _, kind := range []string{"page", "iframe", "worker", "service_worker", "other"} {
			conn.WriteJSON(map[string]any{"method": "Target.attachedToTarget", "params": map[string]any{"sessionId": kind, "waitingForDebugger": true, "targetInfo": map[string]any{"targetId": "target-" + kind, "type": kind}}})
			for {
				var m map[string]any
				if conn.ReadJSON(&m) != nil {
					return
				}
				method := m["method"].(string)
				session := m["sessionId"].(string)
				mu.Lock()
				commands[session] = append(commands[session], method)
				mu.Unlock()
				response := map[string]any{"id": m["id"], "sessionId": session, "result": map[string]any{}}
				if method == "Emulation.setLocaleOverride" {
					delete(response, "result")
					response["error"] = map[string]any{"code": -32601, "message": "fixture unsupported method"}
				}
				if conn.WriteJSON(response) != nil {
					return
				}
				if method == "Runtime.runIfWaitingForDebugger" {
					break
				}
			}
		}
		close(complete)
		for {
			var m any
			if conn.ReadJSON(&m) != nil {
				return
			}
		}
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- observeRuntimeConnection(ctx, port, Identity{Locale: "ja-JP", Timezone: "Asia/Tokyo", Geo: &Geo{35, 139, 1000}}, func(string, map[string]any) {})
	}()
	select {
	case <-complete:
	case <-time.After(5 * time.Second):
		t.Fatal("targets left waiting after an override error")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close idle socket")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, kind := range []string{"page", "iframe", "worker", "service_worker", "other"} {
		got := commands[kind]
		if len(got) == 0 || got[len(got)-1] != "Runtime.runIfWaitingForDebugger" {
			t.Fatalf("%s not resumed: %v", kind, got)
		}
	}
}
