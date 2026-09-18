package launchcode

import (
	"ant-chrome/backend/internal/browser"
	"testing"
)

func TestNormalizeLaunchedProfileRuntimeKeepsLegacyCompatibility(t *testing.T) {
	profile := &browser.Profile{Pid: 4321, DebugPort: 9333}
	got := normalizeLaunchedProfileRuntime(profile)
	if got == nil || !got.Running || !got.DebugReady {
		t.Fatalf("legacy runtime was not normalized: %+v", got)
	}
	if got.RuntimeState != browser.RuntimeRunning {
		t.Fatalf("runtimeState = %q, want %q", got.RuntimeState, browser.RuntimeRunning)
	}
}

func TestNormalizeLaunchedProfileRuntimePreservesExplicitStarting(t *testing.T) {
	profile := &browser.Profile{
		RuntimeState: browser.RuntimeStarting,
		Pid:          4321,
		DebugPort:    9333,
	}
	got := normalizeLaunchedProfileRuntime(profile)
	if got == nil {
		t.Fatal("normalized profile is nil")
	}
	if got.RuntimeState != browser.RuntimeStarting {
		t.Fatalf("runtimeState = %q, want %q", got.RuntimeState, browser.RuntimeStarting)
	}
	if got.Running || got.DebugReady {
		t.Fatalf("explicit starting profile was promoted to running: %+v", got)
	}
}

func TestLaunchSuccessPayloadIncludesRuntimeState(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	profile := &browser.Profile{
		ProfileId:    "profile-1",
		ProfileName:  "Profile 1",
		RuntimeState: browser.RuntimeStarting,
	}
	payload := server.launchSuccessPayload(profile, "CODE_1")
	if got := payload["runtimeState"]; got != browser.RuntimeStarting {
		t.Fatalf("runtimeState = %#v, want %q", got, browser.RuntimeStarting)
	}
}
