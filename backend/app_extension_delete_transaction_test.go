package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserExtensionDeleteRollsBackEverythingWhenCatalogDeleteFails(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)

	db, err := database.NewDB(filepath.Join(root, "extensions-delete.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao := browser.NewSQLiteExtensionDAO(db.GetConn())
	app.browserMgr.ExtensionDAO = dao
	app.browserMgr.InitData()

	extensionID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	runtimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	installDir := filepath.Join(root, "data", "extensions", extensionID)
	packagePath := filepath.Join(root, "data", "extensions", "packages", extensionID, "1.0.0", "extension.crx")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "manifest.json"), []byte(`{"name":"Delete Rollback","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packagePath, []byte("package"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := dao.Upsert(browser.Extension{
		ExtensionID: extensionID,
		Name:        "Delete Rollback",
		Version:     "1.0.0",
		InstallDir:  installDir,
		InstallMode: browser.ExtensionInstallModePersistent,
		PackagePath: packagePath,
		Enabled:     true,
	}); err != nil {
		t.Fatal(err)
	}

	profile := &browser.Profile{ProfileId: "profile-a", ProfileName: "profile-a", UserDataDir: "profile-a", RuntimeState: browser.RuntimeStopped}
	app.browserMgr.Profiles[profile.ProfileId] = profile
	userDataDir := app.browserMgr.ResolveUserDataDir(profile)
	artifactDir := filepath.Join(userDataDir, "Default", "Extensions", runtimeID, "1.0.0")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(artifactDir, "manifest.json")
	if err := os.WriteFile(artifactPath, []byte(`{"name":"Runtime","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := dao.UpsertProfileExtensionRuntime(browser.ProfileExtensionRuntime{
		ProfileID:          profile.ProfileId,
		ExtensionID:        extensionID,
		RuntimeExtensionID: runtimeID,
		InstallMode:        browser.ExtensionInstallModePersistent,
		InstalledVersion:   "1.0.0",
		Status:             browser.ExtensionRuntimeStatusInstalled,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := db.GetConn().Exec(`CREATE TRIGGER reject_extension_delete BEFORE DELETE ON browser_extensions WHEN OLD.extension_id = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' BEGIN SELECT RAISE(ABORT, 'reject extension delete'); END;`); err != nil {
		t.Fatal(err)
	}

	if err := app.BrowserExtensionDelete(extensionID); err == nil {
		t.Fatal("BrowserExtensionDelete should fail when catalog delete is rejected")
	}
	stored, err := dao.Get(extensionID)
	if err != nil {
		t.Fatalf("extension catalog record was not restored: %v", err)
	}
	if !stored.Enabled {
		t.Fatal("extension enabled state was not restored")
	}
	if _, err := os.Stat(filepath.Join(installDir, "manifest.json")); err != nil {
		t.Fatalf("extension install directory was not restored: %v", err)
	}
	if _, err := os.Stat(packagePath); err != nil {
		t.Fatalf("extension package was not restored: %v", err)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("profile extension artifact was not restored: %v", err)
	}
	runtimeState, err := dao.GetProfileExtensionRuntime(profile.ProfileId, extensionID)
	if err != nil {
		t.Fatalf("profile runtime record was not restored: %v", err)
	}
	if runtimeState.Status != browser.ExtensionRuntimeStatusInstalled {
		t.Fatalf("runtime status = %q, want %q", runtimeState.Status, browser.ExtensionRuntimeStatusInstalled)
	}
}
