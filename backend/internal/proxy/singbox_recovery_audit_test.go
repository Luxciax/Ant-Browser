package proxy

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSingBoxWatcherDoesNotSuppressRecoveryOrFailureNotification(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^$")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	const key = "audit-singbox-key"
	bridge := &SingBoxBridge{Cmd: cmd, NodeKey: key, Running: true, RefCount: 1}
	// Missing recovery configuration forces a deterministic recovery failure,
	// after the watcher has marked the bridge as restarting.
	notified := false
	m := &SingBoxManager{Bridges: map[string]*SingBoxBridge{key: bridge}, OnBridgeDied: func(got string, err error) {
		if got != key || err == nil {
			t.Errorf("unexpected notification: %s %v", got, err)
		}
		notified = true
	}}
	m.watchBridge(bridge, key)
	if !notified {
		t.Error("watcher silently skipped recovery and failure notification")
	}
	if m.Bridges[key] != nil {
		t.Error("dead bridge remains registered as restarting")
	}
}

func TestSingBoxRealProcessRecoversPinnedBridge(t *testing.T) {
	binary := os.Getenv("ANT_BROWSER_TEST_SINGBOX")
	if binary == "" {
		t.Skip("set ANT_BROWSER_TEST_SINGBOX to run real process recovery")
	}
	cfg := config.DefaultConfig()
	cfg.Browser.SingBoxBinaryPath = binary
	m := NewSingBoxManager(cfg, t.TempDir())
	t.Cleanup(m.StopAll)
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	const key = "audit-real-singbox"
	bridge, err := m.launchBridgeOnPort(logger.New("Audit"), key, binary, []interface{}{map[string]interface{}{"type": "direct", "tag": "proxy-out"}}, "proxy-out", port, 1)
	if err != nil {
		t.Fatal(err)
	}
	m.registerBridge(key, bridge, true)
	go m.watchBridge(bridge, key)
	if err := bridge.Cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		recovered := m.Bridges[key]
		ready := recovered != nil && recovered != bridge && recovered.Running && recovered.Port == port && recovered.RefCount == 1
		m.mu.Unlock()
		if ready {
			if err := waitSocks5Ready("127.0.0.1", port, time.Second); err != nil {
				t.Fatal(err)
			}
			m.ReleaseBridge(key)
			m.mu.Lock()
			refs := recovered.RefCount
			m.mu.Unlock()
			if refs != 0 {
				t.Fatalf("released bridge refs = %d", refs)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("pinned bridge did not recover on its original SOCKS port")
}
