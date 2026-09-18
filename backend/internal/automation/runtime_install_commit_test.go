package automation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommitRuntimeDirectoryReplacesExistingRuntime(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	stagingDir := filepath.Join(root, "runtime.staging")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := commitRuntimeDirectory(stagingDir, runtimeDir); err != nil {
		t.Fatalf("commitRuntimeDirectory returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "new.txt")); err != nil {
		t.Fatalf("new runtime missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old runtime content still exists: %v", err)
	}
}

func TestCommitRuntimeDirectoryRestoresPreviousRuntimeOnCommitFailure(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	stagingDir := filepath.Join(runtimeDir, "staging")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(runtimeDir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := commitRuntimeDirectory(stagingDir, runtimeDir); err == nil {
		t.Fatal("commitRuntimeDirectory should fail when staging is inside runtimeDir")
	}
	data, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("previous runtime was not restored: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("restored runtime content = %q, want old", string(data))
	}
	if matches, err := filepath.Glob(runtimeDir + ".rollback-*"); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("rollback directory leaked after restore: %v", matches)
	}
}
