package backend

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyImportFormats(t *testing.T) {
	uri := "vless://00000000-0000-4000-8000-000000000001@example.com:443?security=tls&type=ws&path=%2Fsocket&host=cdn.example.com#测试+节点"
	cases := map[string]string{
		"node URI":         uri,
		"BOM URI":          "\uFEFF" + uri,
		"raw subscription": "# comment\n" + uri + "\ntrojan://test-only@example.com:443?sni=example.com#two",
		"base64":           base64.StdEncoding.EncodeToString([]byte(uri)),
		"base64 urlsafe":   base64.RawURLEncoding.EncodeToString([]byte(uri)),
		"base64 wrapped":   base64.StdEncoding.EncodeToString([]byte(uri))[:20] + "\n" + base64.StdEncoding.EncodeToString([]byte(uri))[20:],
		"clash yaml":       "proxies:\n  - {name: test, type: http, server: example.com, port: 8080}\ndns: {enable: true, nameserver: [1.1.1.1]}",
		"clash json":       `{"proxies":[{"name":"test","type":"http","server":"example.com","port":8080}]}`,
		"single node":      `{"type":"http","server":"example.com","port":8080}`,
		"node array":       `[{"type":"http","server":"example.com","port":8080}]`,
		"sing-box":         `{"outbounds":[{"type":"direct","tag":"direct"},{"type":"selector","tag":"select","outbounds":["test"]},{"type":"vless","tag":"test","server":"example.com","server_port":443,"uuid":"00000000-0000-4000-8000-000000000001","tls":{"enabled":true,"server_name":"example.com"},"transport":{"type":"ws","path":"/socket","headers":{"Host":"cdn.example.com"}}}]}`,
		"single sing-box":  `{"type":"hysteria2","tag":"test","server":"example.com","server_port":443,"password":"test-only","tls":{"enabled":true}}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := (&App{}).BrowserProxyParseImport(input)
			if err != nil {
				t.Fatal(err)
			}
			count := result["proxyCount"].(int)
			if count < 1 {
				t.Fatal("no nodes")
			}
			_, payload, err := normalizeClashSubscriptionContent([]byte(result["content"].(string)))
			if err != nil || clashProxyCount(payload) != count {
				t.Fatal("normalization not idempotent", err)
			}
			if name == "node URI" && !strings.Contains(result["content"].(string), "测试+节点") {
				t.Fatal("fragment changed")
			}
			if name == "clash yaml" && result["dnsServers"] == "" {
				t.Fatal("DNS lost")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(input)) }))
			defer server.Close()
			_, remote, err := fetchClashSubscriptionWithUserAgent(context.Background(), server.Client(), server.URL, "format-test")
			if err != nil || clashProxyCount(remote) != count {
				t.Fatal("URL path differs", err)
			}
		})
	}
}

func TestProxyImportRejectsPartialOrLossyInput(t *testing.T) {
	cases := []string{
		"trojan://test-only@example.com:443#ok\nssr://unsupported",
		`{"proxies":[{"type":"http","server":"example.com","port":8080},"bad"]}`,
		`{"type":"vless","server":"example.com","server_port":443,"uuid":"test-only","detour":"another"}`,
		`{"type":"vmess","server":"example.com","server_port":443,"uuid":"test-only","transport":{"type":"http"}}`,
		"vless://test-only@example.com:443?type=xhttp&security=tls",
		"trojan://example.com:443",
		"vless://test-only@example.com:70000",
		"vless://test-only@example.com:443?path=%zz",
		"vless://test-only@example.com:443?security=reality",
		"vless://test-only@example.com:443?type=ws&path=%2F&extra=secret-test",
		strings.Repeat("x", maxClashSubscriptionBytes+1),
	}
	for _, input := range cases {
		if _, _, err := normalizeClashSubscriptionContent([]byte(input)); err == nil {
			t.Fatal("bad or lossy input accepted")
		} else if strings.Contains(err.Error(), "test-only") || strings.Contains(err.Error(), "secret-test") {
			t.Fatal("credentials leaked")
		}
	}
}
