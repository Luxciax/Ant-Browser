package backend

import "strings"

// closeProfilePageSession binds long-lived automation sessions to the browser
// runtime lifecycle. A page session must never survive a profile stop/crash or
// be reused by a later browser generation with otherwise identical settings.
func (a *App) closeProfilePageSession(profileID string) {
	if a == nil || a.automationMgr == nil {
		return
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}
	a.automationMgr.ClosePageSession(profileID)
}
