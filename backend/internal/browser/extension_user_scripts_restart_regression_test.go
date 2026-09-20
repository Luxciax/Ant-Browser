package browser

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchingCommandLineRegistrationDoesNotRewriteUserScriptsPreference(t *testing.T) {
	userDataDir := t.TempDir()
	runtimeID := "ndcooeababalnlpkfedmmbbbgkljhpjf"
	version := "1.4.0"
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestData := []byte(`{"name":"ScriptCat","version":"1.4.0","permissions":["userScripts"]}`)
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	relativePath, err := profileExtensionRelativePath(userDataDir, codePath)
	if err != nil {
		t.Fatal(err)
	}
	securePath := filepath.Join(userDataDir, "Default", "Secure Preferences")
	if err := os.MkdirAll(filepath.Dir(securePath), 0o755); err != nil {
		t.Fatal(err)
	}
	root := map[string]any{
		"extensions": map[string]any{
			"settings": map[string]any{
				runtimeID: map[string]any{
					"location":             8,
					"creation_flags":       38,
					"path":                 relativePath,
					"user_scripts_enabled": true,
				},
			},
		},
		"protection": map[string]any{
			"macs": map[string]any{
				"extensions": map[string]any{
					"settings":                map[string]any{runtimeID: "valid-entry-mac"},
					"settings_encrypted_hash": map[string]any{runtimeID: "valid-entry-hash"},
				},
			},
		},
	}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(securePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(securePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureProfileExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("ensureProfileExtensionRegistration returned error: %v", err)
	}
	after, err := os.ReadFile(securePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("matching location=8 registration was rewritten; this would invalidate Chromium's protected extension prefs")
	}
}

func TestMigrateUserScriptsPreferenceWritesProtectedExtensionSettings(t *testing.T) {
	userDataDir := t.TempDir()
	oldRuntimeID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newRuntimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	securePath := filepath.Join(userDataDir, "Default", "Secure Preferences")
	if err := os.MkdirAll(filepath.Dir(securePath), 0o755); err != nil {
		t.Fatal(err)
	}
	root := map[string]any{
		"extensions": map[string]any{
			"settings": map[string]any{
				oldRuntimeID: map[string]any{"user_scripts_enabled": true},
				newRuntimeID: map[string]any{"location": 8},
			},
		},
		"protection": map[string]any{
			"macs": map[string]any{
				"extensions": map[string]any{
					"settings":                map[string]any{newRuntimeID: "stale-entry-mac"},
					"settings_encrypted_hash": map[string]any{newRuntimeID: "stale-entry-hash"},
				},
			},
		},
	}
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(securePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := migrateProfileExtensionUserScriptsPreference(userDataDir, oldRuntimeID, newRuntimeID); err != nil {
		t.Fatalf("migrateProfileExtensionUserScriptsPreference returned error: %v", err)
	}
	reloaded, err := readProfileJSON(securePath, false)
	if err != nil {
		t.Fatal(err)
	}
	extensions, err := ensureProfileJSONMapIfPresent(reloaded, "extensions")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := ensureProfileJSONMapIfPresent(extensions, "settings")
	if err != nil {
		t.Fatal(err)
	}
	current, ok := settings[newRuntimeID].(map[string]any)
	if !ok {
		t.Fatal("new runtime extension setting missing")
	}
	if enabled, ok := current["user_scripts_enabled"].(bool); !ok || !enabled {
		t.Fatalf("user_scripts_enabled = %#v, want true", current["user_scripts_enabled"])
	}
	protection, _ := ensureProfileJSONMapIfPresent(reloaded, "protection")
	macs, _ := ensureProfileJSONMapIfPresent(protection, "macs")
	protectedExtensions, _ := ensureProfileJSONMapIfPresent(macs, "extensions")
	for _, key := range []string{"settings", "settings_encrypted_hash"} {
		values, _ := ensureProfileJSONMapIfPresent(protectedExtensions, key)
		if _, exists := values[newRuntimeID]; exists {
			t.Fatalf("stale protection value %s.%s was not cleared", key, newRuntimeID)
		}
	}
}
