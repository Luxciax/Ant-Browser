package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/snapshot"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newSnapshotTestApp(t *testing.T) (*App, *BrowserProfile, string) {
	t.Helper()
	root := t.TempDir()
	cfg := config.DefaultConfig()
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)
	profile := &BrowserProfile{
		ProfileId:    "profile-snapshot-test",
		ProfileName:  "snapshot-test",
		UserDataDir:  "profile-snapshot-test",
		RuntimeState: browser.RuntimeStopped,
	}
	app.browserMgr.Profiles[profile.ProfileId] = profile
	userDataDir := app.browserMgr.ResolveUserDataDir(profile)
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return app, profile, userDataDir
}

func TestBrowserSnapshotCreateKeepsMaliciousNameInsideSnapshotDir(t *testing.T) {
	app, profile, userDataDir := newSnapshotTestApp(t)
	if err := os.WriteFile(filepath.Join(userDataDir, "marker.txt"), []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalName := `../../../outside\evil:name?*`
	info, err := app.BrowserSnapshotCreate(profile.ProfileId, originalName)
	if err != nil {
		t.Fatalf("BrowserSnapshotCreate returned error: %v", err)
	}
	if info.Name != originalName {
		t.Fatalf("snapshot display name changed: got %q want %q", info.Name, originalName)
	}
	snapDir, err := app.snapshotDir(profile.ProfileId)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("snapshot dir entries = %d, want 2", len(entries))
	}
	for _, entry := range entries {
		if strings.ContainsAny(entry.Name(), `/\`) || !strings.HasPrefix(entry.Name(), info.SnapshotId) {
			t.Fatalf("unsafe snapshot filename: %q", entry.Name())
		}
	}
}

func TestBrowserSnapshotRestoreInvalidZipPreservesExistingUserData(t *testing.T) {
	app, profile, userDataDir := newSnapshotTestApp(t)
	markerPath := filepath.Join(userDataDir, "marker.txt")
	if err := os.WriteFile(markerPath, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapDir, err := app.snapshotDir(profile.ProfileId)
	if err != nil {
		t.Fatal(err)
	}
	snapshotID := "invalid-snapshot"
	if err := os.WriteFile(filepath.Join(snapDir, snapshotID+"_bad.zip"), []byte("not-a-zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, snapshotID+"_bad.meta.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := app.BrowserSnapshotRestore(profile.ProfileId, snapshotID); err == nil {
		t.Fatal("BrowserSnapshotRestore should reject an invalid zip")
	}
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("existing user data was removed: %v", err)
	}
	if string(data) != "keep-me" {
		t.Fatalf("existing user data changed: %q", data)
	}
}

func TestBrowserSnapshotRestoreLimitFailurePreservesExistingUserData(t *testing.T) {
	app, profile, userDataDir := newSnapshotTestApp(t)
	markerPath := filepath.Join(userDataDir, "marker.txt")
	if err := os.WriteFile(markerPath, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "large.txt"), []byte("too-large"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapDir, err := app.snapshotDir(profile.ProfileId)
	if err != nil {
		t.Fatal(err)
	}
	snapshotID := "limited-snapshot"
	zipPath := filepath.Join(snapDir, snapshotID+"_limit.zip")
	if err := snapshot.ZipDir(sourceDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, snapshotID+"_limit.meta.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	previousLimits := browserSnapshotRestoreUnzipLimits
	browserSnapshotRestoreUnzipLimits = snapshot.UnzipLimits{
		MaxEntries:           10,
		MaxUncompressedBytes: 4,
		MaxSingleFileBytes:   4,
		MaxCompressedBytes:   1024 * 1024,
	}
	t.Cleanup(func() { browserSnapshotRestoreUnzipLimits = previousLimits })

	if err := app.BrowserSnapshotRestore(profile.ProfileId, snapshotID); err == nil {
		t.Fatal("BrowserSnapshotRestore should reject a snapshot that exceeds restore limits")
	}
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("existing user data was removed after limit rejection: %v", err)
	}
	if string(data) != "keep-me" {
		t.Fatalf("existing user data changed after limit rejection: %q", data)
	}
}

func TestBrowserSnapshotRestoreSwapsUserDataAfterSuccessfulExtraction(t *testing.T) {
	app, profile, userDataDir := newSnapshotTestApp(t)
	if err := os.WriteFile(filepath.Join(userDataDir, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapDir, err := app.snapshotDir(profile.ProfileId)
	if err != nil {
		t.Fatal(err)
	}
	snapshotID := "valid-snapshot"
	zipPath := filepath.Join(snapDir, snapshotID+"_good.zip")
	if err := snapshot.ZipDir(sourceDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, snapshotID+"_good.meta.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := app.BrowserSnapshotRestore(profile.ProfileId, snapshotID); err != nil {
		t.Fatalf("BrowserSnapshotRestore returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(userDataDir, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old user data still exists, stat error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(userDataDir, "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("restored data = %q, want new", data)
	}
}
