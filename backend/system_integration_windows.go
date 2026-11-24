//go:build windows

package backend

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const antBrowserProtocol = "antbrowser"

func browserProtocolCommand(executable string) string {
	return fmt.Sprintf("%q %q", strings.TrimSpace(executable), "%1")
}

func browserSystemIntegrationStatus() BrowserSystemIntegrationStatus {
	out := BrowserSystemIntegrationStatus{
		SchemaVersion: 1, Supported: true, Protocol: antBrowserProtocol,
		Actions: []string{"register-url-protocol", "unregister-url-protocol"},
	}
	executable, err := os.Executable()
	if err == nil { out.Executable = executable }
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\`+antBrowserProtocol+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		out.Message = "URL protocol is not registered"
		return out
	}
	defer key.Close()
	command, _, err := key.GetStringValue("")
	if err == nil && strings.TrimSpace(command) == strings.TrimSpace(browserProtocolCommand(executable)) {
		out.Registered = true
		out.Message = "URL protocol is registered for the current user"
	} else {
		out.Message = "URL protocol registration exists but targets a different command"
	}
	return out
}

func setBrowserURLProtocolRegistration(enabled bool) error {
	executable, err := os.Executable()
	if err != nil { return fmt.Errorf("resolve executable: %w", err) }
	rootPath := `Software\Classes\` + antBrowserProtocol
	if !enabled {
		for _, path := range []string{
			rootPath+`\shell\open\command`, rootPath+`\shell\open`, rootPath+`\shell`, rootPath,
		} {
			if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && err != registry.ErrNotExist {
				return fmt.Errorf("remove protocol registration: %w", err)
			}
		}
		return nil
	}
	root, _, err := registry.CreateKey(registry.CURRENT_USER, rootPath, registry.SET_VALUE)
	if err != nil { return fmt.Errorf("create protocol registration: %w", err) }
	if err := root.SetStringValue("", "URL:Ant Browser Protocol"); err != nil { root.Close(); return err }
	if err := root.SetStringValue("URL Protocol", ""); err != nil { root.Close(); return err }
	root.Close()
	commandKey, _, err := registry.CreateKey(registry.CURRENT_USER, rootPath+`\shell\open\command`, registry.SET_VALUE)
	if err != nil { return fmt.Errorf("create protocol command: %w", err) }
	defer commandKey.Close()
	if err := commandKey.SetStringValue("", browserProtocolCommand(executable)); err != nil {
		return fmt.Errorf("set protocol command: %w", err)
	}
	return nil
}
