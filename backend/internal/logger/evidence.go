package logger

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

var evidenceStarted = time.Now()
var evidenceSession = fmt.Sprintf("%s-%d", evidenceStarted.UTC().Format("20060102T150405.000000000"), os.Getpid())
var evidenceSequence atomic.Uint64

func addEvidenceFields(entry *LogEntry) {
	if entry.Fields == nil {
		entry.Fields = map[string]interface{}{}
	}
	seq := evidenceSequence.Add(1)
	entry.Fields["app_session_id"] = evidenceSession
	entry.Fields["event_id"] = fmt.Sprintf("%s-%d", evidenceSession, seq)
	entry.Fields["event_seq"] = seq
	entry.Fields["app_elapsed_ms"] = time.Since(evidenceStarted).Milliseconds()
	if entry.RequestID != "" {
		entry.Fields["operation_id"] = entry.RequestID
	}
	if entry.Method != "" {
		entry.Fields["method"] = entry.Method
	}
	if entry.Duration != 0 {
		entry.Fields["duration_ms"] = entry.Duration
	}
	if entry.Error != "" {
		entry.Fields["error"] = entry.Error
	}
}

func criticalEntry(entry *LogEntry) bool {
	return entry.Level >= ERROR || entry.Fields["critical"] == true
}
