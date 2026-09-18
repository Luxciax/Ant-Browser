package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentExtensionArtifactMatchesVersionedDirectory(t *testing.T) {
	userDataDir := t.TempDir()
	manifestDir := filepath.Join(userDataDir, "Default", "Extensions", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3_0")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "manifest.json"), []byte(`{"name":"Test","version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if !persistentExtensionArtifactMatches(userDataDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3") {
		t.Fatal("persistentExtensionArtifactMatches = false, want true for Chromium version directory suffix")
	}
	if got := persistentExtensionArtifactPath(userDataDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3"); got != manifestDir {
		t.Fatalf("persistentExtensionArtifactPath = %q, want %q", got, manifestDir)
	}
}

func TestPersistentExtensionCodePathUsesChromiumVersionDirectory(t *testing.T) {
	userDataDir := t.TempDir()
	want := filepath.Join(userDataDir, "Default", "Extensions", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3_0")
	if got := persistentExtensionCodePath(userDataDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3"); got != want {
		t.Fatalf("persistentExtensionCodePath = %q, want %q", got, want)
	}
}

func TestInstallExtensionPackageIntoProfileWritesProfileScopedCode(t *testing.T) {
	userDataDir := t.TempDir()
	packagePath := filepath.Join(t.TempDir(), "test.crx")
	const version = "1.2.3"
	publicKey := []byte("test-public-key")
	extensionID := extensionIDFromPublicKey(publicKey)

	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	manifestWriter, err := zipWriter.Create("manifest.json")
	if err != nil {
		t.Fatalf("Create manifest returned error: %v", err)
	}
	if _, err := manifestWriter.Write([]byte(`{"name":"Test","version":"1.2.3"}`)); err != nil {
		t.Fatalf("Write manifest returned error: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("Close archive returned error: %v", err)
	}
	signature := []byte("test-signature")
	packageData := make([]byte, 16+len(publicKey)+len(signature))
	copy(packageData[:4], []byte("Cr24"))
	binary.LittleEndian.PutUint32(packageData[4:8], 2)
	binary.LittleEndian.PutUint32(packageData[8:12], uint32(len(publicKey)))
	binary.LittleEndian.PutUint32(packageData[12:16], uint32(len(signature)))
	copy(packageData[16:], publicKey)
	copy(packageData[16+len(publicKey):], signature)
	packageData = append(packageData, archive.Bytes()...)
	if err := os.WriteFile(packagePath, packageData, 0o644); err != nil {
		t.Fatalf("WriteFile package returned error: %v", err)
	}

	runtimeID, err := installExtensionPackageIntoProfile(userDataDir, packagePath, Extension{
		ExtensionID: extensionID,
		Name:        "Test",
		Version:     version,
	})
	if err != nil {
		t.Fatalf("installExtensionPackageIntoProfile returned error: %v", err)
	}
	if runtimeID != extensionID {
		t.Fatalf("runtime ID = %q, want %q", runtimeID, extensionID)
	}
	targetPath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	manifestData, err := os.ReadFile(filepath.Join(targetPath, "manifest.json"))
	if err != nil {
		t.Fatalf("Read installed manifest returned error: %v", err)
	}
	var installedManifest map[string]any
	if err := json.Unmarshal(manifestData, &installedManifest); err != nil {
		t.Fatalf("Unmarshal installed manifest returned error: %v", err)
	}
	if installedManifest["name"] != "Test" || installedManifest["version"] != version || installedManifest["key"] == nil {
		t.Fatalf("installed manifest = %s", manifestData)
	}
	if !persistentExtensionArtifactMatches(userDataDir, runtimeID, version) {
		t.Fatal("persistentExtensionArtifactMatches = false after profile-scoped installation")
	}
}

func TestPrepareProfileExtensionsUsesLocalPackageWithoutLaunchingBrowserHelper(t *testing.T) {
	appRoot := t.TempDir()
	userDataDir := filepath.Join(appRoot, "profile")
	packagePath := filepath.Join(appRoot, "extension.crx")
	const version = "1.0.0"
	identityBytes := []byte("prepare-profile-local-package-key")
	runtimeID := extensionIDFromPublicKey(identityBytes)
	packageData := buildTestCRX2Package(t, identityBytes, version)
	if err := os.WriteFile(packagePath, packageData, 0o644); err != nil {
		t.Fatalf("WriteFile package returned error: %v", err)
	}

	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID:    runtimeID,
		Name:           "Local package",
		Version:        version,
		PackagePath:    packagePath,
		PackageHash:    extensionPackageHash(packageData),
		InstallMode:    ExtensionInstallModePersistent,
		Enabled:        true,
		DefaultInstall: true,
	}); err != nil {
		t.Fatalf("Upsert extension returned error: %v", err)
	}

	profile := &Profile{ProfileId: "profile", ProfileName: "profile", UserDataDir: userDataDir}
	dirs, warnings := manager.PrepareProfileExtensions(profile, `Z:\definitely-missing\chrome.exe`, userDataDir, []string{"--remote-debugging-port=9222"})
	if len(warnings) != 0 {
		t.Fatalf("PrepareProfileExtensions warnings = %v, want none", warnings)
	}
	if len(dirs) != 1 {
		t.Fatalf("PrepareProfileExtensions dirs = %#v, want one prepared extension", dirs)
	}
	expectedPath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	if !sameProfileExtensionPath(dirs[0], expectedPath) {
		t.Fatalf("prepared extension path = %q, want %q", dirs[0], expectedPath)
	}
	if _, err := os.Stat(filepath.Join(expectedPath, "manifest.json")); err != nil {
		t.Fatalf("prepared manifest missing: %v", err)
	}
	runtimeState, err := manager.ExtensionDAO.GetProfileExtensionRuntime(profile.ProfileId, runtimeID)
	if err != nil {
		t.Fatalf("GetProfileExtensionRuntime returned error: %v", err)
	}
	if runtimeState.Status != ExtensionRuntimeStatusInstalled || runtimeState.RuntimeExtensionID != runtimeID {
		t.Fatalf("runtime state = %#v, want locally installed runtime", runtimeState)
	}
}

func TestProfileExtensionRegistrationUsesChromeRelativePathAndClearsMAC(t *testing.T) {
	userDataDir := t.TempDir()
	runtimeID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	version := "1.2.3"
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), []byte(`{"name":"Test","version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	root := profileExtensionJSON{
		"extensions": profileExtensionJSON{
			"settings": profileExtensionJSON{
				runtimeID: profileExtensionJSON{
					"location": 8,
					"path":     `E:\\shared\\extension`,
				},
			},
		},
		"protection": profileExtensionJSON{
			"macs": profileExtensionJSON{
				"extensions": profileExtensionJSON{
					"settings":                profileExtensionJSON{runtimeID: "old-mac"},
					"settings_encrypted_hash": profileExtensionJSON{runtimeID: "old-hash"},
				},
			},
		},
	}
	if err := writeProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), root); err != nil {
		t.Fatalf("writeProfileJSON returned error: %v", err)
	}

	if err := ensureProfileScopedExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("ensureProfileScopedExtensionRegistration returned error: %v", err)
	}
	storedRoot, err := readProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), false)
	if err != nil {
		t.Fatalf("readProfileJSON returned error: %v", err)
	}
	storedSetting := storedRoot["extensions"].(map[string]any)["settings"].(map[string]any)[runtimeID].(map[string]any)
	if storedSetting["location"] != float64(1) {
		t.Fatalf("location = %#v, want 1", storedSetting["location"])
	}
	expectedPath, err := profileExtensionRelativePath(userDataDir, codePath)
	if err != nil {
		t.Fatalf("profileExtensionRelativePath returned error: %v", err)
	}
	if storedSetting["path"] != expectedPath {
		t.Fatalf("path = %#v, want %q", storedSetting["path"], expectedPath)
	}
	macs := storedRoot["protection"].(map[string]any)["macs"].(map[string]any)["extensions"].(map[string]any)
	if _, exists := macs["settings"].(map[string]any)[runtimeID]; exists {
		t.Fatal("old settings MAC was not removed")
	}
	if _, exists := macs["settings_encrypted_hash"].(map[string]any)[runtimeID]; exists {
		t.Fatal("old encrypted settings hash was not removed")
	}
	if !profileExtensionSettingMatches(userDataDir, runtimeID, version) {
		t.Fatal("profileExtensionSettingMatches = false for a valid profile registration")
	}
	storedSetting["location"] = float64(3)
	storedSetting["path"] = expectedPath
	if err := writeProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), storedRoot); err != nil {
		t.Fatalf("writeProfileJSON external registration returned error: %v", err)
	}
	if !profileExtensionSettingMatches(userDataDir, runtimeID, version) {
		t.Fatal("profileExtensionSettingMatches = false for a valid external profile registration")
	}
	if err := ensureProfileExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("ensureProfileExtensionRegistration returned error: %v", err)
	}
	storedRoot, err = readProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), false)
	if err != nil {
		t.Fatalf("readProfileJSON external registration returned error: %v", err)
	}
	storedSetting = storedRoot["extensions"].(map[string]any)["settings"].(map[string]any)[runtimeID].(map[string]any)
	if storedSetting["location"] != float64(3) {
		t.Fatalf("external registration location = %#v, want 3", storedSetting["location"])
	}

	storedSetting["path"] = `wrong\\path`
	if err := writeProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), storedRoot); err != nil {
		t.Fatalf("writeProfileJSON wrong path returned error: %v", err)
	}
	if profileExtensionSettingMatches(userDataDir, runtimeID, version) {
		t.Fatal("profileExtensionSettingMatches = true for a wrong profile path")
	}
}

func TestCleanupProfileExtensionRuntimeKeepsWorkflowStorageAndRemovesRegistration(t *testing.T) {
	userDataDir := t.TempDir()
	runtimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, "1.0.0")
	workflowPath := filepath.Join(userDataDir, "Default", "Local Extension Settings", runtimeID, "workflow.json")
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatalf("MkdirAll code returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("MkdirAll workflow returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), []byte(`{"name":"Test","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest returned error: %v", err)
	}
	if err := os.WriteFile(workflowPath, []byte("workflow"), 0o644); err != nil {
		t.Fatalf("WriteFile workflow returned error: %v", err)
	}
	if err := ensureProfileScopedExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("ensureProfileScopedExtensionRegistration returned error: %v", err)
	}
	if err := cleanupProfileExtensionRuntime(userDataDir, runtimeID); err != nil {
		t.Fatalf("cleanupProfileExtensionRuntime returned error: %v", err)
	}
	if _, err := os.Stat(codePath); !os.IsNotExist(err) {
		t.Fatalf("code path still exists, stat error = %v", err)
	}
	if _, err := os.Stat(workflowPath); err != nil {
		t.Fatalf("workflow storage was removed: %v", err)
	}
	if profileExtensionSettingMatches(userDataDir, runtimeID, "1.0.0") {
		t.Fatal("profile extension registration still matches after cleanup")
	}
}

func TestRepeatedProfileExtensionRegistrationPreservesRuntimeStorage(t *testing.T) {
	userDataDir := t.TempDir()
	runtimeID := "cccccccccccccccccccccccccccccccc"
	version := "1.0.0"
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatalf("MkdirAll code returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), []byte(`{"name":"Script state test","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest returned error: %v", err)
	}

	if err := ensureProfileScopedExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("initial registration returned error: %v", err)
	}
	storagePath := filepath.Join(userDataDir, "Default", "Local Extension Settings", runtimeID, "CURRENT")
	if err := os.MkdirAll(filepath.Dir(storagePath), 0o755); err != nil {
		t.Fatalf("MkdirAll storage returned error: %v", err)
	}
	const state = "scriptcat-user-scripts-and-settings"
	if err := os.WriteFile(storagePath, []byte(state), 0o644); err != nil {
		t.Fatalf("WriteFile storage returned error: %v", err)
	}

	// Startup verification for an already-installed extension must only repair
	// registration metadata. It must never replace or clean the runtime storage.
	if err := ensureProfileExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("repeat registration returned error: %v", err)
	}
	data, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("ReadFile storage after repeat registration returned error: %v", err)
	}
	if string(data) != state {
		t.Fatalf("extension runtime storage changed across repeat registration: %q", data)
	}
	if !profileExtensionSettingMatches(userDataDir, runtimeID, version) {
		t.Fatal("profile registration no longer matches after repeat verification")
	}
}

func TestRecoverExistingPersistentExtensionRuntimePreservesBrowserStorage(t *testing.T) {
	appRoot := t.TempDir()
	userDataDir := filepath.Join(appRoot, "profile")
	packagePath := filepath.Join(appRoot, "scriptcat.crx")
	const version = "1.2.3"
	publicKey := []byte("scriptcat-stable-public-key")
	runtimeID := extensionIDFromPublicKey(publicKey)

	packageData := buildTestCRX2Package(t, publicKey, version)
	if err := os.WriteFile(packagePath, packageData, 0o644); err != nil {
		t.Fatalf("WriteFile package returned error: %v", err)
	}
	extension := Extension{
		ExtensionID: runtimeID,
		Name:        "ScriptCat",
		Version:     version,
		PackagePath: packagePath,
		InstallMode: ExtensionInstallModePersistent,
		Enabled:     true,
	}
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatalf("MkdirAll code returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), []byte(`{"name":"ScriptCat","version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest returned error: %v", err)
	}
	const codeMarker = "existing-profile-extension-code-must-not-be-replaced"
	if err := os.WriteFile(filepath.Join(codePath, "marker.txt"), []byte(codeMarker), 0o644); err != nil {
		t.Fatalf("WriteFile marker returned error: %v", err)
	}
	if err := ensureProfileScopedExtensionRegistration(userDataDir, codePath, runtimeID, packagePath); err != nil {
		t.Fatalf("ensureProfileScopedExtensionRegistration returned error: %v", err)
	}

	storageFiles := map[string]string{
		filepath.Join(userDataDir, "Default", "Local Extension Settings", runtimeID, "CURRENT"):                         "scriptcat-local-settings",
		filepath.Join(userDataDir, "Default", "Sync Extension Settings", runtimeID, "CURRENT"):                          "scriptcat-sync-settings",
		filepath.Join(userDataDir, "Default", "IndexedDB", "chrome-extension_"+runtimeID+"_0.indexeddb.leveldb", "LOG"): "scriptcat-indexeddb",
		filepath.Join(userDataDir, "Default", "Service Worker", "Database", "LOG"):                                      "scriptcat-service-worker",
	}
	for path, contents := range storageFiles {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll storage %q returned error: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile storage %q returned error: %v", path, err)
		}
	}

	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	profile := &Profile{ProfileId: "profile", ProfileName: "profile", UserDataDir: userDataDir}
	packageHash := extensionPackageHash(packageData)
	artifactPath, recovered, err := manager.recoverExistingPersistentExtensionRuntime(profile, userDataDir, packagePath, packageHash, extension)
	if err != nil {
		t.Fatalf("recoverExistingPersistentExtensionRuntime returned error: %v", err)
	}
	if !recovered {
		t.Fatal("recoverExistingPersistentExtensionRuntime = false, want true")
	}
	if !sameProfileExtensionPath(artifactPath, codePath) {
		t.Fatalf("recovered artifact path = %q, want %q", artifactPath, codePath)
	}
	storedRuntime, err := manager.ExtensionDAO.GetProfileExtensionRuntime(profile.ProfileId, extension.ExtensionID)
	if err != nil {
		t.Fatalf("GetProfileExtensionRuntime returned error: %v", err)
	}
	if storedRuntime.RuntimeExtensionID != runtimeID || storedRuntime.Status != ExtensionRuntimeStatusInstalled || storedRuntime.PackageHash != packageHash {
		t.Fatalf("recovered runtime = %#v", storedRuntime)
	}
	marker, err := os.ReadFile(filepath.Join(codePath, "marker.txt"))
	if err != nil || string(marker) != codeMarker {
		t.Fatalf("existing extension code changed during recovery: data=%q err=%v", marker, err)
	}
	for path, want := range storageFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile storage %q returned error: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("storage %q changed during recovery: %q", path, data)
		}
	}
}

func TestRecoverExistingPersistentExtensionRuntimeRejectsMismatchedRegistration(t *testing.T) {
	appRoot := t.TempDir()
	userDataDir := filepath.Join(appRoot, "profile")
	packagePath := filepath.Join(appRoot, "extension.crx")
	const version = "1.2.3"
	publicKey := []byte("registration-mismatch-public-key")
	runtimeID := extensionIDFromPublicKey(publicKey)
	packageData := buildTestCRX2Package(t, publicKey, version)
	if err := os.WriteFile(packagePath, packageData, 0o644); err != nil {
		t.Fatalf("WriteFile package returned error: %v", err)
	}
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatalf("MkdirAll code returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), []byte(`{"name":"Test","version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest returned error: %v", err)
	}
	wrongPath := filepath.Join(userDataDir, "Default", "Extensions", runtimeID, "wrong_0")
	root := profileExtensionJSON{
		"extensions": profileExtensionJSON{
			"settings": profileExtensionJSON{
				runtimeID: profileExtensionJSON{
					"location": float64(1),
					"path":     wrongPath,
				},
			},
		},
	}
	if err := writeProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), root); err != nil {
		t.Fatalf("writeProfileJSON returned error: %v", err)
	}

	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	profile := &Profile{ProfileId: "profile", ProfileName: "profile", UserDataDir: userDataDir}
	extension := Extension{ExtensionID: runtimeID, Name: "Test", Version: version, PackagePath: packagePath, InstallMode: ExtensionInstallModePersistent}
	if _, recovered, err := manager.recoverExistingPersistentExtensionRuntime(profile, userDataDir, packagePath, extensionPackageHash(packageData), extension); err != nil {
		t.Fatalf("recoverExistingPersistentExtensionRuntime returned error: %v", err)
	} else if recovered {
		t.Fatal("recoverExistingPersistentExtensionRuntime = true for mismatched Secure Preferences registration")
	}
	if _, err := manager.ExtensionDAO.GetProfileExtensionRuntime(profile.ProfileId, extension.ExtensionID); err != sql.ErrNoRows {
		t.Fatalf("runtime row was created for invalid registration: %v", err)
	}
}

func buildTestCRX2Package(t *testing.T, publicKey []byte, version string) []byte {
	t.Helper()
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	manifestWriter, err := zipWriter.Create("manifest.json")
	if err != nil {
		t.Fatalf("Create manifest returned error: %v", err)
	}
	manifestData, err := json.Marshal(map[string]string{"name": "Test", "version": version})
	if err != nil {
		t.Fatalf("Marshal manifest returned error: %v", err)
	}
	if _, err := manifestWriter.Write(manifestData); err != nil {
		t.Fatalf("Write manifest returned error: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("Close archive returned error: %v", err)
	}
	signature := []byte("test-signature")
	packageData := make([]byte, 16+len(publicKey)+len(signature))
	copy(packageData[:4], []byte("Cr24"))
	binary.LittleEndian.PutUint32(packageData[4:8], 2)
	binary.LittleEndian.PutUint32(packageData[8:12], uint32(len(publicKey)))
	binary.LittleEndian.PutUint32(packageData[12:16], uint32(len(signature)))
	copy(packageData[16:], publicKey)
	copy(packageData[16+len(publicKey):], signature)
	return append(packageData, archive.Bytes()...)
}

func TestMigrateExtensionStoragePrefersLegacyData(t *testing.T) {
	userDataDir := t.TempDir()
	oldRuntimeID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newRuntimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	oldPath := filepath.Join(userDataDir, "Default", "Local Extension Settings", oldRuntimeID)
	newPath := filepath.Join(userDataDir, "Default", "Local Extension Settings", newRuntimeID)
	if err := os.MkdirAll(oldPath, 0o755); err != nil {
		t.Fatalf("MkdirAll old path returned error: %v", err)
	}
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		t.Fatalf("MkdirAll new path returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldPath, "legacy.log"), []byte("legacy"), 0o644); err != nil {
		t.Fatalf("WriteFile old path returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newPath, "default.log"), []byte("default"), 0o644); err != nil {
		t.Fatalf("WriteFile new path returned error: %v", err)
	}

	if err := migrateExtensionStorage(userDataDir, oldRuntimeID, newRuntimeID); err != nil {
		t.Fatalf("migrateExtensionStorage returned error: %v", err)
	}
	legacyData, err := os.ReadFile(filepath.Join(newPath, "legacy.log"))
	if err != nil {
		t.Fatalf("ReadFile migrated data returned error: %v", err)
	}
	if string(legacyData) != "legacy" {
		t.Fatalf("migrated data = %q, want legacy data", legacyData)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old storage path still exists, stat error = %v", err)
	}
}

func TestResolveExtensionPackageUsesCurrentFileHash(t *testing.T) {
	appRoot := t.TempDir()
	packagePath := filepath.Join(appRoot, "extension.crx")
	packageData := []byte("Cr24-current-package")
	if err := os.WriteFile(packagePath, packageData, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	manager := NewManager(config.DefaultConfig(), appRoot)
	_, packageHash, err := manager.resolveExtensionPackage(Extension{
		PackagePath: packagePath,
		PackageHash: "stale-hash-from-database",
	}, "")
	if err != nil {
		t.Fatalf("resolveExtensionPackage returned error: %v", err)
	}
	if packageHash != extensionPackageHash(packageData) {
		t.Fatalf("package hash = %q, want current file hash %q", packageHash, extensionPackageHash(packageData))
	}
}

func TestBackupProfileExtensionStateIncludesIndexedDBAndServiceWorker(t *testing.T) {
	appRoot := t.TempDir()
	userDataDir := filepath.Join(appRoot, "profile")
	runtimeID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	indexedDBPath := filepath.Join(userDataDir, "Default", "IndexedDB", "https_example_"+runtimeID+"_0.indexeddb.leveldb")
	serviceWorkerPath := filepath.Join(userDataDir, "Default", "Service Worker", "Database", runtimeID+"-worker")
	for _, path := range []string{indexedDBPath, serviceWorkerPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
	}

	manager := NewManager(config.DefaultConfig(), appRoot)
	backupPath, err := manager.backupProfileExtensionState("profile", runtimeID, userDataDir, []string{runtimeID})
	if err != nil {
		t.Fatalf("backupProfileExtensionState returned error: %v", err)
	}
	for _, relativePath := range []string{
		filepath.Join("IndexedDB", filepath.Base(indexedDBPath)),
		filepath.Join("Service Worker", "Database", filepath.Base(serviceWorkerPath)),
	} {
		if _, err := os.Stat(filepath.Join(backupPath, relativePath)); err != nil {
			t.Fatalf("backup entry %q not found: %v", relativePath, err)
		}
	}

	if err := os.RemoveAll(filepath.Join(userDataDir, "Default", "IndexedDB")); err != nil {
		t.Fatalf("RemoveAll IndexedDB returned error: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(userDataDir, "Default", "Service Worker")); err != nil {
		t.Fatalf("RemoveAll Service Worker returned error: %v", err)
	}
	if err := restoreProfileExtensionState(userDataDir, backupPath, []string{runtimeID}, ""); err != nil {
		t.Fatalf("restoreProfileExtensionState returned error: %v", err)
	}
	for _, path := range []string{indexedDBPath, serviceWorkerPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("restored entry %q not found: %v", path, err)
		}
	}
}

func TestRemoveExtensionFromStoppedProfilesRemovesPersistentCodeWithoutRuntimeState(t *testing.T) {
	appRoot := t.TempDir()
	userDataDir := filepath.Join(appRoot, "data", "profile")
	runtimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	codeDir := filepath.Join(userDataDir, "Default", "Extensions", runtimeID)
	if err := os.MkdirAll(codeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.Profiles["profile"] = &Profile{
		ProfileId:   "profile",
		ProfileName: "profile",
		UserDataDir: "profile",
		Running:     false,
	}
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID: runtimeID,
		Name:        "Persistent test",
		Version:     "1.0.0",
		InstallMode: ExtensionInstallModePersistent,
		Enabled:     false,
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	if err := manager.RemoveExtensionFromStoppedProfiles(runtimeID); err != nil {
		t.Fatalf("RemoveExtensionFromStoppedProfiles returned error: %v", err)
	}
	if _, err := os.Stat(codeDir); !os.IsNotExist(err) {
		t.Fatalf("persistent code directory still exists, stat error = %v", err)
	}
}

func TestExtensionIDFromCRX2PublicKey(t *testing.T) {
	publicKey := []byte("test-public-key")
	data := make([]byte, 16+len(publicKey)+4)
	copy(data[:4], []byte("Cr24"))
	binary.LittleEndian.PutUint32(data[4:8], 2)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(publicKey)))
	binary.LittleEndian.PutUint32(data[12:16], 4)
	copy(data[16:], publicKey)
	if extensionIDFromCRX(data) == "" {
		t.Fatal("extensionIDFromCRX returned empty ID for valid CRX2 header")
	}
}

func TestExtensionIDFromCRX3DeclaredID(t *testing.T) {
	rawID, err := hex.DecodeString("8d5ff66de04dc50615ad5a0d2f1b9220")
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	signedHeaderData := appendCRX3LengthDelimitedField(nil, 1, rawID)
	header := appendCRX3LengthDelimitedField(nil, 10000, signedHeaderData)
	data := make([]byte, 12+len(header))
	copy(data[:4], []byte("Cr24"))
	binary.LittleEndian.PutUint32(data[4:8], 3)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(header)))
	copy(data[12:], header)

	const want = "infppggnoaenmfagbfknfkancpbljcca"
	if got := extensionIDFromCRX(data); got != want {
		t.Fatalf("extensionIDFromCRX = %q, want %q", got, want)
	}
}

func appendCRX3LengthDelimitedField(data []byte, fieldNumber uint64, value []byte) []byte {
	data = appendCRX3Varint(data, fieldNumber<<3|2)
	data = appendCRX3Varint(data, uint64(len(value)))
	return append(data, value...)
}

func appendCRX3Varint(data []byte, value uint64) []byte {
	for value >= 0x80 {
		data = append(data, byte(value)|0x80)
		value >>= 7
	}
	return append(data, byte(value))
}

func TestLegacyDirectorySourceUsesPersistentInstallMode(t *testing.T) {
	appRoot := t.TempDir()
	directorySource := filepath.Join(appRoot, "source-extension")
	if err := os.MkdirAll(directorySource, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	manager := NewManager(config.DefaultConfig(), appRoot)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	dao := newTestExtensionDAO(t, appRoot)
	manager.ExtensionDAO = dao
	legacy := Extension{
		ExtensionID: "cccccccccccccccccccccccccccccccc",
		Name:        "legacy directory",
		Version:     "1.0.0",
		SourceURL:   directorySource,
		InstallDir:  filepath.Join(appRoot, "stored-extension"),
		Enabled:     true,
	}
	if err := dao.Upsert(legacy); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	stored, err := dao.Get(legacy.ExtensionID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.InstallMode != ExtensionInstallModePersistent {
		t.Fatalf("legacy InstallMode = %q, want persistent", stored.InstallMode)
	}
}

func TestPrepareProfileExtensionsContextHonorsCancelledContext(t *testing.T) {
	appRoot := t.TempDir()
	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dirs, warnings, err := manager.PrepareProfileExtensionsContext(ctx, &Profile{ProfileId: "profile"}, `C:\chrome.exe`, filepath.Join(appRoot, "profile"), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PrepareProfileExtensionsContext error = %v, want context.Canceled", err)
	}
	if len(dirs) != 0 || len(warnings) != 0 {
		t.Fatalf("cancelled preparation returned dirs=%v warnings=%v", dirs, warnings)
	}
}

func TestRunExtensionInstallerCommandHonorsContext(t *testing.T) {
	if os.Getenv("ANT_EXTENSION_INSTALLER_HELPER") == "1" {
		time.Sleep(5 * time.Second)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	command := exec.Command(os.Args[0], "-test.run=TestRunExtensionInstallerCommandHonorsContext")
	command.Env = append(os.Environ(), "ANT_EXTENSION_INSTALLER_HELPER=1")
	startedAt := time.Now()
	err := runExtensionInstallerCommand(ctx, command)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runExtensionInstallerCommand error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 3*time.Second {
		t.Fatalf("runExtensionInstallerCommand cancellation took %s", elapsed)
	}
}

func newTestExtensionDAO(t *testing.T, appRoot string) *SQLiteExtensionDAO {
	t.Helper()
	db, err := database.NewDB(filepath.Join(appRoot, "extensions.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		t.Fatalf("Migrate returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSQLiteExtensionDAO(db.GetConn())
}

func TestRemoveExtensionFromStoppedProfilesRollsBackAcrossProfiles(t *testing.T) {
	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	manager := NewManager(cfg, appRoot)
	db, err := database.NewDB(filepath.Join(appRoot, "extensions-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao := NewSQLiteExtensionDAO(db.GetConn())
	manager.ExtensionDAO = dao
	manager.InitData()

	extensionID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	runtimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := dao.Upsert(Extension{
		ExtensionID: extensionID,
		Name:        "Rollback Extension",
		Version:     "1.0.0",
		InstallMode: ExtensionInstallModePersistent,
		Enabled:     true,
	}); err != nil {
		t.Fatal(err)
	}

	profileIDs := []string{"profile-a", "profile-b"}
	for _, profileID := range profileIDs {
		profile := &Profile{ProfileId: profileID, ProfileName: profileID, UserDataDir: profileID, RuntimeState: RuntimeStopped}
		manager.Profiles[profileID] = profile
		userDataDir := manager.ResolveUserDataDir(profile)
		artifactDir := filepath.Join(userDataDir, "Default", "Extensions", runtimeID, "1.0.0")
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(artifactDir, "manifest.json"), []byte(`{"name":"Rollback","version":"1.0.0"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := dao.UpsertProfileExtensionRuntime(ProfileExtensionRuntime{
			ProfileID:          profileID,
			ExtensionID:        extensionID,
			RuntimeExtensionID: runtimeID,
			InstallMode:        ExtensionInstallModePersistent,
			InstalledVersion:   "1.0.0",
			Status:             ExtensionRuntimeStatusInstalled,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := db.GetConn().Exec(`CREATE TRIGGER reject_profile_b_disable BEFORE UPDATE ON browser_profile_extension_runtime WHEN NEW.profile_id = 'profile-b' AND NEW.status = 'disabled' BEGIN SELECT RAISE(ABORT, 'reject profile-b disable'); END;`); err != nil {
		t.Fatal(err)
	}

	if err := manager.RemoveExtensionFromStoppedProfiles(extensionID); err == nil {
		t.Fatal("RemoveExtensionFromStoppedProfiles should fail when the second runtime update is rejected")
	}
	for _, profileID := range profileIDs {
		profile := manager.Profiles[profileID]
		artifactPath := filepath.Join(manager.ResolveUserDataDir(profile), "Default", "Extensions", runtimeID, "1.0.0", "manifest.json")
		if _, err := os.Stat(artifactPath); err != nil {
			t.Fatalf("profile %s plugin artifact was not restored: %v", profileID, err)
		}
		runtimeState, err := dao.GetProfileExtensionRuntime(profileID, extensionID)
		if err != nil {
			t.Fatalf("profile %s runtime state missing after rollback: %v", profileID, err)
		}
		if runtimeState.Status != ExtensionRuntimeStatusInstalled {
			t.Fatalf("profile %s runtime status = %q, want %q", profileID, runtimeState.Status, ExtensionRuntimeStatusInstalled)
		}
	}
}
