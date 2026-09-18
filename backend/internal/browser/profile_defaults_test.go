package browser

import (
	"ant-chrome/backend/internal/config"
	"reflect"
	"testing"
)

func TestApplyDefaultsReportsNonProxyChanges(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultFingerprintArgs = []string{"--fingerprint=123"}
	cfg.Browser.DefaultLaunchArgs = []string{"--disable-gpu"}
	mgr := NewManager(cfg, t.TempDir())
	profile := &Profile{ProfileId: "profile-a"}

	if !mgr.ApplyDefaults(profile) {
		t.Fatal("ApplyDefaults should report changes when non-proxy defaults are applied")
	}
	if profile.UserDataDir != profile.ProfileId {
		t.Fatalf("UserDataDir = %q, want %q", profile.UserDataDir, profile.ProfileId)
	}
	if !reflect.DeepEqual(profile.FingerprintArgs, cfg.Browser.DefaultFingerprintArgs) {
		t.Fatalf("FingerprintArgs = %#v, want %#v", profile.FingerprintArgs, cfg.Browser.DefaultFingerprintArgs)
	}
	if !reflect.DeepEqual(profile.LaunchArgs, cfg.Browser.DefaultLaunchArgs) {
		t.Fatalf("LaunchArgs = %#v, want %#v", profile.LaunchArgs, cfg.Browser.DefaultLaunchArgs)
	}
}

func TestApplyDefaultsReturnsFalseWhenNothingChanges(t *testing.T) {
	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, t.TempDir())
	profile := &Profile{ProfileId: "profile-a"}
	_ = mgr.ApplyDefaults(profile)
	if mgr.ApplyDefaults(profile) {
		t.Fatal("ApplyDefaults reported a change on the second pass without any intervening changes")
	}
}
