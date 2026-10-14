package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const incidentProducer = "ant-browser-diagnostics-v2"

type diagnosticIncident struct {
	Producer       string                   `json:"producer"`
	ID             string                   `json:"incident_id"`
	ProfileID      string                   `json:"profile_id"`
	RunID          string                   `json:"run_id"`
	Event          string                   `json:"event"`
	Started        time.Time                `json:"occurred_at"`
	Updated        time.Time                `json:"updated_at"`
	Status         string                   `json:"status"`
	Facts          []map[string]interface{} `json:"facts"`
	Snapshot       map[string]interface{}   `json:"snapshot"`
	Resources      map[string]interface{}   `json:"resources"`
	SystemEvidence map[string]interface{}   `json:"system_evidence"`
	Coverage       map[string]string        `json:"coverage"`
	Timeline       []logger.MemoryLogEntry  `json:"timeline"`
	Omitted        int                      `json:"omitted_events"`
	FactsOmitted   int                      `json:"omitted_facts"`
}

func (a *App) incidentDir() string {
	if a.appRoot == "" {
		return ""
	}
	return filepath.Join(a.appRoot, "data", "diagnostics", "incidents")
}

// Called outside browser-manager locks; OS collection is bounded and asynchronous.
func (a *App) recordDiagnosticIncident(profile, run, event string, fields map[string]interface{}) string {
	now := time.Now()
	a.incidentMu.Lock()
	a.loadSavedIncidentsLocked()
	if a.incidents == nil {
		a.incidents = map[string]*diagnosticIncident{}
	}
	var incident *diagnosticIncident
	for i := len(a.incidentOrder) - 1; i >= 0; i-- {
		candidate := a.incidents[a.incidentOrder[i]]
		if candidate.ProfileID == profile && candidate.RunID == run && now.Sub(candidate.Started) < 30*time.Second {
			incident = candidate
			break
		}
	}
	if incident == nil {
		id := fmt.Sprintf("incident-%s-%d", now.UTC().Format("20060102T150405.000000000"), len(a.incidentOrder))
		incident = &diagnosticIncident{ID: id, ProfileID: profile, RunID: run, Event: event, Started: now, Status: "collecting", Coverage: map[string]string{"cause": "unknown: a symptom alone does not establish the root cause", "os": "pending", "kernel_output": "pending"}, Snapshot: map[string]interface{}{}, SystemEvidence: map[string]interface{}{}}
		a.incidents[id] = incident
		a.incidentOrder = append(a.incidentOrder, id)
		if len(a.incidentOrder) > 20 {
			delete(a.incidents, a.incidentOrder[0])
			a.incidentOrder = a.incidentOrder[1:]
		}
	}
	incident.Updated = now
	fact := logger.RedactValue(map[string]interface{}{"observed_at": now.Format(time.RFC3339Nano), "event": event, "fields": fields}).(map[string]interface{})
	encodedFact, _ := json.Marshal(fact)
	if len(encodedFact) > 64*1024 {
		fact = map[string]interface{}{"event": event, "observed_at": now.Format(time.RFC3339Nano), "fields_omitted": "fact exceeded 64KB budget"}
		incident.FactsOmitted++
	}
	encodedFacts, _ := json.Marshal(incident.Facts)
	if len(incident.Facts) < 64 && len(encodedFacts)+len(encodedFact) <= 512*1024 {
		incident.Facts = append(incident.Facts, fact)
	} else {
		incident.FactsOmitted++
	}
	id := incident.ID
	fresh := len(incident.Facts) == 1
	a.incidentMu.Unlock()
	if fresh {
		logger.New("Diagnostics").Error("故障证据已登记", logger.F("event", "incident.opened"), logger.F("incident_id", id), logger.F("profile_id", profile), logger.F("run_id", run), logger.F("source_event", event))
	}
	if fresh {
		go a.collectDiagnosticIncident(id)
	} else {
		go a.refreshAndPersistIncident(id, false)
	}
	return id
}

func (a *App) collectDiagnosticIncident(id string) {
	a.incidentMu.Lock()
	source := a.incidents[id]
	if source == nil {
		a.incidentMu.Unlock()
		return
	}
	profile, run, started := source.ProfileID, source.RunID, source.Started
	facts := append([]map[string]interface{}{}, source.Facts...)
	a.incidentMu.Unlock()
	a.refreshAndPersistIncident(id, false)
	snapshot := map[string]interface{}{"profile_id": profile, "run_id": run}
	corePath := ""
	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		if p := a.browserMgr.Profiles[profile]; p != nil && p.LastStartAt == run {
			snapshot["pid"] = p.Pid
			snapshot["debug_port"] = p.DebugPort
			snapshot["running"] = p.Running
			snapshot["debug_ready"] = p.DebugReady
			snapshot["memory_limit_mb"] = p.MemoryLimitMB
			snapshot["core_id"] = p.CoreId
			snapshot["user_data_dir"] = a.browserMgr.ResolveUserDataDir(p)
			snapshot["launch_args"] = diagnosticLaunchArgs(p.LastLaunchArgs)
			if p.Pid > 0 {
				handle := openProcessEvidence(p.Pid)
				snapshot["process"] = handle.snapshot()
				handle.close()
			}
			snapshot["core_id"] = p.CoreId
		} else {
			snapshot["runtime_identity"] = "run no longer active"
		}
		a.browserMgr.Mutex.Unlock()
		a.browserMgr.Mutex.Lock()
		if m := a.browserProcessMonitors[profile]; m != nil {
			m.mu.Lock()
			if m.runID == run {
				corePath = m.kernelBinary
			}
			m.mu.Unlock()
		}
		a.browserMgr.Mutex.Unlock()
	}
	// Prefer evidence recorded at the fault over an identity reopened after exit.
	for _, fact := range facts {
		if f, ok := fact["fields"].(map[string]interface{}); ok {
			if process, ok := f["process"].(map[string]interface{}); ok {
				snapshot["process_at_fault"] = process
				if pid, ok := process["pid"]; ok {
					snapshot["pid"] = pid
				}
			}
			if pid, ok := f["pid"]; ok {
				snapshot["pid"] = pid
			}
			if kernel, ok := f["kernel_binary"].(string); ok && kernel != "" {
				corePath = kernel
			}
		}
	}
	snapshot["app_version"] = a.appVersion()
	if corePath != "" {
		snapshot["kernel_identity"] = kernelIdentity(corePath)
	}
	resources := systemResourceEvidence()
	resources["diagnostics_disk"] = diagnosticDiskEvidence(a.incidentDir())
	osEvidence := collectCrashEvidence(started, snapshot, corePath)
	a.incidentMu.Lock()
	if current := a.incidents[id]; current != nil {
		current.Snapshot = logger.RedactValue(snapshot).(map[string]interface{})
		current.Resources = resources
		current.SystemEvidence = osEvidence
		current.Coverage["os"] = fmt.Sprint(osEvidence["state"])
		current.Coverage["kernel_output"] = "managed stderr only; inheritance after launcher handoff is not guaranteed; see diagnostics.kernel_coverage"
	}
	a.incidentMu.Unlock()
	a.refreshAndPersistIncident(id, false)
	// The initial file survives an Ant crash; follow-up adds recovery/stop context.
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	if a.ctx != nil {
		select {
		case <-timer.C:
		case <-a.ctx.Done():
		}
	} else {
		<-timer.C
	}
	a.refreshAndPersistIncident(id, true)
}

func (a *App) refreshAndPersistIncident(id string, complete bool) {
	a.incidentPersistMu.Lock()
	defer a.incidentPersistMu.Unlock()
	a.incidentMu.Lock()
	incident := a.incidents[id]
	if incident == nil {
		a.incidentMu.Unlock()
		return
	}
	incident.Producer = incidentProducer
	if incident.Coverage == nil {
		incident.Coverage = map[string]string{}
	}
	rows, dropped := logger.GetMemoryWriter().RunEntries(incident.ProfileID, incident.RunID)
	incident.Timeline = logger.MergeEntries(append(incident.Timeline, rows...))
	bytes := 0
	first := len(incident.Timeline)
	for first > 0 && len(incident.Timeline)-first < 320 {
		raw, _ := json.Marshal(incident.Timeline[first-1])
		if bytes+len(raw) > 1024*1024 {
			break
		}
		bytes += len(raw)
		first--
	}
	if first > incident.Omitted {
		incident.Omitted = first
	}
	incident.Timeline = incident.Timeline[first:]
	if dropped > incident.Omitted {
		incident.Omitted = dropped
	}
	incident.Updated = time.Now()
	if complete {
		incident.Status = "complete"
	}
	incident.Coverage["timeline"] = fmt.Sprintf("bounded per-run history; %d events omitted; timeline <= 1MB, facts <= 512KB", incident.Omitted)
	safe, err := safeIncident(incident)
	var data []byte
	if err == nil {
		data, err = json.MarshalIndent(safe, "", "  ")
	}
	a.incidentMu.Unlock()
	if err != nil {
		logger.New("Diagnostics").Error("故障包编码失败", logger.F("incident_id", id), logger.F("error", err))
		return
	}
	dir := a.incidentDir()
	if dir == "" {
		return
	}
	if err = os.MkdirAll(dir, 0700); err == nil {
		err = fsutil.AtomicWriteFile(filepath.Join(dir, id+".json"), data, 0600)
	}
	if err != nil {
		a.incidentMu.Lock()
		if p := a.incidents[id]; p != nil {
			p.Coverage["persistence"] = err.Error()
		}
		a.incidentMu.Unlock()
		logger.New("Diagnostics").Error("故障包落盘失败", logger.F("incident_id", id), logger.F("error", err))
		return
	}
	a.trimIncidentFiles(dir)
}

func (a *App) trimIncidentFiles(dir string) {
	all, _ := filepath.Glob(filepath.Join(dir, "incident-*.json"))
	files := []string{}
	for _, path := range all {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		var meta struct {
			Producer string `json:"producer"`
			ID       string `json:"incident_id"`
		}
		err = json.NewDecoder(io.LimitReader(f, 4*1024*1024)).Decode(&meta)
		f.Close()
		if err == nil && meta.Producer == incidentProducer && filepath.Base(path) == meta.ID+".json" && !strings.ContainsAny(meta.ID, "/\\\\") {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	if len(files) > 20 {
		for _, path := range files[:len(files)-20] {
			if err := os.Remove(path); err != nil {
				logger.New("Diagnostics").Warn("故障包轮转失败", logger.F("error", err))
			}
		}
	}
}

func (a *App) loadSavedIncidentsLocked() {
	if a.incidentLoaded {
		return
	}
	a.incidentLoaded = true
	if a.incidents == nil {
		a.incidents = map[string]*diagnosticIncident{}
	}
	dir := a.incidentDir()
	if dir == "" {
		return
	}
	files, _ := filepath.Glob(filepath.Join(dir, "incident-*.json"))
	sort.Strings(files)
	if len(files) > 20 {
		files = files[len(files)-20:]
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		var v diagnosticIncident
		err = json.NewDecoder(io.LimitReader(f, 4*1024*1024)).Decode(&v)
		f.Close()
		if err == nil && v.Producer == incidentProducer && strings.HasPrefix(v.ID, "incident-") && !strings.ContainsAny(v.ID, "/\\\\") && filepath.Base(path) == v.ID+".json" && a.incidents[v.ID] == nil {
			if v.Status == "collecting" {
				v.Status = "interrupted"
				if v.Coverage == nil {
					v.Coverage = map[string]string{}
				}
				v.Coverage["collection"] = "previous session ended before follow-up; initial evidence retained"
			}
			a.incidents[v.ID] = &v
			a.incidentOrder = append(a.incidentOrder, v.ID)
		}
	}
}

func (a *App) GetBrowserDiagnosticIncidents() []map[string]interface{} {
	a.incidentMu.Lock()
	defer a.incidentMu.Unlock()
	a.loadSavedIncidentsLocked()
	out := []map[string]interface{}{}
	for _, id := range a.incidentOrder {
		p := a.incidents[id]
		out = append(out, map[string]interface{}{"incident_id": id, "profile_id": p.ProfileID, "run_id": p.RunID, "event": p.Event, "occurred_at": p.Started.Format(time.RFC3339Nano), "status": p.Status})
	}
	return out
}

func (a *App) incidentReport(id string) (map[string]interface{}, error) {
	a.incidentMu.Lock()
	a.loadSavedIncidentsLocked()
	p := a.incidents[id]
	a.incidentMu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("故障记录不存在")
	}
	a.refreshAndPersistIncident(id, false)
	a.incidentMu.Lock()
	defer a.incidentMu.Unlock()
	safe, err := safeIncident(a.incidents[id])
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(safe)
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	out["schema_version"] = 2
	out["logger_health"] = logger.Health()
	return out, nil
}

// Normalize JSON tags and time.Time before reflection-based redaction.
func safeIncident(v *diagnosticIncident) (map[string]interface{}, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	normalized := map[string]interface{}{}
	if err = json.Unmarshal(raw, &normalized); err != nil {
		return nil, err
	}
	return logger.RedactValue(normalized).(map[string]interface{}), nil
}

func (a *App) GetBrowserDiagnosticIncident(id string) (map[string]interface{}, error) {
	return a.incidentReport(id)
}

func (a *App) GetBrowserDiagnosticRunLogs(profile, run string) []logger.MemoryLogEntry {
	rows, _ := logger.GetMemoryWriter().RunEntries(profile, run)
	a.incidentMu.Lock()
	defer a.incidentMu.Unlock()
	a.loadSavedIncidentsLocked()
	for _, incident := range a.incidents {
		if incident.ProfileID == profile && incident.RunID == run {
			rows = append(rows, incident.Timeline...)
		}
	}
	return logger.MergeEntries(rows)
}
