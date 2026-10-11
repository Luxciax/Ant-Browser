package logger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWriterActuallyRotatesAndHistoryIsRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	w, err := NewFileWriter(DefaultFileWriterConfig(path), NewJSONFormatter())
	if err != nil {
		t.Fatal(err)
	}
	w.rotation = NewRotationManager(RotationManagerConfig{BasePath: path, MaxBackups: 2, Policy: NewSizeRotationPolicy(1)})
	first := NewLogEntry(INFO, "test", "first")
	w.Write(first)
	w.Flush()
	second := NewLogEntry(ERROR, "test", "https://user:password@example.com/?token=secret")
	w.Write(second)
	w.Flush()
	w.Close()
	archives, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "app.*.log"))
	if len(archives) != 1 {
		t.Fatalf("rotation did not happen: %v", archives)
	}
	rows, err := ReadHistory(path, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("history %#v %v", rows, err)
	}
	recent, err := ReadRecentHistory(path, 10)
	if err != nil || len(recent) != 2 || recent[0].Message != "first" {
		t.Fatalf("rotated crash history unavailable: %#v %v", recent, err)
	}
	if strings.Contains(rows[0].Message, "password") || strings.Contains(rows[0].Message, "secret") {
		t.Fatalf("history not redacted: %s", rows[0].Message)
	}
}

func TestDiskFailureDoesNotHideMemoryLogs(t *testing.T) {
	InitWithConfig(context.Background(), LoggerConfig{Level: "info", FileEnabled: true, FilePath: filepath.Join(t.TempDir(), "missing", "app.log"), Format: "json"})
	defer Close()
	l := New("test")
	l.fileWriteFailed = true
	GetMemoryWriter().Clear()
	l.Error("still visible")
	rows := GetMemoryWriter().GetEntries()
	if len(rows) != 1 || rows[0].Message != "still visible" {
		t.Fatal("file failure hid in-memory error")
	}
}

func TestAsyncPersistenceReportsWriteFailure(t *testing.T) {
	w, err := NewAsyncFileWriter(DefaultFileWriterConfig(filepath.Join(t.TempDir(), "app.log")), NewJSONFormatter())
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.mu.Lock()
	w.file.Close()
	w.mu.Unlock()
	w.Write(NewLogEntry(ERROR, "test", strings.Repeat("x", 8192)))
	deadline := time.Now().Add(time.Second)
	for w.Error() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if w.Error() == nil {
		t.Fatal("background write error was hidden")
	}
}

func TestHistorySkipsIncompleteLastRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	os.WriteFile(path, []byte("{\"timestamp\":\"2026-10-11T10:00:00Z\",\"level\":\"ERROR\",\"message\":\"complete\"}\n{\"unfinished\":"), 0600)
	rows, err := ReadHistory(path, 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("history %v %v", rows, err)
	}
}
func TestAsyncWriterCloseCanRaceWithProducers(t *testing.T) {
	w, err := NewAsyncFileWriter(DefaultFileWriterConfig(filepath.Join(t.TempDir(), "app.log")), NewJSONFormatter())
	if err != nil {
		t.Fatal(err)
	}
	var producers sync.WaitGroup
	for i := 0; i < 8; i++ {
		producers.Add(1)
		go func() {
			defer producers.Done()
			for j := 0; j < 100; j++ {
				_ = w.Write(NewLogEntry(INFO, "test", "shutdown race"))
			}
		}()
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	producers.Wait()
	if err := w.Write(NewLogEntry(INFO, "test", "after close")); err == nil {
		t.Fatal("closed writer accepted log")
	}
	if err := w.Close(); err != nil {
		t.Fatal("second close failed", err)
	}
}
