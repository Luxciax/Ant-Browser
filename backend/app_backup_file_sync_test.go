package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupCopyFileReplacesExistingDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.bin")
	target := filepath.Join(root, "target.bin")
	if err := os.WriteFile(source, []byte("new backup data"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := backupCopyFile(source, target); err != nil {
		t.Fatalf("backupCopyFile returned error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new backup data" {
		t.Fatalf("target content = %q, want replacement data", string(data))
	}
	if matches, err := filepath.Glob(filepath.Join(root, ".backup-copy-*.tmp")); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("temporary files leaked: %v", matches)
	}
}
