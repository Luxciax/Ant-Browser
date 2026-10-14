//go:build windows

package profiletxn

import (
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsLockedFileRetainsJournalAndRetryRecovers(t *testing.T) {
	for _, commit := range []bool{false, true} {
		name := "rollback"
		if commit {
			name = "committed-cleanup"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			db, err := database.NewDB(filepath.Join(root, "app.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if err := db.Migrate(); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(root, "profile")
			source := filepath.Join(root, "staging", "profile")
			writeMarker(t, target, "old")
			writeMarker(t, source, "new")
			txn, err := Begin(db.GetConn(), Plan{Kind: "import", StagingRoot: filepath.Join(root, "staging"), Moves: []Move{{Source: source, Target: target}}}, []string{root})
			if err != nil {
				t.Fatal(err)
			}
			if err := txn.Apply(); err != nil {
				t.Fatal(err)
			}
			lockedDir := target
			if commit {
				tx, err := db.GetConn().Begin()
				if err != nil {
					t.Fatal(err)
				}
				if err := txn.Commit(tx); err != nil {
					t.Fatal(err)
				}
				lockedDir = txn.backup(txn.Plan.Moves[0])
			}
			path, err := windows.UTF16PtrFromString(filepath.Join(lockedDir, "marker"))
			if err != nil {
				t.Fatal(err)
			}
			handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, resolveErr := txn.Resolve()
			_ = windows.CloseHandle(handle)
			if resolveErr == nil {
				t.Fatal("expected Windows sharing violation")
			}
			var count int
			if err := db.GetConn().QueryRow(`SELECT count(*) FROM profile_file_transactions`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("lost recovery journal: %d %v", count, err)
			}
			if data, err := os.ReadFile(filepath.Join(txn.backup(txn.Plan.Moves[0]), "marker")); err != nil || string(data) != "old" {
				t.Fatalf("lost backup: %q %v", data, err)
			}
			if err := Recover(db.GetConn(), []string{root}); err != nil {
				t.Fatal(err)
			}
			want := "old"
			if commit {
				want = "new"
			}
			if data, err := os.ReadFile(filepath.Join(target, "marker")); err != nil || string(data) != want {
				t.Fatalf("retry: %q %v want %s", data, err, want)
			}
		})
	}
}
