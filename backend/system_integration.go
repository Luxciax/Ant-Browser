package backend

import (
	"net/url"
	"os"
	"strings"
)

type BrowserSystemIntegrationStatus struct {
	SchemaVersion int      `json:"schemaVersion"`
	Supported     bool     `json:"supported"`
	Protocol      string   `json:"protocol"`
	Registered    bool     `json:"registered"`
	Executable    string   `json:"executable,omitempty"`
	Actions       []string `json:"actions"`
	Message       string   `json:"message,omitempty"`
}

func (a *App) BrowserSystemIntegrationStatusGet() interface{} {
	return browserSystemIntegrationStatus()
}
func (a *App) BrowserSystemIntegrationSetProtocol(enabled bool) (interface{}, error) {
	if err := setBrowserURLProtocolRegistration(enabled); err != nil {
		return browserSystemIntegrationStatus(), err
	}
	return browserSystemIntegrationStatus(), nil
}

func (a *App) BrowserSystemIntegrationStartupURLs() []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, arg := range os.Args[1:] {
		arg = strings.TrimSpace(arg)
		parsed, err := url.Parse(arg)
		if err != nil || !strings.EqualFold(parsed.Scheme, "antbrowser") { continue }
		if _, exists := seen[arg]; exists { continue }
		seen[arg] = struct{}{}
		out = append(out, arg)
	}
	return out
}
