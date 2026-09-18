package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceCoreDirectoryRestoresOriginalWhenCommitFails(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "core")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "marker.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := replaceCoreDirectory(targetDir, filepath.Join(root, "missing-staging"), true)
	if err == nil {
		t.Fatal("replaceCoreDirectory returned nil error for missing staging directory")
	}
	if strings.Contains(err.Error(), "CORE_ROLLBACK_FAILED") {
		t.Fatalf("rollback unexpectedly failed: %v", err)
	}
	marker, readErr := os.ReadFile(filepath.Join(targetDir, "marker.txt"))
	if readErr != nil {
		t.Fatalf("original core directory was not restored: %v", readErr)
	}
	if string(marker) != "old" {
		t.Fatalf("restored marker = %q, want old", marker)
	}
}

func TestRestoreCoreDirectoryBackupReportsFailure(t *testing.T) {
	root := t.TempDir()
	err := restoreCoreDirectoryBackup(filepath.Join(root, "missing-backup"), filepath.Join(root, "core"))
	if err == nil || !strings.Contains(err.Error(), "恢复旧内核目录失败") {
		t.Fatalf("restore error = %v, want explicit rollback failure", err)
	}
}
