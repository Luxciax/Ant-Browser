package backend

import (
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
)

func TestMigrateToSQLitePreservesLegacyProfileRuntimeSettings(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.Profiles = []config.BrowserProfileConfig{{
		ProfileId:          "legacy-profile",
		ProfileName:        "Legacy",
		UserDataDir:        "legacy-profile",
		RestoreLastSession: "enabled",
		MemoryLimitMB:      1536,
		FingerprintArgs:    []string{"--fingerprint-seed=123"},
		LaunchArgs:         []string{"--lang=en-US"},
	}}

	db, err := database.NewDB(filepath.Join(root, "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	manager := browser.NewManager(cfg, root)
	manager.CoreDAO = browser.NewSQLiteCoreDAO(db.GetConn())
	manager.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	manager.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())
	manager.BookmarkDAO = browser.NewSQLiteBookmarkDAO(db.GetConn())
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = manager

	app.migrateToSQLite()

	profile, err := manager.ProfileDAO.GetById("legacy-profile")
	if err != nil {
		t.Fatal(err)
	}
	if profile.RestoreLastSession != "enabled" {
		t.Fatalf("RestoreLastSession = %q, want enabled", profile.RestoreLastSession)
	}
	if profile.MemoryLimitMB != 1536 {
		t.Fatalf("MemoryLimitMB = %d, want 1536", profile.MemoryLimitMB)
	}
	if browser.NormalizeProfileRuntimeState(profile) != browser.RuntimeStopped {
		t.Fatalf("normalized RuntimeState = %q, want %q", browser.NormalizeProfileRuntimeState(profile), browser.RuntimeStopped)
	}
}
