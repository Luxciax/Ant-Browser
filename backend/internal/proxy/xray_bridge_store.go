package proxy

import (
	"ant-chrome/backend/internal/logger"
	"errors"
	"fmt"
	"time"
)

func (m *XrayManager) discardBridge(key string, bridge *XrayBridge) {
	if bridge == nil {
		return
	}
	m.mu.Lock()
	bridge.Stopping = true
	if current, ok := m.Bridges[key]; ok && current == bridge {
		delete(m.Bridges, key)
	}
	m.mu.Unlock()
	m.stopBridgeProcess(bridge)
}

func (m *XrayManager) tryReuseBridge(key string, pin bool) (string, bool) {
	var stale *XrayBridge

	m.mu.Lock()
	if bridge, ok := m.Bridges[key]; ok && bridge != nil {
		// A speed/health check must not replace a browser's pinned endpoint.
		// An exited pinned process is recovered by watchBridge on the same port.
		if bridge.RefCount > 0 && !bridge.Stopping {
			if pin {
				bridge.RefCount++
			}
			bridge.LastUsedAt = time.Now()
			m.mu.Unlock()
			return fmt.Sprintf("socks5://127.0.0.1:%d", bridge.Port), true
		}
		alive := bridge.Running && bridge.Cmd != nil && bridge.Cmd.Process != nil && bridge.Cmd.ProcessState == nil
		if alive && waitSocks5Ready("127.0.0.1", bridge.Port, 800*time.Millisecond) == nil {
			if pin {
				bridge.RefCount++
			}
			bridge.LastUsedAt = time.Now()
			socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", bridge.Port)
			m.mu.Unlock()
			return socksURL, true
		}

		bridge.Stopping = true
		stale = bridge
		delete(m.Bridges, key)
	}
	m.mu.Unlock()

	if stale != nil {
		m.stopBridgeProcess(stale)
	}
	return "", false
}

func (m *XrayManager) registerBridge(key string, bridge *XrayBridge, pin bool) (string, bool) {
	var duplicate *XrayBridge

	m.mu.Lock()
	if existing, ok := m.Bridges[key]; ok && existing != nil {
		if existing == bridge {
			m.mu.Unlock()
			return "", false
		}

		alive := existing.Running && existing.Cmd != nil && existing.Cmd.Process != nil && existing.Cmd.ProcessState == nil
		if (!existing.Stopping && existing.RefCount > 0 && !existing.Restarting) || (alive && waitSocks5Ready("127.0.0.1", existing.Port, 800*time.Millisecond) == nil) {
			if pin {
				existing.RefCount++
			}
			existing.LastUsedAt = time.Now()
			duplicate = bridge
			socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", existing.Port)
			m.mu.Unlock()
			if duplicate != nil {
				duplicate.Stopping = true
				m.stopBridgeProcess(duplicate)
			}
			return socksURL, true
		}

		transferredRefCount := 0
		if existing.Restarting && existing.RefCount > 0 {
			transferredRefCount = existing.RefCount
		}
		existing.Stopping = true
		delete(m.Bridges, key)
		duplicate = existing
		if transferredRefCount > 0 {
			bridge.RefCount = transferredRefCount
		}
	}

	if pin {
		bridge.RefCount++
	}
	bridge.LastUsedAt = time.Now()
	m.Bridges[key] = bridge
	m.mu.Unlock()

	if duplicate != nil {
		m.stopBridgeProcess(duplicate)
	}
	return "", false
}

func (m *XrayManager) watchBridge(bridge *XrayBridge, key string) {
	if bridge == nil || bridge.Cmd == nil {
		return
	}
	exitErr := bridge.waitExit()

	var shouldRestart bool
	var refCount int
	m.mu.Lock()
	if current, ok := m.Bridges[key]; ok && current == bridge {
		refCount = bridge.RefCount
		if !bridge.Stopping && refCount > 0 && !bridge.Restarting {
			bridge.Restarting = true
			shouldRestart = true
		} else {
			delete(m.Bridges, key)
		}
	}
	bridge.Running = false
	stopping := bridge.Stopping
	m.mu.Unlock()

	if !stopping {
		if m.OnBridgeEvidence != nil {
			m.OnBridgeEvidence(key, "proxy.bridge_exited", map[string]interface{}{"process": bridgeExitEvidence(bridge.Cmd), "stderr_tail": bridgeStderrTail(bridge.Cmd, "xray-stderr.log"), "port": bridge.Port, "recovery_requested": shouldRestart})
		}
		logger.New("Xray").Warn("xray 桥接进程退出", logger.F("key", key), logger.F("port", bridge.Port), logger.F("pid", bridge.Pid), logger.F("exit_error", exitErr), logger.F("stderr_tail", bridgeStderrTail(bridge.Cmd, "xray-stderr.log")), logger.F("process_evidence", bridgeExitEvidence(bridge.Cmd)))
	}
	if shouldRestart {
		log := logger.New("Xray")
		if err := m.restartPinnedBridge(log, key, bridge); err == nil {
			if m.OnBridgeEvidence != nil {
				m.OnBridgeEvidence(key, "proxy.bridge_recovered", map[string]interface{}{"port": bridge.Port})
			}
			return
		} else if errors.Is(err, errXrayBridgeRestartNotNeeded) {
			return
		} else {
			log.Error("xray 桥接同端口恢复失败", logger.F("key", key), logger.F("port", bridge.Port), logger.F("error", err.Error()))
			m.mu.Lock()
			if current, ok := m.Bridges[key]; ok && current == bridge {
				delete(m.Bridges, key)
			}
			m.mu.Unlock()
		}
	}

	if !stopping && m.OnBridgeDied != nil {
		m.OnBridgeDied(key, fmt.Errorf("xray 桥接进程意外退出"))
	}
}
