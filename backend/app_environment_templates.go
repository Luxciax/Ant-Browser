package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/browser"
)

type BrowserEnvironmentTemplate = browser.EnvironmentTemplate

func (a *App) BrowserEnvironmentTemplateList() []BrowserEnvironmentTemplate {
	return browser.BuiltinEnvironmentTemplates()
}

func (a *App) BrowserProfileApplyEnvironmentTemplate(profileID, templateID string) (*BrowserProfile, error) {
	if a == nil || a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	templateID = strings.TrimSpace(templateID)
	if profileID == "" || templateID == "" {
		return nil, fmt.Errorf("profile id and template id are required")
	}
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileID)
	defer unlockRuntimeOp()
	profile, err := a.browserMgr.ApplyEnvironmentTemplate(profileID, templateID)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}
