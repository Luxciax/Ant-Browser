package logger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCriticalEventBypassesFullQueueAndFlushPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.log")
	w, err := NewFileWriter(DefaultFileWriterConfig(path), NewJSONFormatter())
	if err != nil {
		t.Fatal(err)
	}
	// A deliberately full, paused ordinary queue proves ERROR does not enqueue.
	w.asyncMode = true
	w.asyncChan = make(chan *LogEntry, 1)
	ordinary := &LogEntry{Timestamp: time.Now(), Level: INFO, Message: "queued"}
	w.asyncChan <- ordinary
	if err = w.Write(ordinary); err != ErrQueueFull {
		t.Fatalf("queue: %v", err)
	}
	if err = w.Write(&LogEntry{Timestamp: time.Now(), Level: ERROR, Message: "critical-crash"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), "critical-crash") {
		t.Fatalf("critical not durable: %s %v", raw, err)
	}
	w.asyncMode = false
	w.Close()

	w, err = NewAsyncFileWriter(DefaultFileWriterConfig(filepath.Join(t.TempDir(), "barrier.log")), NewJSONFormatter())
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for i := 0; i < 30; i++ {
		if err = w.Write(ordinary); err != nil {
			t.Fatal(err)
		}
	}
	if err = w.Flush(); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(w.GetFilePath())
	if err != nil || strings.Count(string(raw), "queued") != 30 {
		t.Fatalf("barrier: %s %v", raw, err)
	}
}

func TestRunHistorySurvivesOtherProfileFloodAndRetainsFault(t *testing.T) {
	w := &MemoryWriter{maxSize: 10}
	fault := &LogEntry{Timestamp: time.Now(), Level: ERROR, Message: "fault", Fields: map[string]interface{}{"profile_id": "a", "run_id": "run-a", "event_id": "fault"}}
	w.Write(fault)
	for i := 0; i < 1000; i++ {
		w.Write(&LogEntry{Timestamp: time.Now(), Level: INFO, Fields: map[string]interface{}{"profile_id": "b", "run_id": "run-b"}})
	}
	rows, _ := w.RunEntries("a", "run-a")
	if len(rows) != 1 || rows[0].Message != "fault" {
		t.Fatalf("lost isolated context: %v", rows)
	}
	for i := 0; i < 500; i++ {
		w.Write(&LogEntry{Timestamp: time.Now(), Level: INFO, Message: "ordinary", Fields: map[string]interface{}{"profile_id": "a", "run_id": "run-a"}})
	}
	rows, omitted := w.RunEntries("a", "run-a")
	if omitted == 0 || rows[0].Message != "fault" || len(rows) > 288 {
		t.Fatalf("lost protected fault: %d %v", omitted, rows[0])
	}
}

func TestProcessOutputRedactsSplitSecretsAndDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy.log")
	first, err := NewProcessLog(path)
	if err != nil {
		t.Fatal(err)
	}
	first.Write([]byte("Authorization: Bearer "))
	first.Write([]byte("fake-private-token\nhttps://user:pass@example.com/private?token=fake-private-token\nCookie: fake-cookie\n"))
	first.Write([]byte(strings.Repeat("x", 64*1024+1)))
	first.Write([]byte("oversized-secret-suffix\nnext-line\n"))
	first.Close()
	second, err := NewProcessLog(path)
	if err != nil {
		t.Fatal(err)
	}
	second.Write([]byte("new-run\n"))
	second.Close()
	raw, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"fake-private-token", "fake-cookie", "user:pass", "/private", "oversized-secret-suffix", "new-run"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("leaked/overwritten %s: %s", secret, raw)
		}
	}
	if !strings.Contains(string(raw), "next-line") || first.Path == second.Path {
		t.Fatal("missing retained attempt")
	}
}

func TestEvidenceSequenceIncludesTopLevelFailure(t *testing.T) {
	Init(context.Background(), "info")
	entry := &LogEntry{Timestamp: time.Now(), Level: ERROR, Error: "test failure", RequestID: "operation-test", Method: "Start", Duration: 5}
	addEvidenceFields(entry)
	if entry.Fields["event_id"] == nil || entry.Fields["operation_id"] != "operation-test" || entry.Fields["error"] != "test failure" {
		t.Fatal(entry.Fields)
	}
}

func TestFileFailureReportedAndLaterWritesRecover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failure.log")
	w, err := NewFileWriter(DefaultFileWriterConfig(path), NewJSONFormatter())
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.file.Close()
	if err = w.Write(&LogEntry{Timestamp: time.Now(), Level: ERROR, Message: "failed-on-disk"}); err == nil {
		t.Fatal("failure hidden")
	}
	if w.Error() == nil || w.failedWrites == 0 {
		t.Fatal("failure health missing")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.file = f
	w.mu.Unlock()
	if err = w.Write(&LogEntry{Timestamp: time.Now(), Level: ERROR, Message: "recovered-on-disk"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), "recovered-on-disk") {
		t.Fatal("writer did not recover", err)
	}
	if w.discardedBufferBytes == 0 || w.lastFailureError == "" {
		t.Fatal("lost bytes/failure history hidden")
	}
}
