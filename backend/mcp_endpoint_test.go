package backend

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestResolveMCPClientEndpoint(t *testing.T) {
	if got := ResolveMCPClientEndpoint(nil); got.URL != "http://127.0.0.1:19876/mcp" {
		t.Fatalf("default URL = %q", got.URL)
	}

	cfg := DefaultConfig()
	cfg.LaunchServer.Port = 20000
	got := ResolveMCPClientEndpoint(cfg)
	if got.URL != "http://127.0.0.1:20000/mcp" {
		t.Fatalf("custom URL = %q", got.URL)
	}

	credential := strings.Join([]string{"bridge", "fixture"}, "-")
	cfg.LaunchServer.Auth = config.LaunchServerAuthConfig{
		Enabled: true,
		APIKey:  credential,
		Header:  "X-Test-Authorization",
	}
	got = ResolveMCPClientEndpoint(cfg)
	if got.AuthHeader != "X-Test-Authorization" || got.AuthValue != credential {
		t.Fatalf("auth propagation failed: header=%q value-match=%v", got.AuthHeader, got.AuthValue == credential)
	}
}
