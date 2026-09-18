package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"testing"
)

func TestGroupOperationsSynchronizeProfileMemoryAfterCommit(t *testing.T) {
	root := t.TempDir()
	db, err := database.NewDB(root + "/app.db")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	manager := browser.NewManager(config.DefaultConfig(), root)
	manager.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())
	manager.GroupDAO = browser.NewSQLiteGroupDAO(db.GetConn())
	parent, err := manager.GroupDAO.Create(browser.GroupInput{GroupName: "parent"})
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	child, err := manager.GroupDAO.Create(browser.GroupInput{GroupName: "child", ParentId: parent.GroupId})
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}
	profile := &browser.Profile{ProfileId: "profile-1", ProfileName: "profile-1", UserDataDir: "profile-1", GroupId: parent.GroupId}
	if err := manager.ProfileDAO.Upsert(profile); err != nil {
		t.Fatalf("Upsert profile: %v", err)
	}
	manager.InitData()

	app := NewApp(root)
	app.browserMgr = manager
	if err := app.MoveInstancesToGroup([]string{profile.ProfileId}, child.GroupId); err != nil {
		t.Fatalf("MoveInstancesToGroup: %v", err)
	}
	manager.Mutex.Lock()
	inMemory := manager.Profiles[profile.ProfileId]
	if inMemory == nil || inMemory.GroupId != child.GroupId {
		manager.Mutex.Unlock()
		t.Fatalf("memory group after move = %#v, want %q", inMemory, child.GroupId)
	}
	movedUpdatedAt := inMemory.UpdatedAt
	manager.Mutex.Unlock()
	stored, err := manager.ProfileDAO.GetById(profile.ProfileId)
	if err != nil {
		t.Fatalf("Get profile after move: %v", err)
	}
	if stored.GroupId != child.GroupId || stored.UpdatedAt != movedUpdatedAt {
		t.Fatalf("database/memory mismatch after move: db=%#v memory=%#v", stored, inMemory)
	}

	if err := app.DeleteGroup(child.GroupId); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	manager.Mutex.Lock()
	inMemory = manager.Profiles[profile.ProfileId]
	if inMemory == nil || inMemory.GroupId != parent.GroupId {
		manager.Mutex.Unlock()
		t.Fatalf("memory group after delete = %#v, want %q", inMemory, parent.GroupId)
	}
	deletedUpdatedAt := inMemory.UpdatedAt
	manager.Mutex.Unlock()
	stored, err = manager.ProfileDAO.GetById(profile.ProfileId)
	if err != nil {
		t.Fatalf("Get profile after group delete: %v", err)
	}
	if stored.GroupId != parent.GroupId || stored.UpdatedAt != deletedUpdatedAt {
		t.Fatalf("database/memory mismatch after delete: db=%#v memory=%#v", stored, inMemory)
	}
}
