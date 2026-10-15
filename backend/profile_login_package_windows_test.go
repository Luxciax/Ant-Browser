//go:build windows

package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newLoginBackupApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	db := newProfilePackageDatabase(t, root)
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = "profiles"
	app := NewApp(root)
	app.config = cfg
	app.db = db
	app.browserMgr = browser.NewManager(cfg, root)
	app.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())
	app.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(db.GetConn())
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	app.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(db.GetConn())
	app.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(db.GetConn())
	if err := os.MkdirAll(filepath.Join(root, "profiles"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(filepath.Join(root, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	return app
}

func writeLoginSnapshotFixture(t *testing.T, app *App) browser.Profile {
	t.Helper()
	profile := browser.Profile{ProfileId: "snapshot-source", ProfileName: "Synthetic login", UserDataDir: "snapshot-source", CreatedAt: time.Now().Format(time.RFC3339), UpdatedAt: time.Now().Format(time.RFC3339)}
	if err := app.browserMgr.ProfileDAO.(*browser.SQLiteProfileDAO).Upsert(&profile); err != nil {
		t.Fatal(err)
	}
	dir := app.browserMgr.ResolveUserDataDir(&profile)
	writePortableLoginTestLocalState(t, dir, []byte("0123456789abcdef0123456789abcdef"))
	state := portableLoginState{Version: 2, CapturedAt: time.Now().UTC(), Cookies: []portableCookie{{Name: "synthetic_session", Value: "synthetic-value-not-a-real-secret", Domain: "example.invalid", Path: "/", HTTPOnly: true, SameSite: "Lax"}}}
	if err := writeLocalLoginState(dir, loginSnapshotFile, state); err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(dir, "Default", "Local Storage", "leveldb")
	if err := os.MkdirAll(storage, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storage, "synthetic.ldb"), []byte("synthetic-storage"), 0600); err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestLoginFullBackupPasswordAndSameDeviceRestore(t *testing.T) {
	source := newLoginBackupApp(t)
	profile := writeLoginSnapshotFixture(t, source)
	zipPath := filepath.Join(t.TempDir(), "encrypted-login-backup.zip")
	result, err := source.backupExportPackageToPathWithOptions(zipPath, ProfilePackageExportOptions{PortableLogin: true, MigrationPassword: "synthetic-migration-password"})
	if err != nil {
		t.Fatal(err)
	}
	if result["portableLoginCount"] != 1 {
		t.Fatal("full backup lost login component")
	}
	target := newLoginBackupApp(t)
	pending, err := target.backupImportFromPathLocked(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if pending["requiresLoginPassword"] != true {
		t.Fatal("missing password prompt")
	}
	if profiles, err := target.browserMgr.ProfileDAO.List(); err != nil || len(profiles) != 0 {
		t.Fatal("password prompt mutated database")
	}
	if _, err := target.backupImportFromPathWithPasswordLocked(zipPath, "synthetic-wrong-password"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err := os.Stat(target.browserMgr.ResolveUserDataDir(&profile)); !os.IsNotExist(err) {
		t.Fatal("wrong password wrote live profile data")
	}
	// Validate and prepare the complete archived payload, then run the actual
	// import file routing without config reload/scheduler side effects in tests.
	extracted, manifest, err := backupExtractAndValidate(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(extracted)
	count, err := prepareFullBackupLogin(extracted, "synthetic-migration-password")
	if err != nil || count != 1 {
		t.Fatalf("prepare login count=%d err=%v", count, err)
	}
	stats := &backupMergeStats{}
	target.backupImportFileTrees(filepath.Join(extracted, "payload"), source.config, manifest, stats, func(_, _ string, err error) { t.Fatal(err) })
	restoredDir := target.browserMgr.ResolveUserDataDir(&profile)
	state, err := readLocalLoginState(restoredDir, loginRestoreFile)
	if err != nil || len(state.Cookies) != 1 {
		t.Fatal("full backup did not route locally encrypted pending login to restored profile")
	}
	storage, err := os.ReadFile(filepath.Join(restoredDir, "Default", "Local Storage", "leveldb", "synthetic.ldb"))
	if err != nil || string(storage) != "synthetic-storage" {
		t.Fatal("website storage not restored")
	}
	// Ordinary backups on the SAME Windows account also arm cookie restoration.
	if ok, err := prepareSameDeviceLogin(restoredDir); err != nil || !ok {
		t.Fatal("same-device cookie restoration not armed")
	}
}

func TestLoginSnapshotEncryptionAndValidation(t *testing.T) {
	dir := t.TempDir()
	state := portableLoginState{Version: 2, CapturedAt: time.Now(), Cookies: []portableCookie{{Name: "fixture", Value: "synthetic-cookie-value", Domain: "example.invalid", Path: "/"}}}
	if err := writeLocalLoginState(dir, loginSnapshotFile, state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, loginSnapshotFile))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(state.Cookies[0].Value)) {
		t.Fatal("cookie was stored in plaintext")
	}
	got, err := readLocalLoginState(dir, loginSnapshotFile)
	if err != nil || got.Cookies[0].Value != state.Cookies[0].Value {
		t.Fatal("local encrypted snapshot round-trip failed")
	}
	state.Tabs = []string{"file:///synthetic"}
	if validatePortableLoginState(state) == nil {
		t.Fatal("non-web startup URL accepted")
	}
	state.Tabs = nil
	state.Cookies[0].PartitionKey = json.RawMessage(`{"topLevelSite":"https://example.invalid","hasCrossSiteAncestor":true}`)
	if err := validatePortableLoginState(state); err != nil {
		t.Fatal(err)
	}
	one := portableCookieID(state.Cookies[0])
	state.Cookies[0].PartitionKey = json.RawMessage(`{"hasCrossSiteAncestor":true,"topLevelSite":"https://example.invalid"}`)
	if one != portableCookieID(state.Cookies[0]) {
		t.Fatal("partition identity depends on JSON field order")
	}
}

func TestLoginFullBackupRejectsTraversalBeforeLiveMutation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "payload"), 0700); err != nil {
		t.Fatal(err)
	}
	bundle := fullLoginBundle{Version: 1, Profiles: []fullLoginRecord{{ProfileID: "fixture", ArchiveDir: "../outside"}}}
	data, _ := json.Marshal(bundle)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(fullLoginArchivePath)), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareFullBackupLogin(root, "synthetic-password"); err == nil {
		t.Fatal("traversal accepted")
	}
}

func TestLoginPayloadVersionAuthentication(t *testing.T) {
	envelope, err := sealPortableLoginData([]byte("synthetic-cookie-payload"), "synthetic-password", 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openPortableLoginKey(*envelope, "synthetic-password"); err != nil {
		t.Fatal(err)
	}
	envelope.Version = 1
	envelope.PayloadType = ""
	if _, err := openPortableLoginKey(*envelope, "synthetic-password"); err == nil {
		t.Fatal("cookie payload was downgraded to a legacy key")
	}
}
