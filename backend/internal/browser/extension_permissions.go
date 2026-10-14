package browser

import (
	"os"
	"path/filepath"
)

func disabledExtensionPreferencesPath(dir, id string) string {
	return filepath.Join(dir, "Default", ".ant-extension-state", safeExtensionPathSegment(id)+".json")
}

// Chromium may prune registration while an extension is disabled. Keep only
// this extension's preferences outside Chromium's own directories for retry.
func disableProfileExtensionRuntime(dir, id string) error {
	if NormalizeExtensionID(id) == "" {
		return nil
	}
	path := disabledExtensionPreferencesPath(dir, id)
	saved, err := readProfileJSON(path, true)
	if err != nil {
		return err
	}
	for _, name := range []string{"Preferences", "Secure Preferences"} {
		root, err := readProfileJSON(filepath.Join(dir, "Default", name), false)
		if err != nil {
			return err
		}
		extensions, _ := root["extensions"].(map[string]any)
		settings, _ := extensions["settings"].(map[string]any)
		if entry, ok := settings[id].(map[string]any); ok {
			saved[name] = entry
		}
	}
	if err := writeProfileJSON(path, saved); err != nil {
		return err
	}
	return cleanupProfileExtensionRuntime(dir, id)
}

func restoreDisabledExtensionPreferences(dir, oldID, newID string) error {
	saved, err := readProfileJSON(disabledExtensionPreferencesPath(dir, oldID), false)
	if err != nil || saved == nil {
		return err
	}
	for _, name := range []string{"Preferences", "Secure Preferences"} {
		entry, ok := saved[name].(map[string]any)
		if !ok {
			continue
		}
		path := filepath.Join(dir, "Default", name)
		root, err := readProfileJSON(path, true)
		if err != nil {
			return err
		}
		extensions, err := ensureProfileJSONMap(root, "extensions")
		if err != nil {
			return err
		}
		settings, err := ensureProfileJSONMap(extensions, "settings")
		if err != nil {
			return err
		}
		current, err := ensureProfileJSONMap(settings, newID)
		if err != nil {
			return err
		}
		for key, value := range entry {
			if _, exists := current[key]; !exists {
				current[key] = value
			}
		}
		if err := writeProfileJSON(path, root); err != nil {
			return err
		}
	}
	return nil
}

func forgetDisabledExtensionPreferences(dir, id string) {
	_ = os.Remove(disabledExtensionPreferencesPath(dir, id))
}
