package logger

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ProcessLog struct {
	mu        sync.Mutex
	Path      string
	pending   []byte
	written   int64
	truncated bool
	oversized bool
}

// Attempt-specific redaction happens before disk writes. No descriptor is held
// between writes, so an early Close in a legacy launcher cannot break its child.
func NewProcessLog(base string) (*ProcessLog, error) {
	ext := filepath.Ext(base)
	path := strings.TrimSuffix(base, ext) + "-" + time.Now().UTC().Format("20060102T150405.000000000") + ext
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	files, _ := filepath.Glob(strings.TrimSuffix(base, ext) + "-*" + ext)
	sort.Strings(files)
	if len(files) > 4 {
		for _, old := range files[:len(files)-4] {
			if err := os.Remove(old); err != nil {
				return nil, fmt.Errorf("process log rotation: %w", err)
			}
		}
	}
	return &ProcessLog{Path: path}, nil
}
func (w *ProcessLog) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	size := len(data)
	if w.oversized {
		at := bytes.IndexByte(data, '\n')
		if at < 0 {
			return size, nil
		}
		data = data[at+1:]
		w.oversized = false
	}
	w.pending = append(w.pending, data...)
	for {
		at := bytes.IndexByte(w.pending, '\n')
		if at < 0 {
			if len(w.pending) > 64*1024 {
				w.pending = nil
				w.oversized = true
				return size, w.appendLine("[oversized process output omitted]")
			}
			break
		}
		line := w.pending[:at]
		w.pending = w.pending[at+1:]
		if len(line) > 64*1024 {
			line = []byte("[oversized process output omitted]")
		}
		if err := w.appendLine(RedactDiagnosticText(string(line))); err != nil {
			return size, err
		}
	}
	return size, nil
}
func (w *ProcessLog) appendLine(line string) error {
	if w.written > 512*1024 {
		if w.truncated {
			return nil
		}
		w.truncated = true
		line = "[process output truncated at 512KB]"
	}
	f, err := os.OpenFile(w.Path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := f.WriteString(line + "\n")
	w.written += int64(n)
	return err
}
func (w *ProcessLog) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) == 0 {
		return nil
	}
	line := RedactDiagnosticText(string(w.pending))
	w.pending = nil
	return w.appendLine(line)
}
func RedactDiagnosticText(text string) string {
	text = redactLogText(text)
	return logURL.ReplaceAllStringFunc(text, func(raw string) string {
		at := strings.Index(raw, "://")
		if at < 0 {
			return "[URL omitted]"
		}
		rest := raw[at+3:]
		if end := strings.IndexAny(rest, "/?#"); end >= 0 {
			rest = rest[:end]
		}
		return raw[:at+3] + rest
	})
}
