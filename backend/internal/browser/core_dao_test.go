package browser

import (
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func newCoreDAOTestDB(t *testing.T) (*SQLiteCoreDAO, func(string, ...any) error) {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "cores.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return NewSQLiteCoreDAO(db.GetConn()), func(query string, args ...any) error {
		_, err := db.GetConn().Exec(query, args...)
		return err
	}
}

func TestSQLiteCoreDAOUpsertDefaultIsAtomic(t *testing.T) {
	dao, execSQL := newCoreDAOTestDB(t)
	if err := dao.Upsert(Core{CoreId: "core-a", CoreName: "A", CorePath: "chrome/a", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := execSQL(`CREATE TRIGGER reject_bad_core BEFORE INSERT ON browser_cores WHEN NEW.core_id = 'core-bad' BEGIN SELECT RAISE(ABORT, 'reject bad core'); END;`); err != nil {
		t.Fatal(err)
	}

	if err := dao.Upsert(Core{CoreId: "core-bad", CoreName: "Bad", CorePath: "chrome/bad", IsDefault: true}); err == nil {
		t.Fatal("default Upsert should fail when insert trigger rejects the row")
	}
	cores, err := dao.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 1 || cores[0].CoreId != "core-a" || !cores[0].IsDefault {
		t.Fatalf("existing default core was not preserved after rollback: %#v", cores)
	}
}

func TestSQLiteCoreDAOUpsertFirstCoreBecomesDefault(t *testing.T) {
	dao, _ := newCoreDAOTestDB(t)
	if err := dao.Upsert(Core{CoreId: "core-a", CoreName: "A", CorePath: "chrome/a"}); err != nil {
		t.Fatal(err)
	}
	cores, err := dao.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 1 || !cores[0].IsDefault {
		t.Fatalf("first core should be promoted to default: %#v", cores)
	}
}

func TestSQLiteCoreDAOSetDefaultRejectsMissingCoreWithoutClearingDefault(t *testing.T) {
	dao, _ := newCoreDAOTestDB(t)
	if err := dao.Upsert(Core{CoreId: "core-a", CoreName: "A", CorePath: "chrome/a", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := dao.SetDefault("missing"); err == nil {
		t.Fatal("SetDefault should reject a missing core")
	}
	cores, err := dao.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 1 || !cores[0].IsDefault {
		t.Fatalf("existing default was cleared by failed SetDefault: %#v", cores)
	}
}

func TestSQLiteCoreDAODeleteDefaultPromotesRemainingCore(t *testing.T) {
	dao, _ := newCoreDAOTestDB(t)
	if err := dao.Upsert(Core{CoreId: "core-a", CoreName: "A", CorePath: "chrome/a", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := dao.Upsert(Core{CoreId: "core-b", CoreName: "B", CorePath: "chrome/b"}); err != nil {
		t.Fatal(err)
	}

	if err := dao.Delete("core-a"); err != nil {
		t.Fatal(err)
	}
	cores, err := dao.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 1 || cores[0].CoreId != "core-b" || !cores[0].IsDefault {
		t.Fatalf("remaining core was not promoted to default: %#v", cores)
	}
}

func TestSQLiteCoreDAODeleteMissingCoreDoesNotMutateDefaults(t *testing.T) {
	dao, _ := newCoreDAOTestDB(t)
	if err := dao.Upsert(Core{CoreId: "core-a", CoreName: "A", CorePath: "chrome/a", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := dao.Delete("missing"); err == nil {
		t.Fatal("Delete should reject a missing core")
	}
	cores, err := dao.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 1 || !cores[0].IsDefault {
		t.Fatalf("failed delete mutated defaults: %#v", cores)
	}
}

func TestSQLiteCoreDAOUpsertAllRollsBackEntireBatch(t *testing.T) {
	dao, execSQL := newCoreDAOTestDB(t)
	if err := execSQL(`CREATE TRIGGER reject_core_bad BEFORE INSERT ON browser_cores WHEN NEW.core_id = 'core-bad' BEGIN SELECT RAISE(ABORT, 'reject bad core'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := dao.UpsertAll([]Core{
		{CoreId: "core-good", CoreName: "Good", CorePath: "chrome/good", IsDefault: true},
		{CoreId: "core-bad", CoreName: "Bad", CorePath: "chrome/bad"},
	}); err == nil {
		t.Fatal("UpsertAll should fail when one core is rejected")
	}
	cores, err := dao.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cores) != 0 {
		t.Fatalf("batch rollback left persisted cores: %#v", cores)
	}
}
