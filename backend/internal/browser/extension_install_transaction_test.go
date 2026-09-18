package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
)

func TestInstallExtensionDirectoryRollsBackFilesWhenCatalogUpsertFails(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(config.DefaultConfig(), root)
	db, err := database.NewDB(filepath.Join(root, "extension-install-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao := NewSQLiteExtensionDAO(db.GetConn())
	manager.ExtensionDAO = dao

	sourceDir := filepath.Join(root, "source-extension")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest := func(version string) {
		t.Helper()
		data := []byte(`{"manifest_version":3,"name":"Transactional Local","version":"` + version + `"}`)
		if err := os.WriteFile(filepath.Join(sourceDir, "manifest.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeManifest("1.0.0")
	installed, err := manager.InstallExtensionDirectory(sourceDir)
	if err != nil {
		t.Fatalf("initial install failed: %v", err)
	}
	keyPath := manager.localExtensionKeyPath(installed.ExtensionID)
	keyBefore, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	writeManifest("2.0.0")
	if _, err := db.GetConn().Exec(`CREATE TRIGGER reject_extension_update BEFORE UPDATE ON browser_extensions WHEN NEW.extension_id = '` + installed.ExtensionID + `' BEGIN SELECT RAISE(ABORT, 'reject extension update'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.InstallExtensionDirectory(sourceDir); err == nil {
		t.Fatal("second install should fail when catalog update is rejected")
	}

	stored, err := dao.Get(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != "1.0.0" {
		t.Fatalf("catalog version = %q, want 1.0.0", stored.Version)
	}
	managedManifest, err := os.ReadFile(filepath.Join(installed.InstallDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(managedManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatalf("installed manifest version = %q, want 1.0.0", manifest.Version)
	}
	keyAfter, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(keyAfter) != string(keyBefore) {
		t.Fatal("stable local extension key changed after rolled-back update")
	}
}

func TestInstallExtensionPackageRollsBackDirectoryAndNewVersionPackageWhenCatalogUpsertFails(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(config.DefaultConfig(), root)
	db, err := database.NewDB(filepath.Join(root, "extension-package-install-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao := NewSQLiteExtensionDAO(db.GetConn())
	manager.ExtensionDAO = dao

	publicKey := []byte("transactional-extension-public-key")
	v1Package := buildTestCRX2Package(t, publicKey, "1.0.0")
	installed, err := manager.InstallExtensionPackageBytes("", "https://example.test/v1.crx", v1Package)
	if err != nil {
		t.Fatalf("initial package install failed: %v", err)
	}
	oldPackage, err := os.ReadFile(installed.PackagePath)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.GetConn().Exec(`CREATE TRIGGER reject_extension_package_update BEFORE UPDATE ON browser_extensions WHEN NEW.extension_id = '` + installed.ExtensionID + `' BEGIN SELECT RAISE(ABORT, 'reject extension package update'); END;`); err != nil {
		t.Fatal(err)
	}
	v2Package := buildTestCRX2Package(t, publicKey, "2.0.0")
	if _, err := manager.InstallExtensionPackageBytes("", "https://example.test/v2.crx", v2Package); err == nil {
		t.Fatal("second package install should fail when catalog update is rejected")
	}

	stored, err := dao.Get(installed.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != "1.0.0" {
		t.Fatalf("catalog version = %q, want 1.0.0", stored.Version)
	}
	managedManifest, err := os.ReadFile(filepath.Join(installed.InstallDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(managedManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatalf("installed manifest version = %q, want 1.0.0", manifest.Version)
	}
	oldPackageAfter, err := os.ReadFile(installed.PackagePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(oldPackageAfter) != string(oldPackage) {
		t.Fatal("existing v1 package changed after rolled-back v2 install")
	}
	newPackagePath := manager.extensionPackagePath(installed.ExtensionID, "2.0.0")
	if _, err := os.Stat(newPackagePath); !os.IsNotExist(err) {
		t.Fatalf("rolled-back v2 package still exists: %s (err=%v)", newPackagePath, err)
	}
}
