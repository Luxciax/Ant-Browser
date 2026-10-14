package logger

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"sort"
	"time"
)

// ReadRecentHistory includes a bounded set of rotated files; a rotation or
// restart must not erase the crash immediately preceding it from diagnostics.
func ReadRecentHistory(path string, limit int) ([]MemoryLogEntry, error) {
	files, err := NewRotationManager(RotationManagerConfig{BasePath: path}).listRotatedFiles()
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime.After(files[j].ModTime) })
	if len(files) > 2 {
		files = files[:2]
	}
	out := []MemoryLogEntry{}
	for i := len(files) - 1; i >= 0; i-- {
		rows, err := ReadHistory(files[i].Path, limit)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	rows, err := ReadHistory(path, limit)
	if err != nil {
		return nil, err
	}
	out = append(out, rows...)
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

// ReadHistory reads a bounded tail, never an entire installation log archive.
func ReadHistory(path string, limit int) ([]MemoryLogEntry, error) {
	out := []MemoryLogEntry{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	offset := info.Size() - 2*1024*1024
	if offset < 0 {
		offset = 0
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(f)
	if offset > 0 {
		if _, err = reader.ReadString('\n'); err != nil && err != io.EOF {
			return nil, err
		}
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 512*1024)
	for scanner.Scan() {
		var row jsonLogEntry
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			continue
		} // older text logs may coexist.
		entry := &LogEntry{Timestamp: time.Now(), Level: ParseLevel(row.Level), Component: row.Component, Message: row.Message, Fields: row.Fields}
		at, parseErr := time.Parse(time.RFC3339Nano, row.Timestamp)
		if parseErr == nil {
			entry.Timestamp = at
		}
		safe := redactLogEntry(entry)
		out = append(out, MemoryLogEntry{Time: safe.Timestamp.Format("2006-01-02 15:04:05.000Z07:00"), Level: safe.Level.String(), Component: safe.Component, Message: safe.Message, Fields: safe.Fields})
		if len(out) > limit {
			out = out[1:]
		}
	}
	return out, scanner.Err()
}
