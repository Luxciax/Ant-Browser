package browser

import (
	"os"
	"path/filepath"
	"strings"
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

func preserveOptionalExtensionPermissions(manifest, existing profileExtensionJSON) profileExtensionJSON {
	result := extensionPermissionSnapshot(manifest)
	optionalAPI := map[string]bool{}
	var allowedHosts []string
	for _, key := range []string{"optional_permissions", "optional_host_permissions", "host_permissions", "permissions"} {
		items, _ := manifest[key].([]any)
		for _, raw := range items {
			value, ok := raw.(string)
			if !ok {
				continue
			}
			if strings.Contains(value, "://") || value == "<all_urls>" {
				allowedHosts = append(allowedHosts, value)
			} else if key == "optional_permissions" {
				optionalAPI[value] = true
			}
		}
	}
	appendUnique := func(key string, value any) {
		items := result[key].([]any)
		for _, item := range items {
			if item == value {
				return
			}
		}
		result[key] = append(items, value)
	}
	oldAPI, _ := existing["api"].([]any)
	for _, raw := range oldAPI {
		if value, ok := raw.(string); ok && optionalAPI[value] {
			appendUnique("api", value)
		}
	}
	oldHosts, _ := existing["explicit_host"].([]any)
	for _, raw := range oldHosts {
		if value, ok := raw.(string); ok {
			for _, allowed := range allowedHosts {
				if extensionHostGrantAllowed(value, allowed) {
					appendUnique("explicit_host", value)
					break
				}
			}
		}
	}
	return result
}

func extensionHostGrantAllowed(grant, allowed string) bool {
	if grant == allowed || allowed == "<all_urls>" {
		return true
	}
	g := strings.SplitN(grant, "://", 2)
	a := strings.SplitN(allowed, "://", 2)
	if len(g) != 2 || len(a) != 2 {
		return false
	}
	if a[0] != g[0] && !(a[0] == "*" && (g[0] == "http" || g[0] == "https" || g[0] == "*")) {
		return false
	}
	gh := strings.ToLower(strings.SplitN(g[1], "/", 2)[0])
	ah := strings.ToLower(strings.SplitN(a[1], "/", 2)[0])
	if ah == "*" || ah == gh {
		return true
	}
	if strings.HasPrefix(ah, "*.") {
		base := strings.TrimPrefix(ah, "*.")
		return gh == base || strings.HasSuffix(gh, "."+base)
	}
	return false
}
