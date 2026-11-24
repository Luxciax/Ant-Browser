package backend

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"ant-chrome/backend/internal/launchcode"
)

func (a *App) BrowserLaunchAuthRotate(scopes []string) (map[string]interface{}, error) {
	if a == nil || a.config == nil || a.launchServer == nil {
		return nil, fmt.Errorf("launch server is unavailable")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil { return nil, err }
	key := base64.RawURLEncoding.EncodeToString(buf)
	if len(scopes) == 0 { scopes = []string{launchcode.ScopeAll} }
	previous := a.config.LaunchServer.Auth
	a.config.LaunchServer.Auth.Enabled = true
	a.config.LaunchServer.Auth.APIKey = key
	a.config.LaunchServer.Auth.Scopes = append([]string{}, scopes...)
	if strings.TrimSpace(a.config.LaunchServer.Auth.Header) == "" {
		a.config.LaunchServer.Auth.Header = launchcode.DefaultAPIKeyHeader
	}
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.LaunchServer.Auth = previous
		return nil, err
	}
	a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
		Enabled: true, APIKey: key, Header: a.config.LaunchServer.Auth.Header, Scopes: scopes,
	})
	return map[string]interface{}{"apiKey": key, "header": a.config.LaunchServer.Auth.Header, "scopes": scopes}, nil
}

func (a *App) BrowserLaunchAuthRevoke() error {
	if a == nil || a.config == nil || a.launchServer == nil {
		return fmt.Errorf("launch server is unavailable")
	}
	previous := a.config.LaunchServer.Auth
	a.config.LaunchServer.Auth.Enabled = false
	a.config.LaunchServer.Auth.APIKey = ""
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.LaunchServer.Auth = previous
		return err
	}
	a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{Enabled:false, Header:a.config.LaunchServer.Auth.Header, Scopes:a.config.LaunchServer.Auth.Scopes})
	return nil
}
