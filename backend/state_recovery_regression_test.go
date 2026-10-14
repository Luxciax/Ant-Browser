package backend

import (
	"ant-chrome/backend/internal/browser"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAuditReviewStaleDebugPortIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"Browser":"Chrome/140.0","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/OTHER-PROFILE"}`)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(fmt.Sprintf("%d\n/devtools/browser/ORIGINAL-PROFILE\n", port)), 0600); err != nil {
		t.Fatal(err)
	}
	if got, ok := detectBrowserRuntimeByActivePort(dir); ok {
		t.Fatalf("accepted another browser as this profile: port=%d pid=%d; websocket identity differs", got.DebugPort, got.PID)
	}
}

func TestBrowserIdentityAcceptsOnlyMatchingEndpoint(t *testing.T) {
	var endpoint string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"Browser":"Chrome/140","webSocketDebuggerUrl":%q}`, endpoint)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte(u.Port()+"\n/devtools/browser/matching\n"), 0600); err != nil {
		t.Fatal(err)
	}
	endpoint = "ws://" + u.Host + "/devtools/browser/matching"
	if _, ok := detectBrowserRuntimeByActivePort(dir); !ok {
		t.Fatal("rejected matching browser identity")
	}
	for _, wrong := range []string{"ws://" + u.Host + "/devtools/browser/other", "ws://example.org:" + u.Port() + "/devtools/browser/matching", ""} {
		endpoint = wrong
		if _, ok := detectBrowserRuntimeByActivePort(dir); ok {
			t.Fatalf("accepted wrong endpoint %q", wrong)
		}
	}
}

func TestBackupImportsNewSQLiteAsCompleteSnapshot(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(src, "Cookies"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE state(value TEXT); INSERT INTO state VALUES ('from-wal')`); err != nil {
		t.Fatal(err)
	}
	if err := backupSyncDir(src, dst, &backupMergeStats{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "Cookies-wal")); !os.IsNotExist(err) {
		t.Fatalf("published source WAL: %v", err)
	}
	copyDB, err := sql.Open("sqlite", filepath.Join(dst, "Cookies"))
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var value string
	if err := copyDB.QueryRow(`SELECT value FROM state`).Scan(&value); err != nil || value != "from-wal" {
		t.Fatalf("snapshot lost WAL: %q %v", value, err)
	}
}

func TestBackupLevelDBNeverMergesExistingStore(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			src := t.TempDir()
			dst := t.TempDir()
			store := filepath.Join(src, "IndexedDB", "extension.leveldb")
			if err := os.MkdirAll(store, 0755); err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]string{"CURRENT": "MANIFEST-000002\n", "MANIFEST-000002": "source manifest", "000002.ldb": "source records"} {
				if err := os.WriteFile(filepath.Join(store, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(dst, "IndexedDB", "extension.leveldb")
			if existing {
				if err := os.MkdirAll(target, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "CURRENT"), []byte("MANIFEST-000001\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			stats := &backupMergeStats{}
			if err := backupSyncDir(src, dst, stats, nil); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(target, "CURRENT"))
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				if string(data) != "MANIFEST-000001\n" || stats.Conflicts != 1 {
					t.Fatalf("conflicting store changed: %s %+v", data, stats)
				}
				if _, err := os.Stat(filepath.Join(target, "000002.ldb")); !os.IsNotExist(err) {
					t.Fatal("mixed source records into old store")
				}
			} else if string(data) != "MANIFEST-000002\n" {
				t.Fatal("new store not imported")
			}
		})
	}
}

func TestSnapshotRestoreUsesDurableTransaction(t *testing.T) {
	a := newProfileTransactionTestApp(t)
	p := &browser.Profile{ProfileId: "snapshot-transaction", ProfileName: "snapshot", UserDataDir: "snapshot-transaction"}
	if err := a.browserMgr.ProfileDAO.Upsert(p); err != nil {
		t.Fatal(err)
	}
	a.browserMgr.InitData()
	dir := a.browserMgr.ResolveUserDataDir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "marker")
	if err := os.WriteFile(marker, []byte("snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := a.BrowserSnapshotCreate(p.ProfileId, "saved")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("current"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.GetConn().Exec(`CREATE TRIGGER reject_snapshot_commit BEFORE UPDATE OF committed ON profile_file_transactions BEGIN SELECT RAISE(ABORT,'injected snapshot failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := a.BrowserSnapshotRestore(p.ProfileId, info.SnapshotId); err == nil {
		t.Fatal("restore ignored journal commit failure")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "current" {
		t.Fatalf("failed restore changed data: %q %v", data, err)
	}
	if _, err := a.db.GetConn().Exec(`DROP TRIGGER reject_snapshot_commit`); err != nil {
		t.Fatal(err)
	}
	if err := a.BrowserSnapshotRestore(p.ProfileId, info.SnapshotId); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "snapshot" {
		t.Fatalf("restore retry: %q %v", data, err)
	}
}

func TestBackupPreservesOrdinaryJournalNamedFile(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "notes-journal"), []byte("ordinary user text"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := backupSyncDir(src, dst, &backupMergeStats{}, nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "notes-journal")); err != nil || string(data) != "ordinary user text" {
		t.Fatalf("ordinary file filtered as SQLite: %q %v", data, err)
	}
}

func TestBackupRejectsBrokenLevelDBBeforeChangingTarget(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "CURRENT"), []byte("MANIFEST-000001\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "000001.ldb"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := backupSyncDir(src, dst, &backupMergeStats{}, nil); err == nil {
		t.Fatal("accepted incomplete LevelDB")
	}
	entries, err := os.ReadDir(dst)
	if err != nil || len(entries) != 0 {
		t.Fatal("partial store leaked into destination")
	}
}

func TestAuditReviewBackupSQLiteWALMix(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	source, err := sql.Open("sqlite", filepath.Join(src, "Cookies"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.SetMaxOpenConns(1)
	for _, stmt := range []string{`PRAGMA journal_mode=WAL`, `PRAGMA wal_autocheckpoint=0`, `CREATE TABLE audit_owner(value TEXT)`, `INSERT INTO audit_owner VALUES ('source-profile')`} {
		if _, err := source.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	target, err := sql.Open("sqlite", filepath.Join(dst, "Cookies"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE audit_owner(value TEXT); INSERT INTO audit_owner VALUES ('existing-profile'); PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	stats := &backupMergeStats{}
	if err := backupSyncDir(src, dst, stats, nil); err != nil {
		t.Fatal(err)
	}
	mixed, err := sql.Open("sqlite", filepath.Join(dst, "Cookies"))
	if err != nil {
		t.Fatal(err)
	}
	defer mixed.Close()
	var value string
	err = mixed.QueryRow(`SELECT value FROM audit_owner`).Scan(&value)
	if err != nil || value != "existing-profile" {
		t.Fatalf("conflicting Cookies file was skipped, but copied WAL changed its logical contents: value=%q err=%v stats=%+v", value, err, stats)
	}
}
