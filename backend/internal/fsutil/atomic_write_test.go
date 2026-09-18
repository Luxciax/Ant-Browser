package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWriteFileReplacesExistingFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := AtomicWriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", string(data))
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".config.yaml-") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("staging file was not cleaned up: %s", entry.Name())
		}
	}
}

func TestAtomicWriteFileFailureLeavesDestinationUntouched(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := AtomicWriteFile(target, []byte("new"), 0o644); err == nil {
		t.Fatal("AtomicWriteFile should fail when destination is a directory")
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("destination directory was damaged: %v", err)
	}
	if string(data) != "keep" {
		t.Fatalf("marker = %q, want keep", string(data))
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".config.yaml-") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("failed write left staging file behind: %s", entry.Name())
		}
	}
}
