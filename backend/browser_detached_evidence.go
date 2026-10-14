package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"time"
)

func selectBrowserMainProcess(items []browserUserDataProcess, port int) (browserUserDataProcess, bool) {
	var fallback browserUserDataProcess
	for _, item := range items {
		if item.PID <= 0 || isBrowserChildProcessCommandLine(item.CommandLine) {
			continue
		}
		if item.DebugPort == port {
			return item, true
		}
		if item.DebugPort == 0 && fallback.PID == 0 {
			fallback = item
		}
	}
	return fallback, fallback.PID > 0
}

// Discovery occurs on attach and on failed/unknown identity, not each tick.
func (a *App) attachBrowserEvidence(profile, run string, port int) (*processEvidenceHandle, map[string]interface{}) {
	a.browserMgr.Mutex.Lock()
	p := a.browserMgr.Profiles[profile]
	if p == nil || p.LastStartAt != run {
		a.browserMgr.Mutex.Unlock()
		return nil, map[string]interface{}{"state": "unknown", "reason": "run_changed"}
	}
	dir := a.browserMgr.ResolveUserDataDir(p)
	a.browserMgr.Mutex.Unlock()
	items, err := findBrowserUserDataProcesses(dir)
	if err != nil {
		return nil, map[string]interface{}{"state": "unknown", "reason": "process_discovery_failed", "error": err.Error()}
	}
	main, ok := selectBrowserMainProcess(items, port)
	if !ok {
		return nil, map[string]interface{}{"state": "unknown", "reason": "matching_main_process_not_found", "exit_code": nil}
	}
	handle := openProcessEvidence(main.PID)
	proof := handle.snapshot()
	if expected, err := time.Parse(time.RFC3339Nano, main.CreatedAt); err == nil {
		if actual, ok := proof["created_at_ns"].(int64); ok && actual != 0 && expected.UnixNano() != actual {
			delta := expected.UnixNano() - actual
			if delta < -time.Millisecond.Nanoseconds() || delta > time.Millisecond.Nanoseconds() {
				handle.close()
				return nil, map[string]interface{}{"state": "unknown", "reason": "process_generation_changed"}
			}
		}
	}
	proof["parent_pid"] = main.ParentPID
	proof["discovered_created_at"] = main.CreatedAt
	a.browserMgr.Mutex.Lock()
	if current := a.browserMgr.Profiles[profile]; current != nil && current.LastStartAt == run && proof["state"] == "alive" {
		current.Pid = main.PID
	}
	a.browserMgr.Mutex.Unlock()
	return handle, proof
}

func (a *App) publishDebugHealth(profile, run string, port int, ready bool, proof map[string]interface{}, monitor *browserProcessMonitor) {
	a.browserMgr.Mutex.Lock()
	p := a.browserMgr.Profiles[profile]
	if p == nil || p.LastStartAt != run || !a.browserProcessMonitorIsCurrentLocked(profile, monitor) {
		a.browserMgr.Mutex.Unlock()
		return
	}
	p.DebugReady = ready
	warning := "调试连接失联；实际浏览器状态未知，未判定退出"
	if proof["state"] == "alive" {
		warning = "浏览器进程仍存活，调试连接失联；自动化暂不可用"
	}
	if ready {
		if strings.Contains(p.RuntimeWarning, "调试连接失联") {
			p.RuntimeWarning = ""
		}
	} else {
		p.RuntimeWarning = warning
	}
	snapshot := copyBrowserProfileSnapshot(p)
	a.browserMgr.Mutex.Unlock()
	a.emitBrowserInstanceUpdated(snapshot)
	logger.New("BrowserDiagnostics").Warn("浏览器调试连接状态变化", logger.F("event", "browser.debug_health"), logger.F("profile_id", profile), logger.F("run_id", run), logger.F("debug_port", port), logger.F("debug_ready", ready), logger.F("process", proof))
}

func (a *App) monitorDetachedEvidence(profile string, port int, monitor *browserProcessMonitor) {
	a.browserMgr.Mutex.Lock()
	p := a.browserMgr.Profiles[profile]
	if p == nil {
		a.browserMgr.Mutex.Unlock()
		return
	}
	run := p.LastStartAt
	monitor.mu.Lock()
	monitor.runID = run
	monitor.profileID = profile
	monitor.mu.Unlock()
	a.browserMgr.Mutex.Unlock()
	handle, proof := a.attachBrowserEvidence(profile, run, port)
	defer func() {
		if handle != nil {
			handle.close()
		}
	}()
	logger.New("Browser").Info("浏览器实际进程跟踪", logger.F("event", "browser.process_handoff"), logger.F("critical", true), logger.F("profile_id", profile), logger.F("run_id", run), logger.F("process", proof))
	misses := 0
	degraded := false
	lastResources := time.Time{}
	lastDiscover := time.Now()
	for a.browserProcessMonitorIsCurrent(profile, monitor) {
		if handle != nil {
			proof = handle.snapshot()
		}
		if proof["state"] == "exited" {
			a.browserMgr.Mutex.Lock()
			current := a.browserMgr.Profiles[profile]
			if current == nil || current.LastStartAt != run || !a.browserProcessMonitorIsCurrentLocked(profile, monitor) {
				a.browserMgr.Mutex.Unlock()
				return
			}
			expected := current.RuntimeState == "stopping"
			monitor.mu.Lock()
			intent, method, kernel := monitor.stopIntent, monitor.stopMethod, monitor.kernelBinary
			monitor.mu.Unlock()
			code := fmt.Sprint(proof["exit_code"])
			abnormal := !expected && code != "0" && code != "<nil>"
			if abnormal {
				a.markProfileFailedLocked(profile, current, fmt.Errorf("浏览器进程异常退出：%s", proof["exit_code_hex"]))
			} else {
				a.markProfileStoppedLocked(profile, current)
			}
			snapshot := copyBrowserProfileSnapshot(current)
			a.browserMgr.Mutex.Unlock()
			fields := map[string]interface{}{"kernel_binary": kernel, "process": proof, "expected": expected, "stop_intent": intent, "stop_method": method}
			log := logger.New("Browser")
			args := []logger.Field{logger.F("event", "browser.process_exited"), logger.F("critical", true), logger.F("profile_id", profile), logger.F("run_id", run), logger.F("process", proof), logger.F("expected", expected), logger.F("stop_intent", intent), logger.F("stop_method", method)}
			if abnormal {
				log.Error("浏览器实际进程异常退出", args...)
				a.recordDiagnosticIncident(profile, run, "browser.process_exited", fields)
			} else {
				log.Info("已确认浏览器实际进程退出", args...)
			}
			a.closeProfilePageSession(profile)
			if abnormal {
				a.emitRuntimeEvent("browser:instance:crashed", browserInstanceEventPayload(snapshot, false))
			} else {
				a.emitRuntimeEvent("browser:instance:stopped", profile)
			}
			return
		}
		ready := canConnectDebugPort(port, 250*time.Millisecond)
		if ready {
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
				a.recordDiagnosticIncident(profile, run, "browser.debug_disconnected", map[string]interface{}{"debug_port": port, "process": proof})
			}
		}
		if (handle == nil || proof["state"] == "unknown") && time.Since(lastDiscover) >= 10*time.Second {
			if handle != nil {
				handle.close()
			}
			handle, proof = a.attachBrowserEvidence(profile, run, port)
			lastDiscover = time.Now()
		}
		if time.Since(lastResources) >= 30*time.Second {
			logger.New("BrowserDiagnostics").Info("浏览器资源快照", logger.F("event", "browser.resources"), logger.F("profile_id", profile), logger.F("run_id", run), logger.F("process", proof), logger.F("system", systemResourceEvidence()))
			lastResources = time.Now()
		}
		timer := time.NewTimer(500 * time.Millisecond)
		if a.ctx != nil {
			select {
			case <-timer.C:
			case <-a.ctx.Done():
				timer.Stop()
				return
			}
		} else {
			<-timer.C
		}
	}
}
