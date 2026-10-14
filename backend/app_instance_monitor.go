package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"time"
)

func (a *App) waitBrowserProcess(profileId string, monitor *browserProcessMonitor) {
	proof := a.observeManagedProcessHealth(profileId, monitor)
	if !monitor.HasExited() && (!a.browserProcessMonitorIsCurrent(profileId, monitor) || (a.ctx != nil && a.ctx.Err() != nil)) {
		return
	}
	err := monitor.Wait()
	if !a.browserProcessMonitorIsCurrent(profileId, monitor) {
		return
	}

	log := logger.New("Browser")
	result := monitor.Result()
	log.Info("浏览器启动进程退出", logger.F("event", "browser.launcher_exited"), logger.F("critical", true), logger.F("profile_id", profileId), logger.F("run_id", monitor.runID), logger.F("exit_code", result.ExitCode), logger.F("error", err), logger.F("stderr_tail", result.StderrTail), logger.F("process", proof))
	debugPort := 0
	profileName := profileId
	shouldMonitorDetached := false

	a.browserMgr.Mutex.Lock()
	if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
		a.browserMgr.Mutex.Unlock()
		return
	}
	profile, exists := a.browserMgr.Profiles[profileId]
	wasRunning := exists && profile.Running
	if exists {
		profileName = profile.ProfileName
		debugPort = profile.DebugPort
	}
	a.browserMgr.Mutex.Unlock()

	if wasRunning && debugPort > 0 {
		snapshot, changed := a.waitForBrowserDebugReadyForMonitor(profileId, debugPort, browserLauncherDetachGraceWindow, monitor)
		if warningSnapshot, warningChanged := a.finalizeDeferredStartTargetsForMonitor(profileId, debugPort, monitor); warningSnapshot != nil {
			snapshot = warningSnapshot
			changed = changed || warningChanged
		}
		if snapshot != nil && changed {
			if !a.browserProcessMonitorIsCurrent(profileId, monitor) {
				return
			}
			log.Info("浏览器启动器进程退出后，调试接口延迟就绪",
				logger.F("profile_id", profileId),
				logger.F("debug_port", debugPort),
			)
			a.emitBrowserInstanceUpdated(snapshot)
		}

		a.browserMgr.Mutex.Lock()
		profile, exists = a.browserMgr.Profiles[profileId]
		if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
			a.browserMgr.Mutex.Unlock()
			return
		}
		if exists && profile.Running && profile.DebugPort == debugPort && profile.DebugReady && canConnectDebugPort(debugPort, 250*time.Millisecond) {
			delete(a.browserMgr.BrowserProcesses, profileId)
			// Resolve the real browser identity in waitDetachedBrowser. Do not
			// discard PID and replace process evidence with a port-only heuristic.
			shouldMonitorDetached = true
		}
		a.browserMgr.Mutex.Unlock()
		if !shouldMonitorDetached {
			handle, proof := a.attachBrowserEvidence(profileId, monitor.runID, debugPort)
			if handle != nil {
				handle.close()
			}
			shouldMonitorDetached = proof["state"] == "alive" || proof["reason"] == "process_discovery_failed"
		}
		if shouldMonitorDetached {
			log.Info("浏览器启动器已退出，开始跟踪实际浏览器进程",
				logger.F("profile_id", profileId),
				logger.F("profile_name", profileName),
				logger.F("debug_port", debugPort),
				logger.F("run_id", monitor.runID),
			)
			a.waitDetachedBrowser(profileId, debugPort, monitor)
			return
		}
	}

	a.browserMgr.Mutex.Lock()
	if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
		a.browserMgr.Mutex.Unlock()
		return
	}
	profile, exists = a.browserMgr.Profiles[profileId]
	wasRunning = exists && profile.Running
	expected := exists && profile.RuntimeState == "stopping"
	if exists {
		profileName = profile.ProfileName
		if wasRunning && err != nil && !expected {
			a.markProfileFailedLocked(profileId, profile, fmt.Errorf("实例运行异常退出：%w", err))
		} else {
			a.markProfileStoppedLocked(profileId, profile)
		}
	}
	a.browserMgr.Mutex.Unlock()
	a.closeProfilePageSession(profileId)

	if wasRunning && err != nil && !expected {
		a.recordDiagnosticIncident(profileId, monitor.runID, "browser.process_exited", map[string]interface{}{"process": proof, "pid": monitor.initialPID(), "exit_code": result.ExitCode, "kernel_binary": monitor.kernelBinary, "stderr_tail": result.StderrTail, "error": fmt.Sprint(err)})
		log.Error("浏览器进程异常退出", logger.F("event", "browser.process_exited"), logger.F("exit_code", result.ExitCode), logger.F("profile_id", profileId), logger.F("profile_name", profileName), logger.F("run_id", monitor.runID), logger.F("error", err), logger.F("stderr_tail", monitor.Result().StderrTail))
		a.emitRuntimeEvent("browser:instance:crashed", map[string]interface{}{
			"profileId":    profileId,
			"profileName":  profileName,
			"runtimeState": "failed",
			"error":        err.Error(),
		})
	} else {
		a.emitRuntimeEvent("browser:instance:stopped", profileId)
	}
}

func (a *App) waitDetachedBrowser(profileId string, debugPort int, monitor *browserProcessMonitor) {
	a.monitorDetachedEvidence(profileId, debugPort, monitor)
}
