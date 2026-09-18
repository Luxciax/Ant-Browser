package snapshot

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestUnzipToWithLimitsRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "unsafe.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("unsafe")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := UnzipToWithLimits(zipPath, filepath.Join(root, "extract"), UnzipLimits{MaxEntries: 10}); err == nil {
		t.Fatal("path traversal archive was accepted")
	}
}

func TestUnzipToWithLimitsEnforcesExpandedSize(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "large.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("payload/data.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("0123456789")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	err = UnzipToWithLimits(zipPath, filepath.Join(root, "extract"), UnzipLimits{
		MaxEntries:           10,
		MaxUncompressedBytes: 5,
		MaxSingleFileBytes:   5,
	})
	if err == nil {
		t.Fatal("oversized archive was accepted")
	}
}

func TestZipDirRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(sourceDir, "linked.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("symlink is not available in this environment: %v", err)
	}
	if err := ZipDir(sourceDir, filepath.Join(root, "snapshot.zip")); err == nil {
		t.Fatal("snapshot archive accepted a symlink")
	}
}
