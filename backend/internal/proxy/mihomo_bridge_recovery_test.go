package proxy

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// A real child process exposing both ports, without installing/downloading a
// proxy engine or touching an existing browser. Recovery uses production argv.
func init() {
	path := os.Getenv("ANT_MIHOMO_RECOVERY_TEST_CONFIG")
	if path == "" {
		return
	}
	for i, arg := range os.Args {
		if arg != "-f" || i+1 >= len(os.Args) || os.Args[i+1] != path {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			os.Exit(2)
		}
		var ports []int
		if json.Unmarshal(data, &ports) != nil {
			os.Exit(3)
		}
		for _, port := range ports {
			listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
			if err != nil {
				os.Exit(4)
			}
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					conn.Close()
				}
			}()
		}
		select {}
	}
}

func TestMihomoPinnedRecoveryPreservesPortsRefsAndStops(t *testing.T) {
	freePort := func() int {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return port
	}
	ports := []int{freePort(), freePort()}
	path := filepath.Join(t.TempDir(), "test-config.json")
	data, _ := json.Marshal(ports)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANT_MIHOMO_RECOVERY_TEST_CONFIG", path)
	cmd := exec.Command(os.Args[0], "-f", path)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	m := &ClashManager{NodeBridges: map[string]*MihomoNodeBridge{}}
	old := &MihomoNodeBridge{NodeKey: "recovery-test", Port: ports[0], ControllerPort: ports[1], Cmd: cmd, Pid: cmd.Process.Pid, ConfigPath: path, Running: true, RefCount: 2, ExitDone: make(chan struct{})}
	m.registerMihomoNodeBridge(old.NodeKey, old)
	m.watchMihomoNodeBridge(old)
	t.Cleanup(func() { m.StopAll(); _ = cmd.Process.Kill() })
	for _, port := range ports {
		if err := waitTCPPortReady("127.0.0.1", port, 3*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	var next *MihomoNodeBridge
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		current := m.NodeBridges[old.NodeKey]
		if current != nil && current != old {
			next = current
		}
		m.mu.Unlock()
		if next != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if next == nil {
		t.Fatal("pinned bridge did not recover")
	}
	if next.Port != old.Port || next.ControllerPort != old.ControllerPort || next.RefCount != 2 || next.RestartCount != 1 {
		t.Fatalf("recovered bridge: %+v", next)
	}
	if next.Pid == old.Pid {
		t.Fatal("process was not replaced")
	}
	m.ReleaseNodeBridge(old.NodeKey)
	m.mu.Lock()
	refs := next.RefCount
	m.mu.Unlock()
	if refs != 1 {
		t.Fatalf("refcount=%d", refs)
	}
	m.ReleaseNodeBridge(old.NodeKey)
	select {
	case <-next.ExitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("last release did not stop child")
	}
	m.mu.Lock()
	remaining := len(m.NodeBridges)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatal("stopped bridge resurrected")
	}
}
