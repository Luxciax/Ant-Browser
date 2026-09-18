package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProxyCoreDirectorySwapRollbackRestoresPreviousInstall(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "xray")
	stagingDir := filepath.Join(root, "staging")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "old.exe"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "new.exe"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	swap, err := stageProxyCoreDirectory(stagingDir, installDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "new.exe")); err != nil {
		t.Fatalf("new proxy core was not staged: %v", err)
	}
	if err := swap.rollback(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(installDir, "old.exe"))
	if err != nil {
		t.Fatalf("old proxy core was not restored: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("restored data = %q, want old", string(data))
	}
}

func TestProxyCoreDirectorySwapFinishRemovesRollback(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "xray")
	stagingDir := filepath.Join(root, "staging")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "old.exe"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "new.exe"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	swap, err := stageProxyCoreDirectory(stagingDir, installDir)
	if err != nil {
		t.Fatal(err)
	}
	rollbackDir := swap.rollbackDir
	swap.finish()
	if _, err := os.Stat(rollbackDir); !os.IsNotExist(err) {
		t.Fatalf("rollback directory still exists after finish: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "new.exe")); err != nil {
		t.Fatalf("new proxy core missing after finish: %v", err)
	}
}
