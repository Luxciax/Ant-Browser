package browser

import "testing"

func TestProfileRuntimeMutationBlocked(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		blocked bool
	}{
		{name: "stopped", profile: Profile{RuntimeState: RuntimeStopped}, blocked: false},
		{name: "starting", profile: Profile{RuntimeState: RuntimeStarting}, blocked: true},
		{name: "running", profile: Profile{RuntimeState: RuntimeRunning}, blocked: true},
		{name: "stopping", profile: Profile{RuntimeState: RuntimeStopping}, blocked: true},
		{name: "failed quiescent", profile: Profile{RuntimeState: RuntimeFailed}, blocked: false},
		{name: "failed but process alive", profile: Profile{RuntimeState: RuntimeFailed, Pid: 42}, blocked: true},
		{name: "legacy running", profile: Profile{Running: true}, blocked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProfileRuntimeMutationBlocked(&tt.profile); got != tt.blocked {
				t.Fatalf("ProfileRuntimeMutationBlocked() = %v, want %v", got, tt.blocked)
			}
		})
	}
}
