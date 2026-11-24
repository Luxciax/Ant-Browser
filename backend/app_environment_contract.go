package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/browser"
)

type BrowserEnvironmentContractV2 = browser.EnvironmentContractV2

// BrowserProfileEnvironmentContract exposes the stable v2 environment DTO used
// by new UI and service integrations. Raw proxy strings and raw legacy launch
// arguments are intentionally not included.
func (a *App) BrowserProfileEnvironmentContract(profileID string) (*BrowserEnvironmentContractV2, error) {
	if a == nil || a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, fmt.Errorf("profile id is required")
	}
	return a.browserMgr.EnvironmentContract(profileID)
}
