package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestSQLiteProfileDAOPersistsMemoryLimitMB(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteProfileDAO(db.GetConn())
	profile := &Profile{
		ProfileId:       "profile-memory-limit",
		ProfileName:     "memory limit",
		MemoryLimitMB:   768,
		FingerprintArgs: []string{},
		LaunchArgs:      []string{},
		Tags:            []string{},
		Keywords:        []string{},
		CreatedAt:       "2026-07-26T00:00:00Z",
		UpdatedAt:       "2026-07-26T00:00:00Z",
	}

	if err := dao.Upsert(profile); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	stored, err := dao.GetById(profile.ProfileId)
	if err != nil {
		t.Fatalf("GetById returned error: %v", err)
	}
	if stored.MemoryLimitMB != profile.MemoryLimitMB {
		t.Fatalf("MemoryLimitMB = %d, want %d", stored.MemoryLimitMB, profile.MemoryLimitMB)
	}

	listed, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	if listed[0].MemoryLimitMB != profile.MemoryLimitMB {
		t.Fatalf("listed MemoryLimitMB = %d, want %d", listed[0].MemoryLimitMB, profile.MemoryLimitMB)
	}
}

func TestSQLiteProfileDAOUpsertAllRollsBackEntireBatch(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "profiles-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	dao := NewSQLiteProfileDAO(db.GetConn())
	if _, err := db.GetConn().Exec(`CREATE TRIGGER reject_profile_bad BEFORE INSERT ON browser_profiles WHEN NEW.profile_id = 'bad' BEGIN SELECT RAISE(ABORT, 'reject bad profile'); END;`); err != nil {
		t.Fatal(err)
	}

	good := &Profile{ProfileId: "good", ProfileName: "Good"}
	bad := &Profile{ProfileId: "bad", ProfileName: "Bad"}
	if err := dao.UpsertAll([]*Profile{good, bad}); err == nil {
		t.Fatal("UpsertAll should fail when one row is rejected")
	}
	profiles, err := dao.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Fatalf("batch rollback left persisted profiles: %#v", profiles)
	}
	if good.CreatedAt != "" || good.UpdatedAt != "" || bad.CreatedAt != "" || bad.UpdatedAt != "" {
		t.Fatalf("failed batch mutated timestamps: good=%+v bad=%+v", good, bad)
	}
}

func TestManagerSaveProfilesDoesNotMutateProfilesWhenBatchFails(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "profiles-manager-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetConn().Exec(`CREATE TRIGGER reject_profile_bad_manager BEFORE INSERT ON browser_profiles WHEN NEW.profile_id = 'bad' BEGIN SELECT RAISE(ABORT, 'reject bad profile'); END;`); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	mgr := NewManager(cfg, t.TempDir())
	mgr.ProfileDAO = NewSQLiteProfileDAO(db.GetConn())
	good := &Profile{ProfileId: "good", ProfileName: "Good", CoreId: " default "}
	bad := &Profile{ProfileId: "bad", ProfileName: "Bad"}
	mgr.Profiles = map[string]*Profile{"good": good, "bad": bad}

	if err := mgr.SaveProfiles(); err == nil {
		t.Fatal("SaveProfiles should fail when one batch row is rejected")
	}
	if good.CoreId != " default " || good.CreatedAt != "" || good.UpdatedAt != "" {
		t.Fatalf("failed batch mutated good profile: %+v", good)
	}
	if bad.CreatedAt != "" || bad.UpdatedAt != "" {
		t.Fatalf("failed batch mutated bad profile: %+v", bad)
	}
	stored, err := mgr.ProfileDAO.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("failed batch persisted rows: %#v", stored)
	}
}
