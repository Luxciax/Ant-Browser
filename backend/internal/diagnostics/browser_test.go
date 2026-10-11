package diagnostics

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNetworkFailureRecordsOnlyDomainAndNetCode(t *testing.T) {
	s := map[string]*session{"s": {TargetID: "t", Requests: map[string]request{}}}
	events := []Event{}
	emit := func(e Event) { events = append(events, e) }
	at := time.Now()
	handleEvent(message{Method: "Network.requestWillBeSent", SessionID: "s", Params: json.RawMessage(`{"requestId":"r","type":"Fetch","request":{"url":"https://user:password@example.com/private-chat?token=secret","postData":"PRIVATE BODY"}}`)}, s, emit, at)
	handleEvent(message{Method: "Network.loadingFailed", SessionID: "s", Params: json.RawMessage(`{"requestId":"r","type":"Fetch","errorText":"net::ERR_PROXY_CONNECTION_FAILED","canceled":false}`)}, s, emit, at.Add(time.Second))
	raw, _ := json.Marshal(events)
	if len(events) != 1 || events[0].Fields["domain"] != "example.com" || events[0].Fields["net_error"] != "net::ERR_PROXY_CONNECTION_FAILED" || events[0].Fields["duration_ms"] != int64(1000) {
		t.Fatalf("unexpected event %s", raw)
	}
	for _, secret := range []string{"password", "private-chat", "secret", "PRIVATE BODY"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
}

func TestRendererCrashAndHTTPRefusalAreDifferentEvents(t *testing.T) {
	events := []Event{}
	emit := func(e Event) { events = append(events, e) }
	s := map[string]*session{"s": {TargetID: "page", Requests: map[string]request{}}}
	handleEvent(message{Method: "Target.targetCrashed", Params: json.RawMessage(`{"targetId":"page","status":"crashed","errorCode":123}`)}, s, emit, time.Now())
	handleEvent(message{Method: "Network.responseReceived", SessionID: "s", Params: json.RawMessage(`{"requestId":"r","response":{"url":"https://example.com/?password=x","status":403,"protocol":"h2"}}`)}, s, emit, time.Now())
	if len(events) != 2 || events[0].Name != "renderer.crashed" || events[1].Name != "network.response" || events[1].Fields["http_status"] != 403 {
		t.Fatalf("wrong classifications: %#v", events)
	}
}

func TestObserverCancellationClosesConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}
	connected := make(chan struct{}, 1)
	closed := make(chan struct{}, 1)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			json.NewEncoder(w).Encode(map[string]string{"Browser": "Chromium/test", "webSocketDebuggerUrl": strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"})
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		connected <- struct{}{}
		for {
			if _, _, err = conn.ReadMessage(); err != nil {
				closed <- struct{}{}
				return
			}
		}
	}))
	defer server.Close()
	port, _ := strconv.Atoi(strings.Split(server.URL, ":")[2])
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { Observe(ctx, port, func(Event) {}); close(done) }()
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("not connected")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("observer did not stop")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("websocket leaked")
	}
}
