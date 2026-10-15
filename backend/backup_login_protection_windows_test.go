//go:build windows

package backend

import (
	"ant-chrome/backend/internal/browser"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutomaticBackupLoginProtection(t *testing.T) {
	app := newLoginBackupApp(t)
	if _, err := app.BackupSaveLoginProtection(true, "synthetic-recovery-password"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(app.loginProtectionPath())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("synthetic-recovery-password")) {
		t.Fatal("password stored in plaintext")
	}
	restarted := NewApp(app.appRoot)
	options, err := restarted.automaticBackupLoginOptions()
	if err != nil || !options.PortableLogin || !options.ProtectArchive || options.MigrationPassword != "synthetic-recovery-password" {
		t.Fatal("automatic options lost after restart")
	}
	writeLoginSnapshotFixture(t, app)
	zipPath := filepath.Join(t.TempDir(), "scheduled-path.zip")
	result, err := app.backupExportPackageToPath(zipPath)
	if err != nil || result["encryptedArchive"] != true {
		t.Fatal("scheduled export path did not protect the archive")
	}
	if format, err := detectBackupPackageFormat(zipPath); err != nil || format != encryptedBackupFormat {
		t.Fatal("not an encrypted container")
	}
	pending, err := app.backupRestorePackageFromPathLocked(zipPath)
	if err != nil || pending["requiresLoginPassword"] != true || pending["encryptedArchive"] != true {
		t.Fatal("encrypted archive not routed to password prompt")
	}
	if _, err := app.BackupRestoreWithLoginPassword(zipPath, "synthetic-wrong-password"); err == nil {
		t.Fatal("wrong password accepted through actual restore API")
	}
	// The password protection file must not be one of the uploaded archive files.
	plain := filepath.Join(t.TempDir(), "inner.zip")
	if err := decryptFullBackupArchive(zipPath, plain, "synthetic-recovery-password"); err != nil {
		t.Fatal(err)
	}
	inner, err := zip.OpenReader(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range inner.File {
		if filepath.Base(entry.Name) == backupLoginProtectionFile {
			t.Fatal("local credential file was included in backup")
		}
	}
	_ = inner.Close()
	if _, err := app.BackupSaveLoginProtection(false, ""); err != nil {
		t.Fatal(err)
	}
	if options, err := app.automaticBackupLoginOptions(); err != nil || options.PortableLogin {
		t.Fatal("disabled protection remained enabled")
	}
	if err := os.WriteFile(app.loginProtectionPath(), []byte("synthetic-corrupt-record"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.automaticBackupLoginOptions(); err == nil {
		t.Fatal("corrupt record downgraded to plain backup")
	}
	if _, err := app.BackupSaveLoginProtection(true, "synthetic-replacement-password"); err != nil {
		t.Fatal("explicit replacement cannot repair record")
	}
}

func TestLoginRestoreStartupFailureConfiguredKernel(t *testing.T) {
	kernel := os.Getenv("ANT_IDENTITY_TEST_CHROME")
	if kernel == "" {
		t.Skip("set ANT_IDENTITY_TEST_CHROME to the explicitly configured kernel")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "synthetic fixture") }))
	url := server.URL
	server.Close() // Deliberately unavailable loopback destination, no real site.
	app := newLoginBackupApp(t)
	dir := t.TempDir()
	state := portableLoginState{Version: 2, CapturedAt: time.Now().UTC(), Tabs: []string{url}, Pages: []portablePageSession{{URL: url, Storage: []portableSessionStorage{{Origin: url, Entries: [][2]string{{"synthetic_auth", "synthetic_retry"}}}}}}}
	if err := writeLocalLoginState(dir, loginRestoreFile, state); err != nil {
		t.Fatal(err)
	}
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	profile := &BrowserProfile{ProfileId: "synthetic-startup-failure", ProfileName: "Synthetic startup failure"}
	app.markProfileStartingLocked(profile)
	plan := &browserStartPlan{profile: profile, chromeBinaryPath: kernel, userDataDir: dir, assignedDebugPort: port,
		maxStartAttempts: 1, startReadyTimeout: 8 * time.Second, startStableWindow: 100 * time.Millisecond, totalReadyTimeout: 8 * time.Second,
		loginRestore: &state, deferredStartTargets: []string{url}, args: []string{"--headless=new", "--no-first-run", "--no-default-browser-check", "--disable-background-networking", "--no-startup-window", "--user-data-dir=" + dir, fmt.Sprintf("--remote-debugging-port=%d", port)}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = app.startBrowserProfileWithPlan(ctx, browserStartInput{ProfileID: profile.ProfileId}, plan)
	if err == nil || !strings.Contains(err.Error(), "登录态页面恢复未完成") {
		t.Fatalf("unexpected start result: %v", err)
	}
	if profile.Running || profile.DebugReady || len(app.browserMgr.BrowserProcesses) != 0 {
		t.Fatal("failed restore published a running instance")
	}
	if _, err := os.Stat(filepath.Join(dir, loginRestoreFile)); err != nil {
		t.Fatal("failed restore consumed retry marker")
	}
	if _, err := cdpGetEndpointBody(port, "/json/version"); err == nil {
		t.Fatal("failed restore left the spawned browser running")
	}
}

func TestEncryptedFullBackupActualRestoreAndHistory(t *testing.T) {
	source := newLoginBackupApp(t)
	profile := writeLoginSnapshotFixture(t, source)
	zipPath := filepath.Join(t.TempDir(), "full-encrypted.zip")
	_, err := source.backupExportPackageToPathWithOptions(zipPath, ProfilePackageExportOptions{PortableLogin: true, ProtectArchive: true, MigrationPassword: "synthetic-full-backup-password"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(backupMetadataPath(zipPath)); err != nil {
		t.Fatal(err)
	}
	metadata, err := readOrGenerateBackupMetadataForZip(zipPath, filepath.Base(zipPath))
	if err != nil || metadata.PackageType != "full" || len(metadata.Files) != 1 || metadata.Files[0].Path != "payload.bin" {
		t.Fatal("encrypted history inspection failed or exposed plaintext file list")
	}
	target := newLoginBackupApp(t)
	target.browserMgr.BookmarkDAO = browser.NewSQLiteBookmarkDAO(target.db.GetConn())
	t.Cleanup(target.stopAppOnlyRuntimeServices)
	result, err := target.BackupRestoreWithLoginPassword(zipPath, "synthetic-full-backup-password")
	if err != nil {
		t.Fatal(err)
	}
	if result["partial"] == true {
		t.Fatalf("actual encrypted restore partial: %v", result["failedComponents"])
	}
	if _, err := target.browserMgr.ProfileDAO.GetById(profile.ProfileId); err != nil {
		t.Fatal(err)
	}
	state, err := readLocalLoginState(target.browserMgr.ResolveUserDataDir(&profile), loginRestoreFile)
	if err != nil || len(state.Cookies) != 1 {
		t.Fatal("actual full restore lost pending cookie state")
	}
	response, _ := json.Marshal(result)
	if strings.Contains(string(response), "synthetic-full-backup-password") {
		t.Fatal("password returned to frontend")
	}
}
