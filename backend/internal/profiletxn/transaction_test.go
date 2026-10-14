package profiletxn

import (
	"ant-chrome/backend/internal/database"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func writeMarker(t *testing.T, dir, value string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "marker"), []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

// The parent kills this process at a persisted checkpoint. No defers run and
// SQLite must recover an open transaction using its real WAL on reopening.
func TestCrashHelper(t *testing.T) {
	root := os.Getenv("ANT_PROFILE_TX_TEST_ROOT")
	if root == "" {
		return
	}
	phase := os.Getenv("ANT_PROFILE_TX_TEST_PHASE")
	kind := os.Getenv("ANT_PROFILE_TX_TEST_KIND")
	db, err := database.NewDB(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{Kind: kind}
	for _, name := range []string{"existing", "new", "extension"} {
		m := Move{Target: filepath.Join(root, name)}
		if kind == "import" {
			m.Source = filepath.Join(root, "staging", name)
		}
		plan.Moves = append(plan.Moves, m)
	}
	if kind == "import" {
		plan.StagingRoot = filepath.Join(root, "staging")
	}
	txn, err := Begin(db.GetConn(), plan, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if phase == "original-staged" {
		m := txn.Plan.Moves[0]
		if err := os.Rename(m.Target, txn.backup(m)); err != nil {
			t.Fatal(err)
		}
	} else if phase != "intent" {
		if err := txn.Apply(); err != nil {
			t.Fatal(err)
		}
	}
	if phase == "sql-uncommitted" || phase == "committed" || phase == "cleanup-partial" {
		tx, err := db.GetConn().Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`UPDATE audit_state SET value = 'new'`); err != nil {
			t.Fatal(err)
		}
		if err := txn.MarkCommitted(tx); err != nil {
			t.Fatal(err)
		}
		if phase != "sql-uncommitted" {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if phase == "rollback-partial" {
		// Recovery completed one old-directory restore but died before removing
		// the journal. The next recovery must not delete this restored directory.
		m := txn.Plan.Moves[0]
		if kind == "import" {
			if err := os.RemoveAll(m.Target); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Rename(txn.backup(m), m.Target); err != nil {
			t.Fatal(err)
		}
	}
	if phase == "cleanup-partial" {
		if err := os.RemoveAll(txn.backup(txn.Plan.Moves[0])); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ready"), []byte(phase), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestRecoverAfterProcessKilled(t *testing.T) {
	for _, kind := range []string{"import", "delete"} {
		for _, phase := range []string{"intent", "original-staged", "files-applied", "sql-uncommitted", "committed", "rollback-partial", "cleanup-partial"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				root := t.TempDir()
				db, err := database.NewDB(filepath.Join(root, "app.db"))
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Migrate(); err != nil {
					t.Fatal(err)
				}
				if _, err := db.GetConn().Exec(`CREATE TABLE audit_state(value TEXT); INSERT INTO audit_state VALUES ('old')`); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"existing", "extension"} {
					writeMarker(t, filepath.Join(root, name), "old-"+name)
				}
				writeMarker(t, filepath.Join(root, "unrelated"), "keep")
				if kind == "import" {
					for _, name := range []string{"existing", "new", "extension"} {
						writeMarker(t, filepath.Join(root, "staging", name), "new-"+name)
					}
				}
				exe, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(exe, "-test.run=^TestCrashHelper$")
				cmd.Env = append(os.Environ(), "ANT_PROFILE_TX_TEST_ROOT="+root, "ANT_PROFILE_TX_TEST_PHASE="+phase, "ANT_PROFILE_TX_TEST_KIND="+kind)
				var output bytes.Buffer
				cmd.Stdout = &output
				cmd.Stderr = &output
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				deadline := time.Now().Add(10 * time.Second)
				ready := false
				for time.Now().Before(deadline) {
					if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
						ready = true
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				if !ready {
					t.Fatalf("child did not reach checkpoint: %s", output.String())
				}
				db, err = database.NewDB(filepath.Join(root, "app.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
				for attempt := 0; attempt < 2; attempt++ {
					if err := Recover(db.GetConn(), []string{root}); err != nil {
						t.Fatalf("recovery %d: %v", attempt, err)
					}
				}
				committed := phase == "committed" || phase == "cleanup-partial"
				var state string
				if err := db.GetConn().QueryRow(`SELECT value FROM audit_state`).Scan(&state); err != nil {
					t.Fatal(err)
				}
				wantState := "old"
				if committed {
					wantState = "new"
				}
				if state != wantState {
					t.Fatalf("SQL=%s want %s", state, wantState)
				}
				for _, name := range []string{"existing", "new", "extension"} {
					content, err := os.ReadFile(filepath.Join(root, name, "marker"))
					if (committed && kind == "delete") || (!committed && name == "new") {
						if !os.IsNotExist(err) {
							t.Fatalf("unexpected target %s: %v", name, err)
						}
						continue
					}
					want := "old-" + name
					if committed {
						want = "new-" + name
					}
					if err != nil || string(content) != want {
						t.Fatalf("%s: %q %v want %q", name, content, err, want)
					}
				}
				if content, err := os.ReadFile(filepath.Join(root, "unrelated", "marker")); err != nil || string(content) != "keep" {
					t.Fatal("unrelated data changed")
				}
				var pending int
				if err := db.GetConn().QueryRow(`SELECT count(*) FROM profile_file_transactions`).Scan(&pending); err != nil || pending != 0 {
					t.Fatalf("journal not cleared: %d %v", pending, err)
				}
			})
		}
	}
}

func TestJournalRejectsOverlappingOrOutsideDirectories(t *testing.T) {
	root := t.TempDir()
	db, err := database.NewDB(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	for _, moves := range [][]Move{
		{{Target: root}},
		{{Target: filepath.Join(t.TempDir(), "outside")}},
		{{Target: filepath.Join(root, "parent")}, {Target: filepath.Join(root, "parent", "child")}},
	} {
		if _, err := Begin(db.GetConn(), Plan{Kind: "delete", Moves: moves}, []string{root}); err == nil {
			t.Fatalf("accepted invalid plan %#v", moves)
		}
	}
	outside := t.TempDir()
	link := filepath.Join(root, "junction")
	if err := os.Symlink(outside, link); err != nil {
		t.Logf("symlink test unavailable: %v", err)
		return
	}
	if _, err := Begin(db.GetConn(), Plan{Kind: "delete", Moves: []Move{{Target: filepath.Join(link, "not-created")}}}, []string{root}); err == nil {
		t.Fatal("accepted symlink ancestor escape")
	}
}

func TestRecoveryPreservesFilesWhenJournalCannotBeRead(t *testing.T) {
	root := t.TempDir()
	db, err := database.NewDB(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "profile")
	writeMarker(t, target, "old")
	txn, err := Begin(db.GetConn(), Plan{Kind: "delete", Moves: []Move{{Target: target}}}, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := txn.Resolve(); err == nil {
		t.Fatal("guessed state with unavailable DB")
	}
	if data, err := os.ReadFile(filepath.Join(txn.backup(txn.Plan.Moves[0]), "marker")); err != nil || string(data) != "old" {
		t.Fatal("backup changed despite unavailable DB")
	}
	db, err = database.NewDB(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Recover(db.GetConn(), []string{root}); err != nil {
		t.Fatal(err)
	}
}
