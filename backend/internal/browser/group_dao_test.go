package browser

import (
	"ant-chrome/backend/internal/database"
	"testing"
)

func TestGroupDeleteIsTransactional(t *testing.T) {
	root := t.TempDir()
	db, err := database.NewDB(root + "/groups.db")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	groupDAO := NewSQLiteGroupDAO(db.GetConn())
	profileDAO := NewSQLiteProfileDAO(db.GetConn())
	parent, err := groupDAO.Create(GroupInput{GroupName: "parent"})
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	child, err := groupDAO.Create(GroupInput{GroupName: "child", ParentId: parent.GroupId})
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}
	grandchild, err := groupDAO.Create(GroupInput{GroupName: "grandchild", ParentId: child.GroupId})
	if err != nil {
		t.Fatalf("Create grandchild: %v", err)
	}
	profile := &Profile{ProfileId: "profile-1", ProfileName: "profile-1", UserDataDir: "profile-1", GroupId: child.GroupId}
	if err := profileDAO.Upsert(profile); err != nil {
		t.Fatalf("Upsert profile: %v", err)
	}

	if _, err := db.GetConn().Exec(`
		CREATE TRIGGER fail_test_group_delete
		BEFORE DELETE ON browser_groups
		WHEN OLD.group_id = '` + child.GroupId + `'
		BEGIN
			SELECT RAISE(ABORT, 'blocked by test');
		END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	if _, err := groupDAO.Delete(child.GroupId); err == nil {
		t.Fatal("Delete succeeded despite failing delete trigger")
	}

	if _, err := groupDAO.GetById(child.GroupId); err != nil {
		t.Fatalf("child group was partially deleted: %v", err)
	}
	storedGrandchild, err := groupDAO.GetById(grandchild.GroupId)
	if err != nil {
		t.Fatalf("Get grandchild: %v", err)
	}
	if storedGrandchild.ParentId != child.GroupId {
		t.Fatalf("grandchild parent = %q, want rollback to %q", storedGrandchild.ParentId, child.GroupId)
	}
	storedProfile, err := profileDAO.GetById(profile.ProfileId)
	if err != nil {
		t.Fatalf("Get profile: %v", err)
	}
	if storedProfile.GroupId != child.GroupId {
		t.Fatalf("profile group = %q, want rollback to %q", storedProfile.GroupId, child.GroupId)
	}
}

func TestGroupDeleteMovesChildrenAndProfilesAtomically(t *testing.T) {
	root := t.TempDir()
	db, err := database.NewDB(root + "/groups.db")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	groupDAO := NewSQLiteGroupDAO(db.GetConn())
	profileDAO := NewSQLiteProfileDAO(db.GetConn())
	parent, _ := groupDAO.Create(GroupInput{GroupName: "parent"})
	child, _ := groupDAO.Create(GroupInput{GroupName: "child", ParentId: parent.GroupId})
	grandchild, _ := groupDAO.Create(GroupInput{GroupName: "grandchild", ParentId: child.GroupId})
	profile := &Profile{ProfileId: "profile-1", ProfileName: "profile-1", UserDataDir: "profile-1", GroupId: child.GroupId}
	if err := profileDAO.Upsert(profile); err != nil {
		t.Fatalf("Upsert profile: %v", err)
	}

	result, err := groupDAO.Delete(child.GroupId)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if result == nil || result.ParentID != parent.GroupId || result.ProfilesUpdatedAt == "" {
		t.Fatalf("Delete result = %#v", result)
	}
	storedGrandchild, err := groupDAO.GetById(grandchild.GroupId)
	if err != nil {
		t.Fatalf("Get grandchild: %v", err)
	}
	if storedGrandchild.ParentId != parent.GroupId {
		t.Fatalf("grandchild parent = %q, want %q", storedGrandchild.ParentId, parent.GroupId)
	}
	storedProfile, err := profileDAO.GetById(profile.ProfileId)
	if err != nil {
		t.Fatalf("Get profile: %v", err)
	}
	if storedProfile.GroupId != parent.GroupId || storedProfile.UpdatedAt != result.ProfilesUpdatedAt {
		t.Fatalf("profile after delete = group %q updated %q, want group %q updated %q", storedProfile.GroupId, storedProfile.UpdatedAt, parent.GroupId, result.ProfilesUpdatedAt)
	}
}
