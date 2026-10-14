package backend

import (
	"ant-chrome/backend/internal/logger"
	"time"
)

// Covers non-detaching launches as well. Port loss never decides process exit.
func (a *App) observeManagedProcessHealth(profile string, monitor *browserProcessMonitor) map[string]interface{} {
	if monitor.initialPID() <= 0 {
		return map[string]interface{}{"state": "unknown", "reason": "managed process identity unavailable"}
	}
	h := openProcessEvidence(monitor.initialPID())
	defer h.close()
	a.browserMgr.Mutex.Lock()
	p := a.browserMgr.Profiles[profile]
	if p == nil {
		a.browserMgr.Mutex.Unlock()
		return h.snapshot()
	}
	port, run := p.DebugPort, p.LastStartAt
	a.browserMgr.Mutex.Unlock()
	misses := 0
	degraded := false
	resourcesAt := time.Time{}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for a.browserProcessMonitorIsCurrent(profile, monitor) {
		proof := h.snapshot()
		if monitor.HasExited() {
			return proof
		}
		if canConnectDebugPort(port, 250*time.Millisecond) {
			misses = 0
			if degraded {
				a.publishDebugHealth(profile, run, port, true, proof, monitor)
				degraded = false
			}
		} else {
			misses++
			if misses >= 3 && !degraded {
				degraded = true
				a.publishDebugHealth(profile, run, port, false, proof, monitor)
				a.recordDiagnosticIncident(profile, run, "browser.debug_disconnected", map[string]interface{}{"debug_port": port, "process": proof, "kernel_binary": monitor.kernelBinary})
			}
		}
		if time.Since(resourcesAt) >= 30*time.Second {
			logger.New("BrowserDiagnostics").Info("浏览器资源快照", logger.F("event", "browser.resources"), logger.F("profile_id", profile), logger.F("run_id", run), logger.F("process", proof), logger.F("system", systemResourceEvidence()))
			resourcesAt = time.Now()
		}
		if a.ctx != nil {
			select {
			case <-monitor.Done():
				return h.snapshot()
			case <-ticker.C:
			case <-a.ctx.Done():
				return h.snapshot()
			}
		} else {
			select {
			case <-monitor.Done():
				return h.snapshot()
			case <-ticker.C:
			}
		}
	}
	return h.snapshot()
}
