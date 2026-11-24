package backend

import (
	"fmt"
	"strings"
)

func (a *App) hydrateLaunchAuthSecret() error {
	if a == nil || a.config == nil {
		return nil
	}
	auth := &a.config.LaunchServer.Auth
	auth.APIKey = strings.TrimSpace(auth.APIKey)
	auth.APIKeyRef = strings.TrimSpace(auth.APIKeyRef)
	if auth.APIKeyRef != "" {
		if a.secretStore == nil {
			auth.APIKey = ""
			return fmt.Errorf("LaunchServer API key uses SecretRef but SecretStore is unavailable")
		}
		value, err := a.secretStore.Get(auth.APIKeyRef)
		if err != nil {
			auth.APIKey = ""
			return fmt.Errorf("resolve LaunchServer API key SecretRef: %w", err)
		}
		auth.APIKey = strings.TrimSpace(value)
		return nil
	}
	if auth.APIKey == "" {
		return nil
	}
	if a.secretStore == nil {
		return fmt.Errorf("legacy LaunchServer API key could not be migrated because SecretStore is unavailable")
	}
	ref, err := a.secretStore.Set("", "launchserver.apiKey", auth.APIKey)
	if err != nil {
		return fmt.Errorf("migrate LaunchServer API key to SecretRef: %w", err)
	}
	auth.APIKeyRef = ref
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		auth.APIKeyRef = ""
		_ = a.secretStore.Delete(ref)
		return fmt.Errorf("persist LaunchServer API key SecretRef: %w", err)
	}
	return nil
}
