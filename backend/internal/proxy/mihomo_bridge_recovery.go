package proxy

import (
	"ant-chrome/backend/internal/logger"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

func (m *ClashManager) restartMihomoNodeBridge(old *MihomoNodeBridge) error {
	unlock := m.lockLaunchForKey(old.NodeKey)
	defer unlock()
	m.mu.Lock()
	needed := m.NodeBridges[old.NodeKey] == old && !old.Stopping && old.RefCount > 0
	m.mu.Unlock()
	if !needed {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Reuse the exact configuration and both ports; never reselect an exit node.
	cmd := exec.Command(old.Cmd.Path, "-f", old.ConfigPath, "-d", filepath.Dir(old.ConfigPath))
	hideWindow(cmd)
	cmd.Dir = filepath.Dir(old.ConfigPath)
	stderr, err := logger.NewProcessLog(filepath.Join(cmd.Dir, "mihomo-stderr.log"))
	if err != nil {
		return err
	}
	cmd.Stderr = stderr
	cmd.Stdout = stderr
	err = cmd.Start()
	_ = stderr.Close()
	if err != nil {
		return err
	}
	next := &MihomoNodeBridge{NodeKey: old.NodeKey, Port: old.Port, ControllerPort: old.ControllerPort, Cmd: cmd, Pid: cmd.Process.Pid, ConfigPath: old.ConfigPath, Running: true, LastUsedAt: time.Now(), ExitDone: make(chan struct{}), RestartCount: old.RestartCount + 1}
	readyErr := waitTCPPortReadyContext(ctx, "127.0.0.1", next.Port, 10*time.Second)
	if readyErr == nil {
		readyErr = waitTCPPortReadyContext(ctx, "127.0.0.1", next.ControllerPort, 10*time.Second)
	}
	m.mu.Lock()
	needed = m.NodeBridges[old.NodeKey] == old && !old.Stopping && old.RefCount > 0
	if needed && readyErr == nil {
		next.RefCount = old.RefCount
		m.NodeBridges[old.NodeKey] = next
	}
	m.mu.Unlock()
	if !needed || readyErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if !needed {
			return nil
		}
		return fmt.Errorf("同端口恢复未就绪: %w", readyErr)
	}
	logger.New("Mihomo").Info("mihomo 桥接已同端口恢复", logger.F("key", next.NodeKey), logger.F("port", next.Port), logger.F("pid", next.Pid), logger.F("ref_count", next.RefCount), logger.F("restart_count", next.RestartCount))
	m.watchMihomoNodeBridge(next)
	return nil
}
