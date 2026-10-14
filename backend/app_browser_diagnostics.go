package backend

import (
	"ant-chrome/backend/internal/diagnostics"
	"ant-chrome/backend/internal/logger"
	"context"
	"fmt"
	"strings"
)

func (a *App) startBrowserDiagnosticsLocked(p *BrowserProfile) {
	if a.ctx == nil || p.DebugPort <= 0 {
		return
	}
	a.stopBrowserDiagnostics(p.ProfileId)
	ctx, cancel := context.WithCancel(a.ctx)
	a.diagnosticsMu.Lock()
	if a.browserDiagnostics == nil {
		a.browserDiagnostics = map[string]context.CancelFunc{}
	}
	a.browserDiagnostics[p.ProfileId] = cancel
	a.diagnosticsMu.Unlock()
	id, runID, port := p.ProfileId, p.LastStartAt, p.DebugPort
	log := logger.New("BrowserDiagnostics")
	go diagnostics.Observe(ctx, port, func(e diagnostics.Event) {
		fields := []logger.Field{logger.F("profile_id", id), logger.F("run_id", runID), logger.F("event", e.Name)}
		for key, value := range e.Fields {
			fields = append(fields, logger.F(key, value))
		}
		switch e.Level {
		case "ERROR":
			log.Error(e.Name, fields...)
		case "WARN":
			log.Warn(e.Name, fields...)
		default:
			log.Info(e.Name, fields...)
		}
		if e.Name == "renderer.crashed" || e.Name == "diagnostics.disconnected" || e.Name == "diagnostics.connected" {
			a.browserMgr.Mutex.Lock()
			p := a.browserMgr.Profiles[id]
			if p != nil && p.Running && p.LastStartAt == runID {
				if e.Name == "diagnostics.connected" {
					if strings.HasPrefix(p.RuntimeWarning, "diagnostics.disconnected") {
						p.RuntimeWarning = ""
					}
				} else {
					p.RuntimeWarning = fmt.Sprintf("%s：已记录诊断日志（target=%v）；浏览器主进程状态不代表页面健康", e.Name, e.Fields["target_id"])
				}
			}
			snapshot := copyBrowserProfileSnapshot(p)
			a.browserMgr.Mutex.Unlock()
			if snapshot != nil {
				a.emitBrowserInstanceUpdated(snapshot)
			}
		}
	})
}

func (a *App) stopBrowserDiagnostics(id string) {
	a.diagnosticsMu.Lock()
	cancel := a.browserDiagnostics[id]
	delete(a.browserDiagnostics, id)
	a.diagnosticsMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) stopAllBrowserDiagnostics() {
	a.diagnosticsMu.Lock()
	defer a.diagnosticsMu.Unlock()
	for id, cancel := range a.browserDiagnostics {
		cancel()
		delete(a.browserDiagnostics, id)
	}
}
