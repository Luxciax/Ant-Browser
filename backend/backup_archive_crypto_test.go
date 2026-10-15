package backend

import (
	"ant-chrome/backend/internal/backup"
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func rewriteEncryptedTestPayload(t *testing.T, source, target string, change func([]byte) []byte) {
	t.Helper()
	r, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	f, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, entry := range r.File {
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == "payload.bin" {
			data = change(data)
		}
		writer, err := w.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEncryptedFullBackupStreamsWithoutPlainArchive(t *testing.T) {
	outputDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "private-local-storage.bin")
	data := make([]byte, 3*backupCryptoChunkSize)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	scope := backup.Scope{Format: backup.PackageFormat, ManifestVersion: backup.ManifestVersion, Entries: []backup.ScopeEntry{{
		ID: "test-storage", Category: backup.CategoryBrowserData, EntryType: backup.EntryTypeFile,
		Required: true, SourcePath: source, ArchivePath: "payload/private-local-storage.bin",
	}}}
	manifest := backup.BuildManifest(scope, "test", "test", time.Now())
	checked := false
	inspect := func(_ string, progress int, _ string, _ *backupProgressMeta) {
		if progress != 94 {
			return
		}
		entries, err := os.ReadDir(outputDir)
		if err != nil || len(entries) != 1 || entries[0].IsDir() {
			t.Fatal("expected only one encrypted temporary output, no plaintext staging directory")
		}
		partial, err := os.ReadFile(filepath.Join(outputDir, entries[0].Name()))
		if err != nil || !bytes.Contains(partial, []byte(encryptedBackupFormat)) || bytes.Contains(partial, []byte("private-local-storage.bin")) {
			t.Fatal("temporary output is not the encrypted container")
		}
		checked = true
	}
	zipPath := filepath.Join(outputDir, "encrypted.zip")
	if _, _, _, err := backupWritePackageZip(zipPath, scope, manifest, inspect, "synthetic-stream-password"); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("did not inspect in-progress output")
	}
	plain := filepath.Join(t.TempDir(), "inner.zip")
	if err := decryptFullBackupArchive(zipPath, plain, "synthetic-stream-password"); err != nil {
		t.Fatal(err)
	}
	extractRoot, _, err := backupExtractAndValidate(plain)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(extractRoot)
	got, err := os.ReadFile(filepath.Join(extractRoot, "payload", "private-local-storage.bin"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("streamed package content changed")
	}

	before, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	scope.Entries[0].SourcePath = filepath.Join(t.TempDir(), "missing-required-file")
	if _, _, _, err := backupWritePackageZip(zipPath, scope, manifest, nil, "synthetic-stream-password"); err == nil {
		t.Fatal("missing required source was accepted")
	}
	after, err := os.ReadFile(zipPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed export overwrote existing backup")
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil || len(entries) != 2 {
		t.Fatal("failed export left temporary files")
	}
}

func TestEncryptedFullBackupChunksAndTamper(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.zip")
	want := bytes.Repeat([]byte("synthetic-site-storage-token\x00"), 100000)
	if err := os.WriteFile(plain, want, 0600); err != nil {
		t.Fatal(err)
	}
	sealed := filepath.Join(dir, "sealed.zip")
	if err := encryptFullBackupArchive(plain, sealed, "synthetic-backup-password"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("synthetic-site-storage-token")) {
		t.Fatal("website data is present in public encrypted container")
	}
	out := filepath.Join(dir, "restored.zip")
	if err := decryptFullBackupArchive(sealed, out, "synthetic-backup-password"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("multi-chunk round-trip mismatch")
	}
	if err := decryptFullBackupArchive(sealed, filepath.Join(dir, "wrong.zip"), "synthetic-wrong-password"); err == nil {
		t.Fatal("wrong password accepted")
	}
	cases := map[string]func([]byte) []byte{
		"truncated": func(data []byte) []byte { return data[:len(data)-16] },
		"modified":  func(data []byte) []byte { data[12] ^= 1; return data },
		"trailing":  func(data []byte) []byte { return append(data, 1) },
		"reordered": func(data []byte) []byte {
			first := 4 + int(binary.BigEndian.Uint32(data[:4])) + 16
			second := 4 + int(binary.BigEndian.Uint32(data[first:first+4])) + 16
			return append(append(append([]byte{}, data[first:first+second]...), data[:first]...), data[first+second:]...)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			broken := filepath.Join(dir, name+".zip")
			rewriteEncryptedTestPayload(t, sealed, broken, change)
			if err := decryptFullBackupArchive(broken, filepath.Join(dir, name+"-out.zip"), "synthetic-backup-password"); err == nil {
				t.Fatal("unauthenticated archive accepted")
			}
		})
	}
}
