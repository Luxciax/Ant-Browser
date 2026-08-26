package browser

import (
	"os"
	"path/filepath"
	"testing"
)

const testExtensionID = "abcdefghijklmnopabcdefghijklmnop"

type persistenceTestExtensionDAO struct {
	items []Extension
}

func (d *persistenceTestExtensionDAO) List() ([]Extension, error)                { return d.items, nil }
func (d *persistenceTestExtensionDAO) ListEnabled() ([]Extension, error)         { return d.items, nil }
func (d *persistenceTestExtensionDAO) ListByIDs(_ []string) ([]Extension, error) { return d.items, nil }
func (d *persistenceTestExtensionDAO) Get(_ string) (Extension, error)           { return d.items[0], nil }
func (d *persistenceTestExtensionDAO) Upsert(Extension) error                    { return nil }
func (d *persistenceTestExtensionDAO) SetEnabled(string, bool) error             { return nil }
func (d *persistenceTestExtensionDAO) Delete(string) error                       { return nil }
func (d *persistenceTestExtensionDAO) GetProfileSettings(profileID string) (ProfileExtensionSettings, error) {
	return ProfileExtensionSettings{ProfileID: profileID, Configured: false}, nil
}
func (d *persistenceTestExtensionDAO) SetProfileSettings(profileID string, ids []string, configured bool) (ProfileExtensionSettings, error) {
	return ProfileExtensionSettings{ProfileID: profileID, ExtensionIDs: ids, Configured: configured}, nil
}
func (d *persistenceTestExtensionDAO) DeleteProfileSettings(string) error { return nil }

func TestPrepareExtensionDirsForProfileKeepsStablePackageAndStorage(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "global", testExtensionID)
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "manifest.json"), []byte(`{"name":"test","version":"1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "payload.js"), []byte("version-one"), 0o644); err != nil {
		t.Fatal(err)
	}

	userDataDir := filepath.Join(root, "profile-a")
	storageFile := filepath.Join(userDataDir, "Default", "Local Extension Settings", testExtensionID, "CURRENT")
	if err := os.MkdirAll(filepath.Dir(storageFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storageFile, []byte("user-script-state"), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{ExtensionDAO: &persistenceTestExtensionDAO{items: []Extension{{ExtensionID: testExtensionID, InstallDir: source, Enabled: true}}}}
	dirs, err := manager.PrepareExtensionDirsForProfile("profile-a", userDataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 {
		t.Fatalf("dirs = %v, want one", dirs)
	}
	stableDir := filepath.Join(userDataDir, profileExtensionsDir, testExtensionID)
	if dirs[0] != stableDir {
		t.Fatalf("dir = %s, want %s", dirs[0], stableDir)
	}

	if err := os.WriteFile(filepath.Join(source, "payload.js"), []byte("version-two"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs, err = manager.PrepareExtensionDirsForProfile("profile-a", userDataDir)
	if err != nil {
		t.Fatal(err)
	}
	packageData, err := os.ReadFile(filepath.Join(dirs[0], "payload.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(packageData) != "version-one" {
		t.Fatalf("stable package was overwritten: %q", packageData)
	}
	storageData, err := os.ReadFile(storageFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(storageData) != "user-script-state" {
		t.Fatalf("extension storage changed: %q", storageData)
	}
}

func TestPrepareExtensionDirsForProfileUsesIndependentProfilePaths(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "global", testExtensionID)
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "manifest.json"), []byte(`{"name":"test","version":"1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{ExtensionDAO: &persistenceTestExtensionDAO{items: []Extension{{ExtensionID: testExtensionID, InstallDir: source, Enabled: true}}}}

	a, err := manager.PrepareExtensionDirsForProfile("a", filepath.Join(root, "profile-a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := manager.PrepareExtensionDirsForProfile("b", filepath.Join(root, "profile-b"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || len(b) != 1 || a[0] == b[0] {
		t.Fatalf("profile paths are not isolated: %v %v", a, b)
	}
}
