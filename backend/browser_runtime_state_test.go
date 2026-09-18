package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestBrowserInstanceEventPayloadIncludesRuntimeState(t *testing.T) {
	profile := &browser.Profile{ProfileId: "profile-1", RuntimeState: browser.RuntimeStarting}
	payload := browserInstanceEventPayload(profile, false)
	if got := payload["runtimeState"]; got != browser.RuntimeStarting {
		t.Fatalf("runtimeState = %v, want %s", got, browser.RuntimeStarting)
	}
}

func TestWaitBrowserDebugPortStableContextHonorsDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = waitBrowserDebugPortStableContext(ctx, port, t.TempDir(), 5*time.Second, time.Second, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("context deadline was not honored promptly: %s", elapsed)
	}
}

func TestProfileRuntimeOperationLockIsPerProfile(t *testing.T) {
	app := NewApp(t.TempDir())
	unlockA := app.lockProfileRuntimeOperation("profile-a")
	defer unlockA()

	acquiredB := make(chan struct{})
	go func() {
		unlockB := app.lockProfileRuntimeOperation("profile-b")
		close(acquiredB)
		unlockB()
	}()

	select {
	case <-acquiredB:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("profile-b lifecycle operation was blocked by profile-a lock")
	}
}

func TestMaintenanceLockBlocksProfileLifecycleAndSnapshotMutation(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "stop",
			run: func() error {
				_, err := app.BrowserInstanceStop("missing-profile")
				return err
			},
		},
		{
			name: "snapshot-delete",
			run: func() error {
				return app.BrowserSnapshotDelete("missing-profile", "missing-snapshot")
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			app.maintenanceMu.Lock()
			started := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				close(started)
				done <- testCase.run()
			}()
			<-started

			select {
			case err := <-done:
				app.maintenanceMu.Unlock()
				t.Fatalf("operation escaped maintenance lock: %v", err)
			case <-time.After(80 * time.Millisecond):
			}

			app.maintenanceMu.Unlock()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("operation did not resume after maintenance lock release")
			}
		})
	}
}

func TestSnapshotManagedBrowserProfileDeepCopiesSlices(t *testing.T) {
	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{
		ProfileId:        "profile-copy",
		FingerprintArgs: []string{"--fingerprint=1"},
		LaunchArgs:      []string{"--flag"},
		LastLaunchArgs:  []string{"--last"},
		Tags:            []string{"tag-a"},
		Keywords:        []string{"keyword-a"},
	}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.browserMgr = manager

	snapshot := app.snapshotManagedBrowserProfile(profile)
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	snapshot.FingerprintArgs[0] = "--fingerprint=2"
	snapshot.LaunchArgs[0] = "--changed"
	snapshot.LastLaunchArgs[0] = "--changed-last"
	snapshot.Tags[0] = "tag-b"
	snapshot.Keywords[0] = "keyword-b"

	if profile.FingerprintArgs[0] != "--fingerprint=1" ||
		profile.LaunchArgs[0] != "--flag" ||
		profile.LastLaunchArgs[0] != "--last" ||
		profile.Tags[0] != "tag-a" ||
		profile.Keywords[0] != "keyword-a" {
		t.Fatalf("snapshot mutated managed profile: %+v", profile)
	}
}

func TestWaitBrowserProcessMarksUnexpectedExitFailed(t *testing.T) {
	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{
		ProfileId:    "profile-crash",
		ProfileName:  "profile-crash",
		RuntimeState: browser.RuntimeRunning,
		Running:      true,
	}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.browserMgr = manager

	monitor := newDetachedBrowserProcessMonitor()
	monitor.mu.Lock()
	monitor.result.Err = errors.New("simulated crash")
	monitor.mu.Unlock()
	app.browserProcessMonitors[profile.ProfileId] = monitor

	app.waitBrowserProcess(profile.ProfileId, monitor)

	manager.Mutex.Lock()
	defer manager.Mutex.Unlock()
	if profile.RuntimeState != browser.RuntimeFailed {
		t.Fatalf("runtime state = %s, want %s", profile.RuntimeState, browser.RuntimeFailed)
	}
	if profile.Running || profile.Pid != 0 || profile.DebugPort != 0 || profile.DebugReady {
		t.Fatalf("failed profile retained live runtime fields: %+v", profile)
	}
	if profile.LastError == "" {
		t.Fatal("failed profile did not record crash error")
	}
}

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
