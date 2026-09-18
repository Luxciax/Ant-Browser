package backend

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

func TestRestartLaunchServerRestoresPreviousServerWhenNewPortFails(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}

	previousServer := launchcode.NewLaunchServer(nil, nil, nil, 0)
	if err := previousServer.Start(); err != nil {
		t.Fatalf("previous LaunchServer.Start returned error: %v", err)
	}
	defer previousServer.Stop()
	app.launchServer = previousServer

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen returned error: %v", err)
	}
	defer occupied.Close()
	occupiedPort := occupied.Addr().(*net.TCPAddr).Port

	if err := app.restartLaunchServer(occupiedPort); err == nil {
		t.Fatal("restartLaunchServer should fail when the requested port is occupied")
	}
	if app.launchServer != previousServer {
		t.Fatal("restartLaunchServer did not restore the previous LaunchServer pointer")
	}
	if previousServer.Port() <= 0 {
		t.Fatal("restored LaunchServer does not have a bound port")
	}

	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(previousServer.Port())))
	if err != nil {
		t.Fatalf("restored LaunchServer is not accepting connections: %v", err)
	}
	_ = conn.Close()
}

func TestSaveLaunchServerSettingsRestoresRuntimeWhenConfigCommitFails(t *testing.T) {
	root := t.TempDir()
	blockedRoot := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp(blockedRoot)
	app.config = config.DefaultConfig()
	previousServer := launchcode.NewLaunchServer(nil, nil, nil, 0)
	if err := previousServer.Start(); err != nil {
		t.Fatalf("previous LaunchServer.Start returned error: %v", err)
	}
	defer func() {
		if app.launchServer != nil {
			_ = app.launchServer.Stop()
		}
	}()
	app.launchServer = previousServer
	previousPort := previousServer.Port()
	app.config.LaunchServer.Port = previousPort

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	newPort := probe.Addr().(*net.TCPAddr).Port
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := app.SaveLaunchServerSettings(newPort); err == nil {
		t.Fatal("SaveLaunchServerSettings should fail when config path cannot be created")
	}
	if app.config.LaunchServer.Port != previousPort {
		t.Fatalf("config port = %d, want rollback to %d", app.config.LaunchServer.Port, previousPort)
	}
	if app.launchServer != previousServer {
		t.Fatal("previous LaunchServer instance was not restored")
	}
	if app.launchServer.Port() != previousPort {
		t.Fatalf("runtime port = %d, want %d", app.launchServer.Port(), previousPort)
	}
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(previousPort)))
	if err != nil {
		t.Fatalf("restored LaunchServer is not accepting connections: %v", err)
	}
	_ = conn.Close()
}
