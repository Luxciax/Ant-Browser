package browser

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestDeleteRejectsRunningProfile(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), t.TempDir())
	profile := &Profile{
		ProfileId:   "running-profile",
		ProfileName: "Running",
		Running:     true,
		Pid:         1234,
		DebugPort:   9222,
		DebugReady:  true,
	}
	manager.Profiles[profile.ProfileId] = profile

	err := manager.Delete(profile.ProfileId)
	if err == nil || !strings.Contains(err.Error(), "stop it before deletion") {
		t.Fatalf("Delete() error = %v, want running profile rejection", err)
	}
	if _, exists := manager.Profiles[profile.ProfileId]; !exists {
		t.Fatal("running profile was removed from manager after rejected delete")
	}
	if profile.DeletedAt != "" {
		t.Fatalf("running profile DeletedAt = %q, want unchanged", profile.DeletedAt)
	}
}
