package proxy

import (
	"ant-chrome/backend/internal/logger"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errSingBoxBridgeRestartNotNeeded = errors.New("sing-box 桥接已无须恢复")

func cloneStringInterfaceMap(items map[string]interface{}) map[string]interface{} {
	if len(items) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(items))
	for key, value := range items {
		cloned[key] = value
	}
	return cloned
}

func (m *SingBoxManager) restartBridgeOnSamePort(log *logger.Logger, key string, bridge *SingBoxBridge, refCount int) error {
	if bridge == nil {
		return fmt.Errorf("sing-box 桥接不存在")
	}
	unlockLaunch := m.lockLaunchForKey(key)
	defer unlockLaunch()
	m.mu.Lock()
	current := m.Bridges[key]
	// watchBridge owns this transition and already set Restarting. Rejecting
	// that flag here would skip every recovery of a pinned bridge.
	if current != bridge || bridge.Stopping {
		m.mu.Unlock()
		return errSingBoxBridgeRestartNotNeeded
	}
	if len(bridge.Outbounds) == 0 && len(bridge.Outbound) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("sing-box 桥接缺少重启上下文")
	}
	if bridge.RefCount <= 0 {
		delete(m.Bridges, key)
		m.mu.Unlock()
		return errSingBoxBridgeRestartNotNeeded
	}
	bridge.Restarting = true
	m.mu.Unlock()

	binaryPath, err := m.resolveBinary()
	if err != nil {
		return err
	}
	log.Warn("sing-box 桥接进程退出，尝试同端口恢复",
		logger.F("key", key[:8]),
		logger.F("port", bridge.Port),
	)
	outbounds := cloneInterfaceSlice(bridge.Outbounds)
	if len(outbounds) == 0 && len(bridge.Outbound) > 0 {
		outbounds = []interface{}{cloneStringInterfaceMap(bridge.Outbound)}
	}
	routeOutbound := strings.TrimSpace(bridge.RouteOutbound)
	if routeOutbound == "" {
		routeOutbound = "proxy-out"
	}
	restarted, err := m.launchBridgeOnPort(log, key, binaryPath, outbounds, routeOutbound, bridge.Port, 1)
	if err != nil {
		return err
	}
	restarted.RestartCount = bridge.RestartCount + 1
	restarted.LastUsedAt = time.Now()
	m.mu.Lock()
	if current := m.Bridges[key]; current != bridge || bridge.Stopping || bridge.RefCount <= 0 {
		if current == bridge {
			delete(m.Bridges, key)
		}
		m.mu.Unlock()
		restarted.Stopping = true
		m.stopBridgeProcess(restarted)
		return errSingBoxBridgeRestartNotNeeded
	}
	// Profiles may have stopped while the process was being launched.
	restarted.RefCount = bridge.RefCount
	m.Bridges[key] = restarted
	m.mu.Unlock()
	log.Info("sing-box 桥接已同端口恢复",
		logger.F("key", key[:8]),
		logger.F("port", restarted.Port),
		logger.F("pid", restarted.Pid),
	)
	go m.watchBridge(restarted, key)
	return nil
}
