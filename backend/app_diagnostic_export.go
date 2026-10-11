package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) GetAppLogStatus() map[string]string {
	path, problem := logger.PersistenceStatus()
	return map[string]string{"path": path, "error": problem}
}

func (a *App) diagnosticReport() (map[string]any, error) {
	path, problem := logger.PersistenceStatus()
	_ = logger.New("Diagnostics").Flush()
	history := []logger.MemoryLogEntry{}
	if path != "" {
		var err error
		history, err = logger.ReadRecentHistory(path, 2000)
		if err != nil {
			problem = err.Error()
		}
	}
	// Merge persisted and live entries so an async queue or disk failure cannot
	// hide the newest crash from the exported report.
	seen := map[string]bool{}
	logs := []logger.MemoryLogEntry{}
	for _, e := range append(history, logger.GetMemoryWriter().GetEntries()...) {
		payload, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		key := string(payload)
		if !seen[key] {
			seen[key] = true
			logs = append(logs, e)
		}
	}
	if len(logs) > 2000 {
		logs = logs[len(logs)-2000:]
	}
	profiles := []map[string]any{}
	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		for _, p := range a.browserMgr.Profiles {
			profiles = append(profiles, map[string]any{"profile_id": p.ProfileId, "runtime_state": p.RuntimeState, "running": p.Running, "debug_ready": p.DebugReady, "debug_port": p.DebugPort, "pid": p.Pid, "run_id": p.LastStartAt, "core_id": p.CoreId, "memory_limit_mb": p.MemoryLimitMB, "runtime_warning": p.RuntimeWarning, "last_error": p.LastError})
		}
		a.browserMgr.Mutex.Unlock()
	}
	cores := []map[string]any{}
	if a.browserMgr != nil && a.browserMgr.Config != nil {
		for _, core := range a.BrowserCoreList() {
			cores = append(cores, map[string]any{"core_id": core.CoreId, "core_path": core.CorePath})
		}
	}
	settings := map[string]any{}
	if a.config != nil {
		settings["connector_type"] = a.config.Browser.DefaultConnectorType
		settings["start_ready_timeout_ms"] = a.config.Browser.StartReadyTimeoutMs
		settings["app_memory_limit_mb"] = a.config.Runtime.MaxMemoryMB
	}
	report := map[string]any{"cores": cores, "settings": settings, "schema_version": 1, "generated_at": time.Now().Format(time.RFC3339Nano), "app_version": a.appVersion(), "persistence_error": problem, "profiles": profiles, "logs": logs, "limitations": []string{"网络事件不采集请求/响应体、Cookie、Authorization 或完整 URL", "未记录到崩溃事件不代表没有崩溃", "历史 QUIC 失败不能单独证明本次断网原因"}}
	// Normalize JSON tags before reflection-based redaction (Time -> time etc.).
	encoded, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, err
	}
	return logger.RedactValue(normalized).(map[string]any), nil
}

func (a *App) ExportBrowserDiagnostics() (map[string]any, error) {
	report, err := a.diagnosticReport()
	if err != nil {
		return nil, err
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{Title: "导出本地诊断包", DefaultFilename: "ant-diagnostics-" + time.Now().Format("20060102-150405") + ".json", Filters: []wailsruntime.FileFilter{{DisplayName: "JSON 诊断包", Pattern: "*.json"}}})
	if err != nil {
		return nil, fmt.Errorf("选择诊断包保存位置失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return map[string]any{"cancelled": true}, nil
	}
	if !strings.EqualFold(filepath.Ext(path), ".json") {
		path += ".json"
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = fsutil.AtomicWriteFile(path, payload, 0600); err != nil {
		return nil, err
	}
	return map[string]any{"cancelled": false, "path": path}, nil
}
