//go:build windows

package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestEvidenceChild(t *testing.T) {
	if os.Getenv("ANT_EVIDENCE_CHILD") != "1" {
		return
	}
	_, _ = io.ReadAll(os.Stdin)
	os.Exit(23)
}

func TestRetainedProcessEvidenceAndLiveBrowserWithoutDebugPort(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestEvidenceChild$")
	cmd.Env = append(os.Environ(), "ANT_EVIDENCE_CHILD=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	handle := openProcessEvidence(cmd.Process.Pid)
	defer handle.close()
	before := handle.snapshot()
	if before["state"] != "alive" || before["created_at_ns"] == int64(0) || before["exit_code"] != nil {
		t.Fatal(before)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	old := findBrowserUserDataProcesses
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) {
		return []browserUserDataProcess{{PID: cmd.Process.Pid, DebugPort: port, CommandLine: "test-browser"}}, nil
	}
	defer func() { findBrowserUserDataProcesses = old }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := browser.NewManager(config.DefaultConfig(), t.TempDir())
	p := &browser.Profile{ProfileId: "live-evidence", Running: true, RuntimeState: browser.RuntimeRunning, LastStartAt: "native-run", Pid: cmd.Process.Pid, DebugPort: port, DebugReady: true}
	manager.Profiles[p.ProfileId] = p
	app := NewApp("")
	app.ctx = ctx
	app.browserMgr = manager
	app.runtimeStopped = true
	monitor := newDetachedBrowserProcessMonitor()
	app.browserProcessMonitors[p.ProfileId] = monitor
	done := make(chan struct{})
	go func() { app.monitorDetachedEvidence(p.ProfileId, port, monitor); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	degraded := false
	for time.Now().Before(deadline) {
		manager.Mutex.Lock()
		running, ready := p.Running, p.DebugReady
		manager.Mutex.Unlock()
		if !running {
			t.Fatal("live process falsely stopped after CDP loss")
		}
		if !ready {
			degraded = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !degraded {
		t.Fatal("no debug degradation")
	}
	stdin.Close()
	cmd.Wait()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not observe actual exit")
	}
	after := handle.snapshot()
	if after["state"] != "exited" || after["exit_code"] != uint32(23) || after["created_at_ns"] != before["created_at_ns"] {
		t.Fatal(after)
	}
	manager.Mutex.Lock()
	defer manager.Mutex.Unlock()
	if p.Running || p.RuntimeState != browser.RuntimeFailed {
		t.Fatalf("actual exit not applied: %+v", p)
	}
}

func TestMainProcessSelectionRejectsRendererAndOtherDebugPort(t *testing.T) {
	main, ok := selectBrowserMainProcess([]browserUserDataProcess{{PID: 1, DebugPort: 5000, CommandLine: "chrome --type=renderer"}, {PID: 2, DebugPort: 6000, CommandLine: "chrome"}, {PID: 3, DebugPort: 5000, CommandLine: "chrome"}}, 5000)
	if !ok || main.PID != 3 {
		t.Fatal(main, ok)
	}
}
