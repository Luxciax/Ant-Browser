package browser

import (
	"ant-chrome/backend/internal/config"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func auditExtensionFixture(t *testing.T) (*Manager, *Profile, Extension, string) {
	t.Helper()
	root := t.TempDir()
	key := []byte("audit-extension-key")
	data := buildTestCRX2Package(t, key, "2.0.0")
	ext := Extension{ExtensionID: extensionIDFromPublicKey(key), Name: "Audit", Version: "2.0.0", PackagePath: filepath.Join(root, "extension.crx"), InstallDir: filepath.Join(root, "extension"), Enabled: true, DefaultInstall: true}
	if err := os.WriteFile(ext.PackagePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(config.DefaultConfig(), root)
	m.ExtensionDAO = newTestExtensionDAO(t, root)
	if err := m.ExtensionDAO.Upsert(ext); err != nil {
		t.Fatal(err)
	}
	return m, &Profile{ProfileId: "audit"}, ext, filepath.Join(root, "profile")
}

func auditWriteFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAuditInitialRuntimeMigrationPreservesUserScriptsPermission(t *testing.T) {
	m, profile, ext, dir := auditExtensionFixture(t)
	oldID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := writeProfileJSON(filepath.Join(dir, "Default", "Secure Preferences"), profileExtensionJSON{
		"extensions": profileExtensionJSON{"settings": profileExtensionJSON{oldID: profileExtensionJSON{"location": 8, "path": ext.InstallDir}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeProfileJSON(filepath.Join(dir, "Default", "Preferences"), profileExtensionJSON{
		"extensions": profileExtensionJSON{"settings": profileExtensionJSON{oldID: profileExtensionJSON{"user_scripts_enabled": true}}},
	}); err != nil {
		t.Fatal(err)
	}
	auditWriteFile(t, filepath.Join(dir, "Default", "Local Extension Settings", oldID, "data"), "user scripts")
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := m.ensurePersistentExtensionInstalled(profile, dir, "unused", ext, nil); err != nil {
			t.Fatal(err)
		}
		root, err := readProfileJSON(filepath.Join(dir, "Default", "Preferences"), false)
		if err != nil {
			t.Fatal(err)
		}
		settings := root["extensions"].(map[string]any)["settings"].(map[string]any)
		setting, ok := settings[ext.ExtensionID].(map[string]any)
		if !ok || setting["user_scripts_enabled"] != true {
			t.Fatalf("attempt %d lost permission: %#v", attempt, settings)
		}
	}
}

type auditFailRuntimeDAO struct{ ExtensionDAO }

func TestAuditRecoveredRuntimeMigratesLegacyPermissionOnFirstLaunch(t *testing.T) {
	m, profile, ext, dir := auditExtensionFixture(t)
	if _, err := installExtensionPackageIntoProfile(dir, ext.PackagePath, ext); err != nil {
		t.Fatal(err)
	}
	oldID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	securePath := filepath.Join(dir, "Default", "Secure Preferences")
	root, err := readProfileJSON(securePath, false)
	if err != nil {
		t.Fatal(err)
	}
	settings := root["extensions"].(map[string]any)["settings"].(map[string]any)
	settings[oldID] = profileExtensionJSON{"location": 8, "path": ext.InstallDir}
	if err := writeProfileJSON(securePath, root); err != nil {
		t.Fatal(err)
	}
	preferencesPath := filepath.Join(dir, "Default", "Preferences")
	if err := writeProfileJSON(preferencesPath, profileExtensionJSON{"extensions": profileExtensionJSON{"settings": profileExtensionJSON{oldID: profileExtensionJSON{"user_scripts_enabled": true}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ensurePersistentExtensionInstalled(profile, dir, "unused", ext, nil); err != nil {
		t.Fatal(err)
	}
	root, err = readProfileJSON(preferencesPath, false)
	if err != nil {
		t.Fatal(err)
	}
	settings = root["extensions"].(map[string]any)["settings"].(map[string]any)
	setting, ok := settings[ext.ExtensionID].(map[string]any)
	if !ok || setting["user_scripts_enabled"] != true {
		t.Fatalf("index recovery postponed migration until a second launch: %#v", settings)
	}
}

func (d auditFailRuntimeDAO) UpsertProfileExtensionRuntime(ProfileExtensionRuntime) error {
	return errors.New("injected runtime write failure")
}

func TestAuditInstallRollbackPreservesUnindexedTargetStorage(t *testing.T) {
	m, profile, ext, dir := auditExtensionFixture(t)
	oldID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := writeProfileJSON(filepath.Join(dir, "Default", "Secure Preferences"), profileExtensionJSON{
		"extensions": profileExtensionJSON{"settings": profileExtensionJSON{oldID: profileExtensionJSON{"location": 8, "path": ext.InstallDir}}},
	}); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "Default", "Local Extension Settings", oldID, "data")
	newPath := filepath.Join(dir, "Default", "Local Extension Settings", ext.ExtensionID, "data")
	auditWriteFile(t, oldPath, "old scripts")
	auditWriteFile(t, newPath, "new scripts edited after partial migration")
	dao := m.ExtensionDAO
	m.ExtensionDAO = auditFailRuntimeDAO{dao}
	if _, err := m.ensurePersistentExtensionInstalled(profile, dir, "unused", ext, nil); err == nil {
		t.Fatal("expected DB failure")
	}
	for path, want := range map[string]string{oldPath: "old scripts", newPath: "new scripts edited after partial migration"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("rollback lost %s: %q, %v", path, data, err)
		}
	}
	m.ExtensionDAO = dao
	if _, err := m.ensurePersistentExtensionInstalled(profile, dir, "unused", ext, nil); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func TestProfileJSONWriteFailureKeepsDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Preferences")
	// A directory/locked target must remain in place when replacement fails.
	auditWriteFile(t, filepath.Join(path, "sentinel"), "original")
	if err := writeProfileJSON(path, profileExtensionJSON{"new": true}); err == nil {
		t.Fatal("expected replacement failure")
	}
	if data, err := os.ReadFile(filepath.Join(path, "sentinel")); err != nil || string(data) != "original" {
		t.Fatalf("destination changed on failure: %q %v", data, err)
	}
}

func TestProfileJSONAtomicReplacementPreservesUnrelatedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Preferences")
	if err := writeProfileJSON(path, profileExtensionJSON{"browser": profileExtensionJSON{"name": "主人 <user>"}, "revision": 1}); err != nil {
		t.Fatal(err)
	}
	root, err := readProfileJSON(path, false)
	if err != nil {
		t.Fatal(err)
	}
	root["revision"] = 2
	if err := writeProfileJSON(path, root); err != nil {
		t.Fatal(err)
	}
	root, err = readProfileJSON(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if root["revision"] != float64(2) || root["browser"].(map[string]any)["name"] != "主人 <user>" {
		t.Fatalf("replacement changed unrelated data: %#v", root)
	}
}
