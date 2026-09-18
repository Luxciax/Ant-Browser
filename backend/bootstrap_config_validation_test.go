package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDoesNotAutoRepairSecurityValidationFailure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	original := "launch_server:\n  port: 19876\n  auth:\n    enabled: true\n    api_key: ''\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "配置校验失败") {
		t.Fatalf("LoadConfig error = %v, want security validation failure", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != original {
		t.Fatalf("invalid security config was rewritten:\n%s", got)
	}
	broken, globErr := filepath.Glob(path + ".broken-*")
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(broken) != 0 {
		t.Fatalf("security validation failure created broken-config backup: %#v", broken)
	}
}
