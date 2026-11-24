package backend

import (
	"fmt"
	"strings"
)

type BrowserSyncStatusV1 struct {
	SchemaVersion  int                    `json:"schemaVersion"`
	Mode           string                 `json:"mode"`
	Continuous     bool                   `json:"continuous"`
	Bidirectional  bool                   `json:"bidirectional"`
	ConflictPolicy string                 `json:"conflictPolicy"`
	LocalDirectory string                 `json:"localDirectory,omitempty"`
	Schedule       map[string]interface{} `json:"schedule"`
	Limitations    []string               `json:"limitations"`
}

func (a *App) BrowserSyncStatus() BrowserSyncStatusV1 {
	local := a.BackupGetLocalSettings()
	schedule := a.BackupScheduledGetSettings()
	return BrowserSyncStatusV1{
		SchemaVersion: 1,
		Mode: "backup-channel",
		Continuous: false,
		Bidirectional: false,
		ConflictPolicy: "preserve-local-and-report",
		LocalDirectory: strings.TrimSpace(local.LocalDirectory),
		Schedule: schedule,
		Limitations: []string{
			"scheduled backup is not continuous profile synchronization",
			"imports preserve unresolved conflicts instead of overwriting silently",
		},
	}
}

func (a *App) BrowserControlTrashList() (map[string]interface{}, error) {
	if a == nil || a.browserMgr == nil { return nil, fmt.Errorf("browser manager is unavailable") }
	items := a.BrowserProfileTrashList()
	public := make([]interface{}, 0, len(items))
	for i := range items { public = append(public, browserProfileTrashPublicView(&items[i])) }
	return map[string]interface{}{"items": public, "count": len(public), "retentionHours": 72}, nil
}

func browserProfileTrashPublicView(profile *BrowserProfile) interface{} {
	if profile == nil { return nil }
	return map[string]interface{}{
		"profileId": profile.ProfileId,
		"profileName": profile.ProfileName,
		"coreId": profile.CoreId,
		"proxyId": profile.ProxyId,
		"tags": append([]string{}, profile.Tags...),
		"keywords": append([]string{}, profile.Keywords...),
		"groupId": profile.GroupId,
		"deletedAt": profile.DeletedAt,
		"createdAt": profile.CreatedAt,
		"updatedAt": profile.UpdatedAt,
	}
}

func (a *App) BrowserControlTrashRestore(profileID string) (map[string]interface{}, error) {
	profile, err := a.BrowserProfileRestore(profileID)
	if err != nil { return nil, err }
	return map[string]interface{}{"profile": browserProfileTrashPublicView(profile)}, nil
}
func (a *App) BrowserControlTrashPermanentlyDelete(profileID string) error { return a.BrowserProfilePermanentlyDelete(profileID) }
func (a *App) BrowserControlTrashCleanup() error { return a.BrowserProfileTrashCleanup() }
func (a *App) BrowserControlSyncStatus() (map[string]interface{}, error) { return phase2PublicMap(a.BrowserSyncStatus()) }
