package backend

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRollbackProfilePackageDirectorySwapsRestoresOriginalDirectory(t *testing.T) {
	root := t.TempDir()
	finalDir := filepath.Join(root, "profile")
	backupDir := filepath.Join(root, "profile-backup")
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(finalDir, "marker.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "marker.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := rollbackProfilePackageDirectorySwaps([]profilePackageDirectorySwap{{
		FinalDir: finalDir, BackupDir: backupDir, HadOriginal: true,
	}})
	if err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	marker, err := os.ReadFile(filepath.Join(finalDir, "marker.txt"))
	if err != nil {
		t.Fatalf("read restored marker: %v", err)
	}
	if string(marker) != "old" {
		t.Fatalf("restored marker = %q, want old", marker)
	}
}

func TestRollbackProfilePackageDirectorySwapsReportsRestoreFailure(t *testing.T) {
	root := t.TempDir()
	finalDir := filepath.Join(root, "profile")
	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := rollbackProfilePackageDirectorySwaps([]profilePackageDirectorySwap{{
		FinalDir: finalDir, BackupDir: filepath.Join(root, "missing-backup"), HadOriginal: true,
	}})
	if err == nil {
		t.Fatal("rollback returned nil error for missing backup")
	}

	combined := combineProfilePackageRollbackError(errors.New("import failed"), err)
	if !strings.Contains(combined.Error(), "IMPORT_ROLLBACK_FAILED") {
		t.Fatalf("combined error = %v, want rollback failure marker", combined)
	}
}
