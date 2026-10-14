package logger

import (
	"encoding/json"
	"fmt"
	"sort"
)

type runLogBuffer struct {
	entries   []MemoryLogEntry
	important []MemoryLogEntry
	bytes     int
	dropped   int
}

func (w *MemoryWriter) recordRunLocked(entry MemoryLogEntry, critical bool) {
	id, run := fmt.Sprint(entry.Fields["profile_id"]), fmt.Sprint(entry.Fields["run_id"])
	if id == "<nil>" || run == "<nil>" || id == "" || run == "" {
		return
	}
	key := id + "\n" + run
	if w.runs == nil {
		w.runs = map[string]*runLogBuffer{}
	}
	buf := w.runs[key]
	if buf == nil {
		if len(w.runOrder) >= 32 {
			delete(w.runs, w.runOrder[0])
			w.runOrder = w.runOrder[1:]
		}
		buf = &runLogBuffer{}
		w.runs[key] = buf
		w.runOrder = append(w.runOrder, key)
	}
	data, _ := json.Marshal(entry)
	if len(data) > 16384 {
		fields := map[string]interface{}{"profile_id": id, "run_id": run, "truncated": true}
		for _, name := range []string{"event_id", "event_seq", "app_session_id", "event", "error"} {
			if v, ok := entry.Fields[name]; ok {
				if s, ok := v.(string); ok && len(s) > 2048 {
					v = s[:2048]
				}
				fields[name] = v
			}
		}
		entry.Fields = fields
		if len(entry.Message) > 2048 {
			entry.Message = entry.Message[:2048]
		}
		data, _ = json.Marshal(entry)
	}
	buf.entries = append(buf.entries, entry)
	buf.bytes += len(data)
	for len(buf.entries) > 256 || buf.bytes > 512*1024 {
		removed, _ := json.Marshal(buf.entries[0])
		buf.bytes -= len(removed)
		buf.entries = buf.entries[1:]
		buf.dropped++
	}
	if critical {
		buf.important = append(buf.important, entry)
		if len(buf.important) > 32 {
			buf.important = buf.important[1:]
		}
	}
}

func (w *MemoryWriter) RunEntries(profile, run string) ([]MemoryLogEntry, int) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	buf := w.runs[profile+"\n"+run]
	if buf == nil {
		return []MemoryLogEntry{}, 0
	}
	rows := append(append([]MemoryLogEntry{}, buf.entries...), buf.important...)
	return MergeEntries(rows), buf.dropped
}

// MergeEntries keeps event identity and ordering through async writes/rotation.
func MergeEntries(rows []MemoryLogEntry) []MemoryLogEntry {
	seen := map[string]bool{}
	out := make([]MemoryLogEntry, 0, len(rows))
	for _, row := range rows {
		key := fmt.Sprint(row.Fields["event_id"])
		if key == "<nil>" || key == "" {
			data, _ := json.Marshal(row)
			key = string(data)
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, row)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if fmt.Sprint(a.Fields["app_session_id"]) == fmt.Sprint(b.Fields["app_session_id"]) {
			seq := func(v interface{}) float64 {
				switch n := v.(type) {
				case uint64:
					return float64(n)
				case float64:
					return n
				case int:
					return float64(n)
				}
				return 0
			}
			x, y := seq(a.Fields["event_seq"]), seq(b.Fields["event_seq"])
			if x != 0 && y != 0 {
				return x < y
			}
		}
		return a.Time < b.Time
	})
	return out
}
