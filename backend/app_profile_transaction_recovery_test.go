package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/profiletxn"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newProfileTransactionTestApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	a := NewApp(root)
	a.config = config.DefaultConfig()
	a.db = newProfilePackageDatabase(t, root)
	a.browserMgr = browser.NewManager(a.config, root)
	a.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(a.db.GetConn())
	a.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(a.db.GetConn())
	a.launchCodeSvc = launchcode.NewLaunchCodeService(launchcode.NewSQLiteLaunchCodeDAO(a.db.GetConn()))
	a.browserMgr.CodeProvider = a.launchCodeSvc
	return a
}

func TestDurableProfileDeletionKeepsRelationsAtomic(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "commit"
		if reject {
			name = "reject SQL"
		}
		t.Run(name, func(t *testing.T) {
			a := newProfileTransactionTestApp(t)
			p := &browser.Profile{ProfileId: "trash", UserDataDir: "trash", ProfileName: "trash"}
			if err := a.browserMgr.ProfileDAO.Upsert(p); err != nil {
				t.Fatal(err)
			}
			if err := a.browserMgr.ProfileDAO.SoftDelete(p.ProfileId, time.Now().Format(time.RFC3339)); err != nil {
				t.Fatal(err)
			}
			if _, err := a.launchCodeSvc.EnsureCode(p.ProfileId); err != nil {
				t.Fatal(err)
			}
			if _, err := a.browserMgr.ExtensionDAO.SetProfileSettings(p.ProfileId, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, true); err != nil {
				t.Fatal(err)
			}
			if err := a.browserMgr.ExtensionDAO.UpsertProfileExtensionRuntime(browser.ProfileExtensionRuntime{ProfileID: p.ProfileId, ExtensionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Status: browser.ExtensionRuntimeStatusInstalled}); err != nil {
				t.Fatal(err)
			}
			dir := a.browserMgr.ResolveUserDataDir(p)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "sentinel"), []byte("old-data"), 0600); err != nil {
				t.Fatal(err)
			}
			if reject {
				if _, err := a.db.GetConn().Exec(`CREATE TRIGGER reject_profile_delete BEFORE DELETE ON browser_profiles BEGIN SELECT RAISE(ABORT,'injected deletion failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			err := a.browserMgr.PermanentlyDelete(p.ProfileId)
			if (err != nil) != reject {
				t.Fatalf("delete: %v", err)
			}
			want := 0
			if reject {
				want = 1
			}
			for _, table := range []string{"browser_profiles", "browser_profile_extension_settings", "browser_profile_extensions", "browser_profile_extension_runtime", "launch_codes"} {
				var count int
				if err := a.db.GetConn().QueryRow("SELECT count(*) FROM "+table+" WHERE profile_id = ?", p.ProfileId).Scan(&count); err != nil || count != want {
					t.Fatalf("%s: %d want %d, %v", table, count, want, err)
				}
			}
			if _, found := a.launchCodeSvc.LookupCode(p.ProfileId); found != reject {
				t.Fatal("launch-code cache differs from DB")
			}
			data, err := os.ReadFile(filepath.Join(dir, "sentinel"))
			if reject {
				if err != nil || string(data) != "old-data" {
					t.Fatalf("failed deletion lost data: %q %v", data, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("committed deletion retained profile data")
			}
			if err := a.recoverProfileFileTransactions(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProfileImportJournalCommitFailureRollsBackFilesAndDatabase(t *testing.T) {
	a := newProfileTransactionTestApp(t)
	p := browser.Profile{ProfileId: "source", ProfileName: "source", UserDataDir: "source"}
	archive := filepath.Join(a.appRoot, "profile.zip")
	writeTestProfilePackage(t, archive, []browser.Profile{p}, map[string]string{"source/Default/Preferences": "new-data"})
	if _, err := a.db.GetConn().Exec(`CREATE TRIGGER reject_profile_commit BEFORE UPDATE OF committed ON profile_file_transactions BEGIN SELECT RAISE(ABORT,'injected commit marker failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.importProfilePackageFromPathWithMode(archive, profilePackageImportModeNew); err == nil {
		t.Fatal("expected marker failure")
	}
	for _, table := range []string{"browser_profiles", "profile_file_transactions"} {
		var count int
		if err := a.db.GetConn().QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rollback left %s rows: %d %v", table, count, err)
		}
	}
	if _, err := a.db.GetConn().Exec(`DROP TRIGGER reject_profile_commit`); err != nil {
		t.Fatal(err)
	}
	result, err := a.importProfilePackageFromPathWithMode(archive, profilePackageImportModeNew)
	if err != nil || result.ImportedCount != 1 {
		t.Fatalf("retry: %#v %v", result, err)
	}
	imported, err := a.browserMgr.ProfileDAO.GetById(result.ProfileMappings[p.ProfileId])
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(a.browserMgr.ResolveUserDataDir(imported), "Default", "Preferences"))
	if err != nil || string(data) != "new-data" {
		t.Fatalf("committed import lost its files: %q %v", data, err)
	}
	if err := a.recoverProfileFileTransactions(); err != nil {
		t.Fatal(err)
	}
}

func TestPendingProfileTransactionBlocksBrowserStartUntilStartupRecovery(t *testing.T) {
	a := newProfileTransactionTestApp(t)
	target := filepath.Join(a.resolveAppPath("data"), "pending")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	txn, err := profiletxn.Begin(a.db.GetConn(), profiletxn.Plan{Kind: "delete", Moves: []profiletxn.Move{{Target: target}}}, a.profileTransactionRoots())
	if err != nil {
		t.Fatal(err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.BrowserInstanceStart("missing-profile"); err == nil {
		t.Fatal("start allowed during pending recovery")
	}
	if err := a.browserMgr.CheckProfileFileTransactions(); err == nil {
		t.Fatal("pending transaction not detected")
	}
	if err := a.recoverProfileFileTransactions(); err != nil {
		t.Fatal(err)
	}
	if err := a.browserMgr.CheckProfileFileTransactions(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("startup recovery did not restore directory: %v", err)
	}
}
