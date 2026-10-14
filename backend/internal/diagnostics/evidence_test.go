package diagnostics

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCrashMissingCodeRemainsUnknownAndLifecycleResumptionIsExplicit(t *testing.T) {
	sessions := map[string]*session{"s": {TargetID: "page", TargetType: "page", Requests: map[string]request{}}}
	var events []Event
	emit := func(e Event) { events = append(events, e) }
	handleEvent(message{Method: "Inspector.targetCrashed", SessionID: "s", Params: json.RawMessage(`{}`)}, sessions, emit, time.Now())
	if len(events) != 1 || events[0].Fields["error_code"] != nil || events[0].Fields["status"] != nil || events[0].Fields["source_event"] != "Inspector.targetCrashed" {
		t.Fatalf("invented crash evidence: %#v", events)
	}
	handleEvent(message{Method: "Page.frameStoppedLoading", SessionID: "s", Params: json.RawMessage(`{}`)}, sessions, emit, time.Now())
	if len(events) < 2 || events[len(events)-1].Name != "renderer.target_resumed" {
		t.Fatalf("missing resumption evidence: %#v", events)
	}
}

func TestTimingUnavailableIsNotZeroAndCancellationIsInformational(t *testing.T) {
	timing := networkTimingEvidence(map[string]float64{"dnsStart": -1, "dnsEnd": -1, "sslStart": 2, "sslEnd": 12})
	if timing["dns_ms"] != nil || timing["tls_ms"] != float64(10) {
		t.Fatal(timing)
	}
	var got Event
	handleEvent(message{Method: "Network.loadingFailed", SessionID: "s", Params: json.RawMessage(`{"requestId":"r","canceled":true,"errorText":"net::ERR_ABORTED"}`)}, map[string]*session{"s": {TargetID: "page", Requests: map[string]request{}}}, func(e Event) { got = e }, time.Now())
	if got.Level != "INFO" || got.Fields["canceled"] != true {
		t.Fatal(got)
	}
}
