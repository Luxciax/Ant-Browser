package browser

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditReviewDisableEnablePreservesScriptPermission(t *testing.T) {
	m, p, ext, dir := auditExtensionFixture(t)
	m.Config.Browser.UserDataRoot = filepath.Dir(dir)
	p.UserDataDir = filepath.Base(dir)
	m.InitData()
	m.Profiles[p.ProfileId] = p
	if _, err := m.ensurePersistentExtensionInstalled(p, dir, "unused", ext, nil); err != nil {
		t.Fatal(err)
	}
	pref := filepath.Join(dir, "Default", "Preferences")
	if err := writeProfileJSON(pref, profileExtensionJSON{"extensions": profileExtensionJSON{"settings": profileExtensionJSON{ext.ExtensionID: profileExtensionJSON{"user_scripts_enabled": true}}}}); err != nil {
		t.Fatal(err)
	}
	if err := m.ExtensionDAO.SetEnabled(ext.ExtensionID, false); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveExtensionFromStoppedProfiles(ext.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if err := m.ExtensionDAO.SetEnabled(ext.ExtensionID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ensurePersistentExtensionInstalled(p, dir, "unused", ext, nil); err != nil {
		t.Fatal(err)
	}
	root, err := readProfileJSON(pref, false)
	if err != nil {
		t.Fatal(err)
	}
	settings := root["extensions"].(map[string]any)["settings"].(map[string]any)
	record, ok := settings[ext.ExtensionID].(map[string]any)
	if !ok || record["user_scripts_enabled"] != true {
		t.Fatalf("disable/enable erased existing permission: %#v", settings)
	}
}

func TestAuditReviewUpgradePreservesOptionalGrant(t *testing.T) {
	m, p, ext, dir := auditExtensionFixture(t)
	if err := os.WriteFile(ext.PackagePath, reviewOptionalCRX(t, "2.0.0"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ensurePersistentExtensionInstalled(p, dir, "unused", ext, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Default", "Secure Preferences")
	root, err := readProfileJSON(path, false)
	if err != nil {
		t.Fatal(err)
	}
	settings := root["extensions"].(map[string]any)["settings"].(map[string]any)
	old := settings[ext.ExtensionID].(map[string]any)
	old["active_permissions"] = profileExtensionJSON{"api": []any{"tabs"}, "explicit_host": []any{"https://example.org/*"}}
	old["granted_permissions"] = old["active_permissions"]
	if err := writeProfileJSON(path, root); err != nil {
		t.Fatal(err)
	}
	data := reviewOptionalCRX(t, "3.0.0")
	if err := os.WriteFile(ext.PackagePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	ext.Version = "3.0.0"
	if _, err := m.ensurePersistentExtensionInstalled(p, dir, "unused", ext, nil); err != nil {
		t.Fatal(err)
	}
	root, err = readProfileJSON(path, false)
	if err != nil {
		t.Fatal(err)
	}
	next := root["extensions"].(map[string]any)["settings"].(map[string]any)[ext.ExtensionID].(map[string]any)
	permission := next["granted_permissions"].(map[string]any)
	hosts, _ := permission["explicit_host"].([]any)
	if len(hosts) != 1 || hosts[0] != "https://example.org/*" {
		t.Fatalf("upgrade discarded existing optional host grant: %#v", permission)
	}
}

func reviewOptionalCRX(t *testing.T, version string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	manifest, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(manifest, `{"manifest_version":3,"name":"Optional grant regression","version":%q,"permissions":["storage"],"optional_permissions":["tabs"],"optional_host_permissions":["https://example.org/*"]}`, version)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	identity := []byte("audit-extension-key")
	signature := []byte("test")
	data := make([]byte, 16+len(identity)+len(signature))
	copy(data, []byte("Cr24"))
	binary.LittleEndian.PutUint32(data[4:8], 2)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(identity)))
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(signature)))
	copy(data[16:], identity)
	copy(data[16+len(identity):], signature)
	return append(data, buffer.Bytes()...)
}

func TestDisabledPermissionSurvivesEnableFailureAndRetry(t *testing.T) {
	m, p, ext, dir := auditExtensionFixture(t)
	m.Config.Browser.UserDataRoot = filepath.Dir(dir)
	p.UserDataDir = filepath.Base(dir)
	m.InitData()
	m.Profiles[p.ProfileId] = p
	if _, err := m.ensurePersistentExtensionInstalled(p, dir, "unused", ext, nil); err != nil {
		t.Fatal(err)
	}
	pref := filepath.Join(dir, "Default", "Preferences")
	if err := writeProfileJSON(pref, profileExtensionJSON{"extensions": profileExtensionJSON{"settings": profileExtensionJSON{ext.ExtensionID: profileExtensionJSON{"user_scripts_enabled": true}}}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := m.RemoveExtensionFromStoppedProfiles(ext.ExtensionID); err != nil {
			t.Fatal(err)
		}
	}
	dao := m.ExtensionDAO
	m.ExtensionDAO = auditFailRuntimeDAO{dao}
	if _, err := m.ensurePersistentExtensionInstalled(p, dir, "unused", ext, nil); err == nil {
		t.Fatal("expected injected DB error")
	}
	m.ExtensionDAO = dao
	if _, err := m.ensurePersistentExtensionInstalled(p, dir, "unused", ext, nil); err != nil {
		t.Fatal(err)
	}
	root, err := readProfileJSON(pref, false)
	if err != nil {
		t.Fatal(err)
	}
	entry := root["extensions"].(map[string]any)["settings"].(map[string]any)[ext.ExtensionID].(map[string]any)
	if entry["user_scripts_enabled"] != true {
		t.Fatal("retry lost saved permission")
	}
}

func TestOptionalPermissionsRespectManifestAndUserChoice(t *testing.T) {
	manifest := profileExtensionJSON{"permissions": []any{"storage"}, "optional_permissions": []any{"tabs"}, "optional_host_permissions": []any{"https://*.example.org/*"}}
	old := profileExtensionJSON{"api": []any{"tabs", "debugger"}, "explicit_host": []any{"https://sub.example.org/*", "https://removed.test/*"}}
	got := preserveOptionalExtensionPermissions(manifest, old)
	if len(got["api"].([]any)) != 2 || len(got["explicit_host"].([]any)) != 1 {
		t.Fatalf("wrong permission diff: %#v", got)
	}
	fresh := preserveOptionalExtensionPermissions(manifest, nil)
	if len(fresh["api"].([]any)) != 1 || len(fresh["explicit_host"].([]any)) != 0 {
		t.Fatalf("granted optional permissions without user choice: %#v", fresh)
	}
}
