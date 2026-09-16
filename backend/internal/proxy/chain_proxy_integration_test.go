package proxy

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func encodeReferencedChainForTest(t *testing.T, frontProxyID string, landing chainSocks5Hop, preferredKernel string) string {
	t.Helper()
	payload, err := json.Marshal(chainProxyConfig{
		Version:       2,
		FrontProxyID:  frontProxyID,
		Landing:       landing,
		PreferredCore: preferredKernel,
	})
	if err != nil {
		t.Fatal(err)
	}
	return chainProxyPrefix + url.QueryEscape(string(payload))
}

func TestParseChainProxyConfigNormalizesReferencedChain(t *testing.T) {
	src := encodeReferencedChainForTest(t, " front-a ", chainSocks5Hop{
		Protocol: "SOCKS5",
		Server:   "landing.example.com",
		Port:     1080,
	}, "singbox")

	cfg, err := ParseChainProxyConfig(src)
	if err != nil {
		t.Fatalf("ParseChainProxyConfig returned error: %v", err)
	}
	if cfg.Version != 2 || cfg.FrontProxyID != "front-a" || cfg.Landing.Protocol != "socks5" {
		t.Fatalf("normalized config = %#v", cfg)
	}
	if cfg.PreferredCore != ProxyKernelSingBox {
		t.Fatalf("preferred kernel = %q, want %q", cfg.PreferredCore, ProxyKernelSingBox)
	}
}

func TestResolveChainFrontRejectsSelfNestedAndMissing(t *testing.T) {
	landing := chainSocks5Hop{Protocol: "socks5", Server: "landing.example.com", Port: 1080}
	cfg, err := ParseChainProxyConfig(encodeReferencedChainForTest(t, "front-a", landing, ""))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := resolveChainFront(cfg, nil, "front-a"); err == nil || !strings.Contains(err.Error(), "自身") {
		t.Fatalf("self reference error = %v", err)
	}

	nested := encodeReferencedChainForTest(t, "front-b", landing, "")
	if _, err := resolveChainFront(cfg, []config.BrowserProxy{{ProxyId: "front-a", ProxyConfig: nested}}, "chain-a"); err == nil || !strings.Contains(err.Error(), "链式代理") {
		t.Fatalf("nested chain error = %v", err)
	}

	if _, err := resolveChainFront(cfg, []config.BrowserProxy{{ProxyId: "other", ProxyConfig: "socks5://127.0.0.1:1080"}}, "chain-a"); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("missing front error = %v", err)
	}
}

func TestReferencedChainKernelFollowsFrontProtocol(t *testing.T) {
	landing := chainSocks5Hop{Protocol: "socks5", Server: "landing.example.com", Port: 1080}
	src := encodeReferencedChainForTest(t, "front-a", landing, "")

	tests := []struct {
		name      string
		front     string
		wantFirst string
	}{
		{name: "vless uses xray", front: "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&sni=example.com#front", wantFirst: ProxyKernelXray},
		{name: "hy2 uses sing-box", front: "hysteria2://password@example.com:443?sni=example.com", wantFirst: ProxyKernelSingBox},
		{name: "anytls uses sing-box", front: "anytls://password@example.com:443?sni=example.com", wantFirst: ProxyKernelSingBox},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxies := []config.BrowserProxy{{ProxyId: "chain-a", ProxyConfig: src}, {ProxyId: "front-a", ProxyConfig: tt.front}}
			resolution, err := ResolveProxyKernel(src, proxies, "chain-a", "")
			if err != nil {
				t.Fatalf("ResolveProxyKernel returned error: %v", err)
			}
			if resolution.Protocol != "chain+proxy" || len(resolution.SupportedKernels) == 0 || resolution.SupportedKernels[0] != tt.wantFirst {
				t.Fatalf("resolution = %#v, want first kernel %q", resolution, tt.wantFirst)
			}
		})
	}
}

func TestBuildXrayReferencedChainOutbounds(t *testing.T) {
	landing := chainSocks5Hop{Protocol: "socks5", Server: "landing.example.com", Port: 1080, Username: "u", Password: "p"}
	src := encodeReferencedChainForTest(t, "front-a", landing, "")
	cfg, err := ParseChainProxyConfig(src)
	if err != nil {
		t.Fatal(err)
	}
	proxies := []config.BrowserProxy{{ProxyId: "front-a", ProxyConfig: "socks5://127.0.0.1:2080"}}

	outbounds, routes, err := buildXrayReferencedChainOutbounds(cfg, proxies, "chain-a")
	if err != nil {
		t.Fatalf("buildXrayReferencedChainOutbounds returned error: %v", err)
	}
	if len(outbounds) != 2 || len(routes) != 1 {
		t.Fatalf("outbounds/routes = %d/%d", len(outbounds), len(routes))
	}
	front := outbounds[0].(map[string]interface{})
	landingOutbound := outbounds[1].(map[string]interface{})
	if front["tag"] != "front-hop" || landingOutbound["tag"] != "landing-hop" {
		t.Fatalf("unexpected tags: front=%v landing=%v", front["tag"], landingOutbound["tag"])
	}
}

func TestChainProxyRuntimeKeySourceChangesWithReferencedNode(t *testing.T) {
	landing := chainSocks5Hop{Protocol: "socks5", Server: "landing.example.com", Port: 1080}
	src := encodeReferencedChainForTest(t, "front-a", landing, "")
	first := chainProxyRuntimeKeySource(src, []config.BrowserProxy{{ProxyId: "front-a", ProxyConfig: "socks5://127.0.0.1:2080"}}, "chain-a")
	second := chainProxyRuntimeKeySource(src, []config.BrowserProxy{{ProxyId: "front-a", ProxyConfig: "socks5://127.0.0.1:3080"}}, "chain-a")
	if first == second {
		t.Fatalf("runtime key source did not change after referenced node update: %q", first)
	}
}
