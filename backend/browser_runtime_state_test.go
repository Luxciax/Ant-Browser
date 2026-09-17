package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"net"
	"testing"
	"time"
)

func TestMarkProfileRunningRegistersDetachedBrowserMonitor(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for debug port: %v", err)
	}
	defer listener.Close()

	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{ProfileId: "profile-1"}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.browserMgr = manager
	app.profileWindowMarkers[profile.ProfileId] = &profileWindowMarker{
		code: "A",
		stop: make(chan struct{}),
	}

	debugPort := listener.Addr().(*net.TCPAddr).Port
	manager.Mutex.Lock()
	app.markProfileRunningLocked(profile.ProfileId, profile, nil, 1234, debugPort, true, "")
	monitor := app.browserProcessMonitors[profile.ProfileId]
	manager.Mutex.Unlock()

	if monitor == nil {
		t.Fatal("detached browser monitor was not registered")
	}

	manager.Mutex.Lock()
	app.markProfileStoppedLocked(profile.ProfileId, profile)
	manager.Mutex.Unlock()
}

func TestDetachedBrowserMonitorMarksProfileStoppedAfterDebugPortCloses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for debug port: %v", err)
	}

	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{ProfileId: "profile-1", ProfileName: "profile-1"}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.browserMgr = manager

	debugPort := listener.Addr().(*net.TCPAddr).Port
	manager.Mutex.Lock()
	app.markProfileRunningLocked(profile.ProfileId, profile, nil, 1234, debugPort, true, "")
	manager.Mutex.Unlock()

	if err := listener.Close(); err != nil {
		t.Fatalf("close debug listener: %v", err)
	}

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		manager.Mutex.Lock()
		running := profile.Running
		pid := profile.Pid
		port := profile.DebugPort
		manager.Mutex.Unlock()
		if !running {
			if pid != 0 || port != 0 {
				t.Fatalf("stopped profile retained runtime state: pid=%d debugPort=%d", pid, port)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatal("profile remained running after detached browser debug port closed")
}
