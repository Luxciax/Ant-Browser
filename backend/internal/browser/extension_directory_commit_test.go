package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommitExtensionDirectoryReplacesExistingDirectory(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "extension")
	stagedDir := filepath.Join(root, "extension.tmp")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedDir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := commitExtensionDirectory(stagedDir, installDir); err != nil {
		t.Fatalf("commitExtensionDirectory returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "new.txt")); err != nil {
		t.Fatalf("new extension content missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old extension content still present: %v", err)
	}
	if matches, err := filepath.Glob(installDir + ".rollback-*"); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("rollback directory was not cleaned up: %v", matches)
	}
}

func TestCommitExtensionDirectoryLeavesExistingDirectoryWhenStagingIsInvalid(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "extension")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(installDir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := commitExtensionDirectory(filepath.Join(root, "missing-stage"), installDir); err == nil {
		t.Fatal("commitExtensionDirectory should reject a missing staging directory")
	}
	data, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("existing extension directory was damaged: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("existing content = %q, want old", string(data))
	}
}

func TestCommitExtensionDirectoryRestoresOriginalAfterCommitRenameFails(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "extension")
	stagedDir := filepath.Join(installDir, "stage")
	if err := os.MkdirAll(stagedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(installDir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedDir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Moving installDir to its rollback name also moves stagedDir, so the
	// subsequent stagedDir -> installDir rename must fail. The old directory
	// must then be restored from the rollback copy.
	if err := commitExtensionDirectory(stagedDir, installDir); err == nil {
		t.Fatal("commitExtensionDirectory should fail when staging is inside the installation directory")
	}
	data, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("original extension directory was not restored: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("restored content = %q, want old", string(data))
	}
	if matches, err := filepath.Glob(installDir + ".rollback-*"); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("rollback directory leaked after restore: %v", matches)
	}
}
