package browser

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestDeleteRejectsRunningProfile(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), t.TempDir())
	profile := &Profile{ProfileId: "running-profile", ProfileName: "Running", Running: true, Pid: 1234, DebugPort: 9222, DebugReady: true}
	manager.Profiles[profile.ProfileId] = profile
	err := manager.Delete(profile.ProfileId)
	if err == nil || !strings.Contains(err.Error(), "stop it before deletion") {
		t.Fatalf("Delete() error = %v, want running profile rejection", err)
	}
	if _, exists := manager.Profiles[profile.ProfileId]; !exists {
		t.Fatal("running profile was removed after rejected delete")
	}
}

func TestDeleteRejectsTransitionalRuntimeStates(t *testing.T) {
	for _, state := range []ProfileRuntimeState{RuntimeStarting, RuntimeStopping} {
		t.Run(string(state), func(t *testing.T) {
			manager := NewManager(config.DefaultConfig(), t.TempDir())
			profile := &Profile{ProfileId: "profile-" + string(state), ProfileName: "Profile", RuntimeState: state}
			manager.Profiles[profile.ProfileId] = profile

			err := manager.Delete(profile.ProfileId)
			if err == nil || !strings.Contains(err.Error(), "stop it before deletion") {
				t.Fatalf("Delete() error = %v, want %s rejection", err, state)
			}
			if _, exists := manager.Profiles[profile.ProfileId]; !exists {
				t.Fatalf("profile in %s state was removed after rejected delete", state)
			}
		})
	}
}
