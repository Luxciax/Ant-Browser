package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/config"
)

const defaultMCPPath = "/mcp"

// MCPClientEndpoint describes how a local helper process can reach the MCP
// endpoint hosted by the main Ant Browser process.
type MCPClientEndpoint struct {
	URL        string
	AuthHeader string
	AuthValue  string
}

func ResolveMCPClientEndpoint(cfg *Config) MCPClientEndpoint {
	port := config.DefaultLaunchServerPort
	if cfg != nil && cfg.LaunchServer.Port > 0 {
		port = cfg.LaunchServer.Port
	}

	endpoint := MCPClientEndpoint{
		URL: fmt.Sprintf("http://127.0.0.1:%d%s", port, defaultMCPPath),
	}
	if cfg == nil || !cfg.LaunchServer.Auth.Enabled {
		return endpoint
	}

	credential := strings.TrimSpace(cfg.LaunchServer.Auth.APIKey)
	if credential == "" {
		return endpoint
	}
	header := strings.TrimSpace(cfg.LaunchServer.Auth.Header)
	if header == "" {
		header = config.DefaultLaunchServerAPIKeyHeader
	}
	endpoint.AuthHeader = header
	endpoint.AuthValue = credential
	return endpoint
}
