package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigRestoresLastSession(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ConfigVersion != CurrentConfigVersion {
		t.Fatalf("config_version = %d, want %d", cfg.ConfigVersion, CurrentConfigVersion)
	}
	if !cfg.Browser.RestoreLastSession {
		t.Fatal("default config should restore the last browser session")
	}
}

func TestLoadRejectsEnabledLaunchServerAuthWithoutAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("launch_server:\n  port: 19876\n  auth:\n    enabled: true\n    api_key: ''\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load returned nil error for enabled auth without key")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Load error = %T %v, want ValidationError", err, err)
	}
}

func TestSaveRejectsEnabledLaunchServerAuthWithoutAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LaunchServer.Auth.Enabled = true
	cfg.LaunchServer.Auth.APIKey = "  "
	err := cfg.Save(filepath.Join(t.TempDir(), "config.yaml"))
	if err == nil {
		t.Fatal("Save returned nil error for enabled auth without key")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Save error = %T %v, want ValidationError", err, err)
	}
}
