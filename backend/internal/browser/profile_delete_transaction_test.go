package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type profileDeleteCodeProvider struct {
	codes map[string]string
}

func (p *profileDeleteCodeProvider) EnsureCode(profileID string) (string, error) {
	if code := p.codes[profileID]; code != "" {
		return code, nil
	}
	code := "ROLLBACK1"
	p.codes[profileID] = code
	return code, nil
}

func (p *profileDeleteCodeProvider) LookupCode(profileID string) (string, bool) {
	code, ok := p.codes[profileID]
	return code, ok
}

func (p *profileDeleteCodeProvider) SetCode(profileID, code string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("code is empty")
	}
	p.codes[profileID] = code
	return code, nil
}

func (p *profileDeleteCodeProvider) Remove(profileID string) error {
	delete(p.codes, profileID)
	return nil
}

func TestPermanentlyDeleteRestoresLaunchCodeAndExtensionStateWhenProfileDeleteFails(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	manager := NewManager(cfg, root)
	profile := &Profile{
		ProfileId:   "profile-delete-relations",
		ProfileName: "relations",
		UserDataDir: "profile-delete-relations",
		DeletedAt:   time.Now().Add(-4 * 24 * time.Hour).Format(time.RFC3339),
	}
	manager.ProfileDAO = &profileDeleteAuditTestDAO{
		profiles:   map[string]*Profile{profile.ProfileId: profile},
		failDelete: true,
	}
	codeProvider := &profileDeleteCodeProvider{codes: map[string]string{profile.ProfileId: "KEEP1"}}
	manager.CodeProvider = codeProvider

	db, err := database.NewDB(filepath.Join(root, "delete-relations.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	extensionDAO := NewSQLiteExtensionDAO(db.GetConn())
	manager.ExtensionDAO = extensionDAO
	extensionID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := extensionDAO.SetProfileSettings(profile.ProfileId, []string{extensionID}, true); err != nil {
		t.Fatal(err)
	}
	if err := extensionDAO.UpsertProfileExtensionRuntime(ProfileExtensionRuntime{
		ProfileID:          profile.ProfileId,
		ExtensionID:        extensionID,
		RuntimeExtensionID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		InstallMode:        ExtensionInstallModePersistent,
		InstalledVersion:   "1.0.0",
		Status:             ExtensionRuntimeStatusInstalled,
	}); err != nil {
		t.Fatal(err)
	}

	if err := manager.PermanentlyDelete(profile.ProfileId); err == nil {
		t.Fatal("PermanentlyDelete should fail when ProfileDAO.Delete fails")
	}
	if codeProvider.codes[profile.ProfileId] != "KEEP1" {
		t.Fatalf("launch code was not restored: %q", codeProvider.codes[profile.ProfileId])
	}
	settings, err := extensionDAO.GetProfileSettings(profile.ProfileId)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Configured || len(settings.ExtensionIDs) != 1 || settings.ExtensionIDs[0] != extensionID {
		t.Fatalf("profile extension settings were not restored: %#v", settings)
	}
	runtimeState, err := extensionDAO.GetProfileExtensionRuntime(profile.ProfileId, extensionID)
	if err != nil {
		t.Fatal(err)
	}
	if runtimeState.Status != ExtensionRuntimeStatusInstalled {
		t.Fatalf("profile extension runtime was not restored: %#v", runtimeState)
	}
}

func TestDeleteConfigFallbackRestoresMemoryWhenSaveFails(t *testing.T) {
	root := t.TempDir()
	blockedRoot := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(config.DefaultConfig(), blockedRoot)
	profile := &Profile{
		ProfileId:    "profile-delete-fallback",
		ProfileName:  "fallback",
		UserDataDir:  "profile-delete-fallback",
		CreatedAt:    "2026-09-17T00:00:00Z",
		UpdatedAt:    "2026-09-17T00:00:00Z",
		RuntimeState: RuntimeStopped,
	}
	manager.Profiles[profile.ProfileId] = profile

	if err := manager.Delete(profile.ProfileId); err == nil {
		t.Fatal("Delete should fail when config persistence fails")
	}
	restored, ok := manager.Profiles[profile.ProfileId]
	if !ok || restored != profile {
		t.Fatalf("profile was not restored to memory after failed save: %#v", manager.Profiles)
	}
	if profile.DeletedAt != "" || profile.UpdatedAt != "2026-09-17T00:00:00Z" {
		t.Fatalf("profile timestamps were not rolled back: %+v", profile)
	}
}

func TestPermanentlyDeleteDoesNotCreateLaunchCodeWhenNoneExisted(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(config.DefaultConfig(), root)
	profile := &Profile{
		ProfileId:   "profile-delete-no-code",
		ProfileName: "no-code",
		UserDataDir: "profile-delete-no-code",
		DeletedAt:   time.Now().Add(-4 * 24 * time.Hour).Format(time.RFC3339),
	}
	manager.ProfileDAO = &profileDeleteAuditTestDAO{
		profiles:   map[string]*Profile{profile.ProfileId: profile},
		failDelete: true,
	}
	codeProvider := &profileDeleteCodeProvider{codes: map[string]string{}}
	manager.CodeProvider = codeProvider

	if err := manager.PermanentlyDelete(profile.ProfileId); err == nil {
		t.Fatal("PermanentlyDelete should fail when ProfileDAO.Delete fails")
	}
	if code, exists := codeProvider.codes[profile.ProfileId]; exists {
		t.Fatalf("failed permanent delete created an unexpected launch code: %q", code)
	}
}
