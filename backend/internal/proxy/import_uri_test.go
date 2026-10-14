package proxy

import (
	"encoding/base64"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestImportedURIReachesSelectedStackParser(t *testing.T) {
	vmess := base64.RawStdEncoding.EncodeToString([]byte(`{"v":"2","ps":"vmess","add":"example.com","port":443,"id":"00000000-0000-4000-8000-000000000001","aid":0,"net":"ws","type":"none","path":"/ws","host":"cdn.example.com","tls":"tls","sni":"example.com"}`))
	ss := base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:test-only"))
	cases := []string{
		"http://user:test-only@example.com:8080#http",
		"https://user:test-only@example.com:443#https",
		"socks5://user:test-only@[2001:db8::1]:1080#socks",
		"vmess://" + vmess,
		"vless://00000000-0000-4000-8000-000000000001@example.com:443?security=reality&pbk=test-only&sid=ab&fp=chrome",
		"trojan://test-only@example.com:443?sni=example.com&type=grpc&serviceName=test",
		"ss://" + ss + "@example.com:8388#ss",
		"ss://" + base64.RawStdEncoding.EncodeToString([]byte("aes-128-gcm:test-only@example.com:8388")) + "#ss",
		"hy2://test-only@example.com:443?sni=example.com&obfs=salamander&obfs-password=test-only",
		"hy2://test-only@example.com?sni=example.com",
		"hysteria://test-only@example.com:443?upmbps=10&downmbps=20",
		"tuic://00000000-0000-4000-8000-000000000001:test-only@example.com:443?congestion_control=bbr",
		"anytls://test-only@example.com:443?sni=example.com",
	}
	for _, uri := range cases {
		t.Run(strings.Split(uri, ":")[0], func(t *testing.T) {
			node, err := ImportShareURI(uri, 0)
			if err != nil {
				t.Fatal(err)
			}
			data, err := yaml.Marshal(node)
			if err != nil {
				t.Fatal(err)
			}
			switch getMapString(node, "type") {
			case "hysteria", "hysteria2", "tuic", "anytls":
				if _, err := BuildSingBoxOutbound(string(data)); err != nil {
					t.Fatal(err)
				}
			default:
				_, standard, err := parseClashNode(string(data))
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(uri, "https://") && !strings.HasPrefix(standard, "https://") {
					t.Fatal("HTTPS downgraded")
				}
				if strings.HasPrefix(uri, "socks5://") && !strings.Contains(standard, "[2001:db8::1]:1080") {
					t.Fatal("IPv6 corrupted")
				}
			}
			if _, err := parseClashYAMLToMapping(string(data)); err != nil {
				t.Fatal("Mihomo mapping", err)
			}
		})
	}
}

func TestSingBoxImportPreservesTLSAndRejectsPolicy(t *testing.T) {
	tuic, err := ImportSingBoxOutbound(map[string]interface{}{"type": "tuic", "server": "example.com", "server_port": 443, "uuid": "test-only", "password": "test-only", "udp_relay_mode": "native", "zero_rtt_handshake": true, "heartbeat": "10s"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := buildSingBoxTUICFromClash(tuic)
	if err != nil || out["udp_relay_mode"] != "native" || out["zero_rtt_handshake"] != true || out["heartbeat"] != "10000ms" {
		t.Fatal("TUIC options lost", err)
	}
	node, err := ImportSingBoxOutbound(map[string]interface{}{"type": "vless", "server": "example.com", "server_port": 443, "uuid": "test-only", "tls": map[string]interface{}{"enabled": true, "server_name": "example.com", "utls": map[string]interface{}{"enabled": true, "fingerprint": "chrome"}, "reality": map[string]interface{}{"enabled": true, "public_key": "test-only", "short_id": "ab"}}})
	if err != nil {
		t.Fatal(err)
	}
	outbound, _, err := buildOutboundFromClashVless(node)
	if err != nil {
		t.Fatal(err)
	}
	stream := toStringMap(outbound["streamSettings"])
	if getMapString(stream, "security") != "reality" {
		t.Fatal("Reality lost")
	}
	for _, field := range []string{"detour", "multiplex", "domain_resolver", "bind_interface"} {
		_, err := ImportSingBoxOutbound(map[string]interface{}{"type": "http", "server": "example.com", "server_port": 8080, field: "test-only"})
		if err == nil {
			t.Fatal("unsupported policy accepted", field)
		}
	}
}
