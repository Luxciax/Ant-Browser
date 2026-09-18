package backend

import (
	"fmt"
	"net"
	"strconv"

	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/mcpserver"
)

func (a *App) SaveLaunchServerSettings(port int) (map[string]interface{}, error) {
	if a.config == nil {
		return nil, fmt.Errorf("launch server config is not initialized")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("端口必须在 1-65535 之间")
	}

	currentPort := 0
	if a.launchServer != nil {
		currentPort = a.launchServer.Port()
	}
	if currentPort != port {
		if err := ensureLaunchServerPortAvailable(port); err != nil {
			return nil, err
		}
	}

	previousServer := a.launchServer
	previousPort := a.config.LaunchServer.Port
	if currentPort != port {
		if err := a.restartLaunchServer(port); err != nil {
			return nil, err
		}
	}

	a.config.LaunchServer.Port = port
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.LaunchServer.Port = previousPort
		if currentPort != port {
			if restoreErr := a.restoreLaunchServer(previousServer); restoreErr != nil {
				return nil, fmt.Errorf("保存 LaunchServer 配置失败: %w；恢复旧 LaunchServer 失败: %v", err, restoreErr)
			}
		}
		return nil, err
	}

	return a.GetLaunchServerInfo(), nil
}

func ensureLaunchServerPortAvailable(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("端口 %d 不可用: %w", port, err)
	}
	return listener.Close()
}

func (a *App) restartLaunchServer(port int) error {
	log := logger.New("LaunchServer")
	previousServer := a.launchServer

	server := launchcode.NewLaunchServer(a.launchCodeSvc, a, a.browserMgr, port)
	server.SetProxyProvider(newAppProxyProvider(a))
	server.SetPageDriver(newAppPageDriver(a))
	server.SetAPIAuthConfig(launchcode.APIAuthConfig{
		Enabled: a.config.LaunchServer.Auth.Enabled,
		APIKey:  a.config.LaunchServer.Auth.APIKey,
		Header:  a.config.LaunchServer.Auth.Header,
	})
	mcpService := mcpserver.New(server, a.appVersion())
	server.SetMCPHandler(mcpserver.DefaultPath, mcpService.Handler(mcpserver.Options{}))
	if err := server.Start(); err != nil {
		return fmt.Errorf("启动 LaunchServer 失败: %w", err)
	}
	if previousServer != nil {
		if err := previousServer.Stop(); err != nil {
			if stopNewErr := server.Stop(); stopNewErr != nil {
				return fmt.Errorf("停止旧 LaunchServer 失败: %w；停止候选 LaunchServer 失败: %v", err, stopNewErr)
			}
			return fmt.Errorf("停止旧 LaunchServer 失败: %w", err)
		}
	}
	a.launchServer = server
	log.Info("LaunchServer 端口已切换", logger.F("port", server.Port()))
	return nil
}

func (a *App) restoreLaunchServer(previousServer *launchcode.LaunchServer) error {
	currentServer := a.launchServer
	if currentServer == previousServer {
		return nil
	}
	if currentServer != nil {
		if err := currentServer.Stop(); err != nil {
			return fmt.Errorf("停止新 LaunchServer 失败: %w", err)
		}
	}
	if previousServer == nil {
		a.launchServer = nil
		return nil
	}
	if err := previousServer.Start(); err != nil {
		a.launchServer = nil
		return fmt.Errorf("重新启动旧 LaunchServer 失败: %w", err)
	}
	a.launchServer = previousServer
	return nil
}
