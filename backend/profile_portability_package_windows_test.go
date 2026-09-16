//go:build windows

package backend

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestProfilePackagePortableLoginRoundTrip(t *testing.T) {
	const migrationPassword = "portable-login-test-password"
	masterKey := []byte("0123456789abcdef0123456789abcdef")

	sourceRoot := t.TempDir()
	sourceDB := newProfilePackageDatabase(t, sourceRoot)
	sourceCfg := config.DefaultConfig()
	sourceCfg.Browser.UserDataRoot = filepath.Join(sourceRoot, "user-data")
	sourceApp := NewApp(sourceRoot)
	sourceApp.config = sourceCfg
	sourceApp.db = sourceDB
	sourceApp.browserMgr = browser.NewManager(sourceCfg, sourceRoot)

	profile := browser.Profile{
		ProfileId:   "portable-source-profile",
		ProfileName: "Portable Login Source",
		UserDataDir: "portable-source-profile",
		CreatedAt:   "2026-09-16T00:00:00Z",
		UpdatedAt:   "2026-09-16T00:00:00Z",
	}
	sourceUserData := sourceApp.browserMgr.ResolveUserDataDir(&profile)
	writePortableLoginTestLocalState(t, sourceUserData, masterKey)
	cookiePath := filepath.Join(sourceUserData, "Default", "Network", "Cookies")
	if err := os.MkdirAll(filepath.Dir(cookiePath), 0o755); err != nil {
		t.Fatalf("create cookie directory: %v", err)
	}
	const cookieFixture = "encrypted-cookie-database-fixture"
	if err := os.WriteFile(cookiePath, []byte(cookieFixture), 0o600); err != nil {
		t.Fatalf("write cookie fixture: %v", err)
	}

	zipPath := filepath.Join(sourceRoot, "portable-profile.zip")
	fileCount, portableCount, warnings, err := sourceApp.writeProfilePackageWithOptions(
		zipPath,
		[]browser.Profile{profile},
		ProfilePackageExportOptions{PortableLogin: true, MigrationPassword: migrationPassword},
	)
	if err != nil {
		t.Fatalf("writeProfilePackageWithOptions: %v", err)
	}
	if fileCount == 0 || portableCount != 1 || len(warnings) != 0 {
		t.Fatalf("unexpected export result: files=%d portable=%d warnings=%v", fileCount, portableCount, warnings)
	}
	assertPortableLoginEnvelopeInPackage(t, zipPath, profile.ProfileId, masterKey)

	targetRoot := t.TempDir()
	targetDB := newProfilePackageDatabase(t, targetRoot)
	targetCfg := config.DefaultConfig()
	targetCfg.Browser.UserDataRoot = filepath.Join(targetRoot, "user-data")
	targetApp := NewApp(targetRoot)
	targetApp.config = targetCfg
	targetApp.db = targetDB
	targetApp.browserMgr = browser.NewManager(targetCfg, targetRoot)
	targetApp.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(targetDB.GetConn())
	targetApp.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(targetDB.GetConn())
	targetApp.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(targetDB.GetConn())
	targetApp.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(targetDB.GetConn())
	targetApp.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(targetDB.GetConn())

	preview, err := targetApp.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepareProfilePackageImportFromPath: %v", err)
	}
	if !preview.PortableLogin || !preview.RequiresMigrationPassword || preview.PortableLoginCount != 1 {
		t.Fatalf("portable-login preview not detected: %+v", preview)
	}

	result, err := targetApp.importProfilePackageFromPathWithModeAndActionsAndPassword(
		zipPath,
		profilePackageImportModeNew,
		true,
		nil,
		migrationPassword,
	)
	if err != nil {
		t.Fatalf("import portable profile package: %v", err)
	}
	newProfileID := result.ProfileMappings[profile.ProfileId]
	if newProfileID == "" || newProfileID == profile.ProfileId {
		t.Fatalf("unexpected profile mapping: %#v", result.ProfileMappings)
	}

	targetProfile, err := browser.NewSQLiteProfileDAO(targetDB.GetConn()).GetById(newProfileID)
	if err != nil {
		t.Fatalf("read imported profile: %v", err)
	}
	targetUserData := targetApp.browserMgr.ResolveUserDataDir(targetProfile)
	restoredKey, err := readAndUnprotectProfileOSCryptKey(targetUserData)
	if err != nil {
		t.Fatalf("read target os_crypt key: %v", err)
	}
	defer zeroSensitiveBytes(restoredKey)
	if !bytes.Equal(restoredKey, masterKey) {
		t.Fatal("target DPAPI key does not unwrap to the original browser master key")
	}

	gotCookie, err := os.ReadFile(filepath.Join(targetUserData, "Default", "Network", "Cookies"))
	if err != nil {
		t.Fatalf("read imported cookie database fixture: %v", err)
	}
	if string(gotCookie) != cookieFixture {
		t.Fatalf("cookie database changed during migration: %q", gotCookie)
	}
}

func TestProfilePackagePortableLoginRejectsWrongPasswordBeforeCommit(t *testing.T) {
	const migrationPassword = "portable-login-test-password"
	masterKey := []byte("abcdef0123456789abcdef0123456789")

	sourceRoot := t.TempDir()
	sourceDB := newProfilePackageDatabase(t, sourceRoot)
	sourceCfg := config.DefaultConfig()
	sourceCfg.Browser.UserDataRoot = filepath.Join(sourceRoot, "user-data")
	sourceApp := NewApp(sourceRoot)
	sourceApp.config = sourceCfg
	sourceApp.db = sourceDB
	sourceApp.browserMgr = browser.NewManager(sourceCfg, sourceRoot)
	profile := browser.Profile{ProfileId: "wrong-password-source", ProfileName: "Wrong Password Source", UserDataDir: "wrong-password-source"}
	writePortableLoginTestLocalState(t, sourceApp.browserMgr.ResolveUserDataDir(&profile), masterKey)

	zipPath := filepath.Join(sourceRoot, "portable-profile.zip")
	if _, count, _, err := sourceApp.writeProfilePackageWithOptions(
		zipPath,
		[]browser.Profile{profile},
		ProfilePackageExportOptions{PortableLogin: true, MigrationPassword: migrationPassword},
	); err != nil || count != 1 {
		t.Fatalf("export portable package: count=%d err=%v", count, err)
	}

	targetRoot := t.TempDir()
	targetDB := newProfilePackageDatabase(t, targetRoot)
	targetCfg := config.DefaultConfig()
	targetCfg.Browser.UserDataRoot = filepath.Join(targetRoot, "user-data")
	targetApp := NewApp(targetRoot)
	targetApp.config = targetCfg
	targetApp.db = targetDB
	targetApp.browserMgr = browser.NewManager(targetCfg, targetRoot)
	targetApp.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(targetDB.GetConn())
	targetApp.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(targetDB.GetConn())
	targetApp.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(targetDB.GetConn())
	targetApp.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(targetDB.GetConn())
	targetApp.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(targetDB.GetConn())

	result, err := targetApp.importProfilePackageFromPathWithModeAndActionsAndPassword(
		zipPath,
		profilePackageImportModeNew,
		true,
		nil,
		"definitely-wrong-password",
	)
	if err == nil {
		t.Fatalf("wrong migration password unexpectedly succeeded: %+v", result)
	}
	if len(result.ProfileMappings) != 0 {
		t.Fatalf("failed import returned mappings: %#v", result.ProfileMappings)
	}
	var storedCount int
	if err := targetDB.GetConn().QueryRow(`SELECT COUNT(*) FROM browser_profiles`).Scan(&storedCount); err != nil {
		t.Fatalf("count target profiles after failed import: %v", err)
	}
	if storedCount != 0 {
		t.Fatalf("wrong-password import committed %d browser_profiles rows", storedCount)
	}
	importsRoot := filepath.Join(targetCfg.Browser.UserDataRoot, ".imports")
	entries, readErr := os.ReadDir(importsRoot)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("read import staging root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("wrong-password import left staging batches behind: %v", entries)
	}
}

func writePortableLoginTestLocalState(t *testing.T, userDataDir string, masterKey []byte) {
	t.Helper()
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatalf("create user data directory: %v", err)
	}
	protected, err := protectProfileOSCryptKey(masterKey)
	if err != nil {
		t.Fatalf("protect test browser master key: %v", err)
	}
	defer zeroSensitiveBytes(protected)
	dpapiBlob := append([]byte("DPAPI"), protected...)
	defer zeroSensitiveBytes(dpapiBlob)
	state := map[string]any{
		"os_crypt": map[string]any{
			"encrypted_key": base64.StdEncoding.EncodeToString(dpapiBlob),
		},
		"browser": map[string]any{"fixture": true},
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal Local State fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDataDir, localStateFileName), data, 0o600); err != nil {
		t.Fatalf("write Local State fixture: %v", err)
	}
}

func assertPortableLoginEnvelopeInPackage(t *testing.T, zipPath string, profileID string, masterKey []byte) {
	t.Helper()
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported package: %v", err)
	}
	defer reader.Close()

	var envelope profilePortableLoginEnvelope
	if err := readProfilePackageJSON(reader.File, profilePortableLoginEntryName(profileID), &envelope); err != nil {
		t.Fatalf("read portable login envelope: %v", err)
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal portable login envelope: %v", err)
	}
	if bytes.Contains(encoded, masterKey) {
		t.Fatal("portable login envelope contains the raw browser master key")
	}
}
