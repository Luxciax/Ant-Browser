package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectedExtensionDeletionDoesNotRewriteRunningProfile(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	a := NewApp(root)
	a.config = cfg
	a.browserMgr = browser.NewManager(cfg, root)
	db, err := database.NewDB(filepath.Join(root, "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao := browser.NewSQLiteExtensionDAO(db.GetConn())
	a.browserMgr.ExtensionDAO = dao
	a.browserMgr.InitData()
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	installDir := filepath.Join(root, "data", "extensions", id)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := dao.Upsert(browser.Extension{ExtensionID: id, InstallDir: installDir, InstallMode: browser.ExtensionInstallModePersistent, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := &browser.Profile{ProfileId: "live", UserDataDir: "live", Running: true, RuntimeState: browser.RuntimeRunning}
	a.browserMgr.Profiles[p.ProfileId] = p
	if err := dao.UpsertProfileExtensionRuntime(browser.ProfileExtensionRuntime{ProfileID: p.ProfileId, ExtensionID: id, RuntimeExtensionID: id, Status: browser.ExtensionRuntimeStatusInstalled}); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE audit_runtime_writes (id INTEGER)`,
		`CREATE TRIGGER count_runtime_writes BEFORE INSERT ON browser_profile_extension_runtime BEGIN INSERT INTO audit_runtime_writes VALUES (1); END`,
	} {
		if _, err := db.GetConn().Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.BrowserExtensionDelete(id); err == nil {
		t.Fatal("expected running-profile rejection")
	}
	var writes int
	if err := db.GetConn().QueryRow(`SELECT count(*) FROM audit_runtime_writes`).Scan(&writes); err != nil {
		t.Fatal(err)
	}
	if writes != 0 {
		t.Fatalf("rejected deletion performed %d runtime rollback writes on a live profile", writes)
	}
	stored, err := dao.Get(id)
	if err != nil || !stored.Enabled {
		t.Fatalf("catalog not preserved: %#v %v", stored, err)
	}
}
