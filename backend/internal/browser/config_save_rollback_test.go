package browser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ant-chrome/backend/internal/config"
)

func newBrowserConfigSaveFailureManager(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	blockedRoot := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewManager(config.DefaultConfig(), blockedRoot)
}

func TestMigrateConfigRollsBackMemoryWhenSaveFails(t *testing.T) {
	m := newBrowserConfigSaveFailureManager(t)
	m.Config.Browser.Environments = []config.BrowserEnvironment{{
		CoreId: "legacy-core", CoreName: "Legacy", CorePath: "chrome/legacy", IsDefault: true,
	}}
	previous := m.Config.Browser

	if m.MigrateConfig() {
		t.Fatal("MigrateConfig should report failure when config save fails")
	}
	if !reflect.DeepEqual(m.Config.Browser, previous) {
		t.Fatalf("browser config was not rolled back after migration save failure")
	}
}

func TestSaveProfilesRollsBackConfigSliceWhenSaveFails(t *testing.T) {
	m := newBrowserConfigSaveFailureManager(t)
	previous := []ProfileConfig{{ProfileId: "old", ProfileName: "Old"}}
	m.Config.Browser.Profiles = append([]ProfileConfig(nil), previous...)
	m.Profiles = map[string]*Profile{
		"new": {ProfileId: "new", ProfileName: "New"},
	}

	if err := m.SaveProfiles(); err == nil {
		t.Fatal("SaveProfiles should fail when config save fails")
	}
	if !reflect.DeepEqual(m.Config.Browser.Profiles, previous) {
		t.Fatalf("config profile slice was not rolled back: %#v", m.Config.Browser.Profiles)
	}
}

func TestCreateRollsBackProfileMapWhenSaveFails(t *testing.T) {
	m := newBrowserConfigSaveFailureManager(t)
	if _, err := m.Create(ProfileInput{ProfileName: "New", UserDataDir: "new-profile"}); err == nil {
		t.Fatal("Create should fail when config save fails")
	}
	if len(m.Profiles) != 0 {
		t.Fatalf("failed Create left profile in memory: %#v", m.Profiles)
	}
}

func TestUpdateRollsBackProfileWhenSaveFails(t *testing.T) {
	m := newBrowserConfigSaveFailureManager(t)
	profile := &Profile{
		ProfileId:       "profile-update-rollback",
		ProfileName:     "Old",
		UserDataDir:     "profile-update-rollback",
		FingerprintArgs: []string{"--old-fingerprint"},
		LaunchArgs:      []string{"--old-launch"},
		Tags:            []string{"old-tag"},
		Keywords:        []string{"old-keyword"},
		UpdatedAt:       "2026-09-17T00:00:00Z",
		RuntimeState:    RuntimeStopped,
	}
	m.Profiles[profile.ProfileId] = profile
	previous := cloneProfileValue(profile)

	_, err := m.Update(profile.ProfileId, ProfileInput{
		ProfileName:     "New",
		UserDataDir:     "new-profile-dir",
		FingerprintArgs: []string{"--new-fingerprint"},
		LaunchArgs:      []string{"--new-launch"},
		Tags:            []string{"new-tag"},
		Keywords:        []string{"new-keyword"},
	})
	if err == nil {
		t.Fatal("Update should fail when config save fails")
	}
	if !reflect.DeepEqual(*profile, previous) {
		t.Fatalf("failed Update mutated profile: got=%+v want=%+v", *profile, previous)
	}
}

func TestSetKeywordsRollsBackWhenSaveFails(t *testing.T) {
	m := newBrowserConfigSaveFailureManager(t)
	profile := &Profile{
		ProfileId:    "profile-keyword-rollback",
		ProfileName:  "Keywords",
		Keywords:     []string{"old"},
		UpdatedAt:    "2026-09-17T00:00:00Z",
		RuntimeState: RuntimeStopped,
	}
	m.Profiles[profile.ProfileId] = profile

	if _, err := m.SetKeywords(profile.ProfileId, []string{"new"}); err == nil {
		t.Fatal("SetKeywords should fail when config save fails")
	}
	if !reflect.DeepEqual(profile.Keywords, []string{"old"}) || profile.UpdatedAt != "2026-09-17T00:00:00Z" {
		t.Fatalf("failed SetKeywords mutated profile: %+v", profile)
	}
}

func TestCopyRollsBackNewProfileWhenSaveFails(t *testing.T) {
	m := newBrowserConfigSaveFailureManager(t)
	source := &Profile{
		ProfileId:       "profile-copy-source",
		ProfileName:     "Source",
		UserDataDir:     "profile-copy-source",
		FingerprintArgs: []string{"--fingerprint-seed=source"},
		RuntimeState:    RuntimeStopped,
	}
	m.Profiles[source.ProfileId] = source

	if _, err := m.Copy(source.ProfileId, "Copy"); err == nil {
		t.Fatal("Copy should fail when config save fails")
	}
	if len(m.Profiles) != 1 || m.Profiles[source.ProfileId] != source {
		t.Fatalf("failed Copy left a partial profile in memory: %#v", m.Profiles)
	}
}
