package browser

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
)

func newUserDataPathTestManager(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "data")
	return NewManager(cfg, root)
}

func TestNormalizeManagedUserDataDirRejectsAbsoluteAndTraversal(t *testing.T) {
	manager := newUserDataPathTestManager(t)

	if got, err := manager.normalizeManagedUserDataDir("", "profile-a"); err != nil || got != "profile-a" {
		t.Fatalf("blank path = %q, err=%v, want profile-a", got, err)
	}
	if got, err := manager.normalizeManagedUserDataDir(filepath.Join("nested", "profile-a"), "profile-a"); err != nil || got != filepath.Join("nested", "profile-a") {
		t.Fatalf("nested path = %q, err=%v", got, err)
	}
	if _, err := manager.normalizeManagedUserDataDir(filepath.Join("..", "escape"), "profile-a"); err == nil {
		t.Fatal("traversal path was accepted")
	}
	if _, err := manager.normalizeManagedUserDataDir(filepath.Join(t.TempDir(), "external"), "profile-a"); err == nil {
		t.Fatal("absolute path was accepted")
	}
}

func TestResolveUserDataDirFallsBackForPersistedInvalidPath(t *testing.T) {
	manager := newUserDataPathTestManager(t)
	root, err := manager.managedUserDataRoot()
	if err != nil {
		t.Fatal(err)
	}
	profile := &Profile{ProfileId: "profile-a", UserDataDir: filepath.Join("..", "outside")}
	got := manager.ResolveUserDataDir(profile)
	want := filepath.Join(root, "profile-a")
	if !samePath(got, want) {
		t.Fatalf("ResolveUserDataDir = %q, want safe fallback %q", got, want)
	}
}

func TestNormalizeManagedUserDataDirRejectsSymlinkEscape(t *testing.T) {
	manager := newUserDataPathTestManager(t)
	root, err := manager.managedUserDataRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "profile"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := manager.normalizeManagedUserDataDir(filepath.Join("link", "profile"), "profile-a"); err == nil {
		t.Fatal("symlink escape path was accepted")
	}
}
