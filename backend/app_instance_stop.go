package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"time"
)

func (a *App) BrowserInstanceStop(profileId string) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()
	return a.browserInstanceStopWithRuntimeLock(profileId)
}

// browserInstanceStopWithRuntimeLock executes one stop transaction while the
// caller owns the per-profile lifecycle lock.
func (a *App) browserInstanceStopWithRuntimeLock(profileId string) (*BrowserProfile, error) {
	log := logger.New("Browser")
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		a.browserMgr.Mutex.Unlock()
		return nil, fmt.Errorf("profile not found")
	}
	if browser.NormalizeProfileRuntimeState(profile) == browser.RuntimeStopped && !profile.Running && profile.Pid <= 0 && profile.DebugPort <= 0 {
		snapshot := copyBrowserProfileSnapshot(profile)
		a.browserMgr.Mutex.Unlock()
		a.closeProfilePageSession(profileId)
		return snapshot, nil
	}

	cmd := a.browserMgr.BrowserProcesses[profileId]
	debugPort := profile.DebugPort
	pid := profile.Pid
	if pid <= 0 && cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	var stopEvidence *processEvidenceHandle
	if pid > 0 {
		stopEvidence = openProcessEvidence(pid)
		defer func() {
			proof := stopEvidence.snapshot()
			logger.New("Browser").Info("实例关闭结果证据", logger.F("event", "browser.stop_result"), logger.F("critical", true), logger.F("profile_id", profileId), logger.F("run_id", profile.LastStartAt), logger.F("process", proof))
			stopEvidence.close()
		}()
	}
	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if monitor := a.browserProcessMonitors[profileId]; monitor != nil {
		monitor.mu.Lock()
		if monitor.stopIntent == "" {
			monitor.stopIntent = "user_stop"
		}
		monitor.stopMethod = "cdp_then_process_tree"
		monitor.mu.Unlock()
	}
	log.Info("实例关闭请求", logger.F("event", "browser.stop_requested"), logger.F("critical", true), logger.F("profile_id", profileId), logger.F("run_id", profile.LastStartAt), logger.F("pid", pid), logger.F("debug_port", debugPort))
	a.markProfileStoppingLocked(profile)
	stoppingSnapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	a.closeProfilePageSession(profileId)
	a.emitBrowserInstanceUpdated(stoppingSnapshot)

	stopRun := profile.LastStartAt
	closedByCDP := tryCloseBrowserViaCDP(debugPort, 5*time.Second)
	if closedByCDP && waitBrowserRuntimeStopped(pid, debugPort, 5*time.Second) {
		a.browserMgr.Mutex.Lock()
		current, stillExists := a.browserMgr.Profiles[profileId]
		if stillExists {
			a.markProfileStoppedLocked(profileId, current)
			profile = current
		}
		snapshot := copyBrowserProfileSnapshot(profile)
		a.browserMgr.Mutex.Unlock()
		log.Info("实例停止", logger.F("profile_id", profileId), logger.F("method", "cdp"), logger.F("debug_port", debugPort))
		a.emitRuntimeEvent("browser:instance:stopped", profileId)
		return snapshot, nil
	}

	log.Info("关闭升级为进程清理", logger.F("event", "browser.stop_escalated"), logger.F("critical", true), logger.F("profile_id", profileId), logger.F("run_id", stopRun), logger.F("method", "managed_process_and_user_data_dir"), logger.F("cdp_close_succeeded", closedByCDP))
	if cmd != nil && cmd.Process != nil {
		if err := a.stopBrowserProcess(cmd); err != nil {
			log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("error", err))
			return a.markProfileStopFailed(profileId, err), err
		}
	}
	if terminated, err := terminateBrowserProcessesByUserDataDir(userDataDir, 5*time.Second); err != nil {
		log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("user_data_dir", userDataDir), logger.F("error", err))
		return a.markProfileStopFailed(profileId, err), err
	} else if terminated {
		log.Info("已清理实例用户数据目录关联的浏览器进程", logger.F("profile_id", profileId), logger.F("user_data_dir", userDataDir))
	}

	if !waitBrowserRuntimeStopped(pid, debugPort, 5*time.Second) {
		err := fmt.Errorf("实例停止失败：浏览器进程或调试端口在强制清理后仍处于活动状态（pid=%d, debugPort=%d）", pid, debugPort)
		log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("debug_port", debugPort), logger.F("reason", err.Error()))
		return a.markProfileStopFailed(profileId, err), err
	}

	a.browserMgr.Mutex.Lock()
	current, stillExists := a.browserMgr.Profiles[profileId]
	if stillExists {
		a.markProfileStoppedLocked(profileId, current)
		profile = current
	}
	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	log.Info("实例停止", logger.F("profile_id", profileId))
	a.emitRuntimeEvent("browser:instance:stopped", profileId)
	return snapshot, nil
}

func (a *App) BrowserInstanceRestart(profileId string) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()

	a.browserMgr.Mutex.Lock()
	if monitor := a.browserProcessMonitors[profileId]; monitor != nil {
		monitor.mu.Lock()
		monitor.stopIntent = "user_restart"
		monitor.mu.Unlock()
	}
	a.browserMgr.Mutex.Unlock()
	if _, err := a.browserInstanceStopWithRuntimeLock(profileId); err != nil {
		return nil, err
	}
	return a.browserInstanceStartWithRuntimeLock(profileId, nil, nil, false, false, false, "", "")
}
