package launchcode

import (
	"errors"
	"testing"
)

type failingLaunchCodeDAO struct {
	base       *MemoryLaunchCodeDAO
	failUpsert bool
	failDelete bool
}

func (d *failingLaunchCodeDAO) FindProfileId(code string) (string, error) {
	return d.base.FindProfileId(code)
}

func (d *failingLaunchCodeDAO) FindCode(profileID string) (string, error) {
	return d.base.FindCode(profileID)
}

func (d *failingLaunchCodeDAO) Upsert(profileID, code string) error {
	if d.failUpsert {
		return errors.New("forced launch code upsert failure")
	}
	return d.base.Upsert(profileID, code)
}

func (d *failingLaunchCodeDAO) Delete(profileID string) error {
	if d.failDelete {
		return errors.New("forced launch code delete failure")
	}
	return d.base.Delete(profileID)
}

func (d *failingLaunchCodeDAO) LoadAll() (map[string]string, error) {
	return d.base.LoadAll()
}

func TestLaunchCodeRemoveKeepsCacheWhenDAODeleteFails(t *testing.T) {
	dao := &failingLaunchCodeDAO{base: NewMemoryLaunchCodeDAO()}
	service := NewLaunchCodeService(dao)
	if _, err := service.SetCode("profile-a", "OLD1"); err != nil {
		t.Fatal(err)
	}
	dao.failDelete = true
	if err := service.Remove("profile-a"); err == nil {
		t.Fatal("Remove should fail when DAO delete fails")
	}
	if profileID, err := service.Resolve("OLD1"); err != nil || profileID != "profile-a" {
		t.Fatalf("cached launch code was lost after failed delete: profile=%q err=%v", profileID, err)
	}
	if code, err := dao.base.FindCode("profile-a"); err != nil || code != "OLD1" {
		t.Fatalf("persisted launch code changed after failed delete: code=%q err=%v", code, err)
	}
}

func TestLaunchCodeRegenerateKeepsOldMappingWhenDAOUpsertFails(t *testing.T) {
	dao := &failingLaunchCodeDAO{base: NewMemoryLaunchCodeDAO()}
	service := NewLaunchCodeService(dao)
	if _, err := service.SetCode("profile-a", "OLD1"); err != nil {
		t.Fatal(err)
	}
	dao.failUpsert = true
	if _, err := service.RegenerateCode("profile-a"); err == nil {
		t.Fatal("RegenerateCode should fail when DAO upsert fails")
	}
	if profileID, err := service.Resolve("OLD1"); err != nil || profileID != "profile-a" {
		t.Fatalf("cached old launch code was lost after failed regenerate: profile=%q err=%v", profileID, err)
	}
	if code, err := dao.base.FindCode("profile-a"); err != nil || code != "OLD1" {
		t.Fatalf("persisted old launch code changed after failed regenerate: code=%q err=%v", code, err)
	}
}
