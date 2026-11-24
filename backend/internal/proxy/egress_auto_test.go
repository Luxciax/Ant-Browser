package proxy

import "testing"

func TestPhysicalInterfaceScoreRejectsVirtualAdapters(t *testing.T) {
	for _, name := range []string{"v2rayN TUN", "Wintun Userspace Tunnel", "WireGuard Tunnel", "VMware Network Adapter", "Tailscale", "vEthernet (Default Switch)", "vEthernet (WSL)"} {
		if score := physicalInterfaceScore(name); score > -100 {
			t.Fatalf("virtual adapter %q score = %d, want rejected", name, score)
		}
	}
	if score := physicalInterfaceScore("Ethernet"); score <= 0 {
		t.Fatalf("Ethernet score = %d, want physical candidate", score)
	}
	if score := physicalInterfaceScore("Wi-Fi"); score <= 0 {
		t.Fatalf("Wi-Fi score = %d, want physical candidate", score)
	}
}

func TestCloneOutboundsForEgressIsDeep(t *testing.T) {
	base := []interface{}{
		map[string]interface{}{
			"type": "hysteria2",
			"tls": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	cloned := cloneOutboundsForEgress(base)
	out := cloned[0].(map[string]interface{})
	out["bind_interface"] = "Ethernet"
	out["tls"].(map[string]interface{})["server_name"] = "example.com"

	original := base[0].(map[string]interface{})
	if _, ok := original["bind_interface"]; ok {
		t.Fatalf("physical egress mutation leaked into base outbound: %#v", original)
	}
	if _, ok := original["tls"].(map[string]interface{})["server_name"]; ok {
		t.Fatalf("nested mutation leaked into base outbound: %#v", original)
	}
}

func TestApplySingBoxPhysicalEgressOnlyBindsNetworkOrigin(t *testing.T) {
	outbounds := []interface{}{
		map[string]interface{}{"type": "hysteria2", "tag": "first-hop"},
		map[string]interface{}{"type": "socks", "tag": "second-hop", "detour": "first-hop"},
	}
	applySingBoxPhysicalEgress(outbounds, "Ethernet")
	first := outbounds[0].(map[string]interface{})
	second := outbounds[1].(map[string]interface{})
	if first["bind_interface"] != "Ethernet" {
		t.Fatalf("first hop bind_interface = %v, want Ethernet", first["bind_interface"])
	}
	if _, ok := second["bind_interface"]; ok {
		t.Fatalf("detoured second hop must not bind interface independently: %#v", second)
	}
}

func TestApplyXrayPhysicalEgressOnlyBindsNetworkOrigin(t *testing.T) {
	outbounds := []interface{}{
		map[string]interface{}{"protocol": "vless", "tag": "first-hop"},
		map[string]interface{}{
			"protocol": "socks",
			"tag":      "second-hop",
			"proxySettings": map[string]interface{}{"tag": "first-hop"},
		},
	}
	applyXrayPhysicalEgress(outbounds, "Ethernet")

	first := outbounds[0].(map[string]interface{})
	stream := first["streamSettings"].(map[string]interface{})
	sockopt := stream["sockopt"].(map[string]interface{})
	if sockopt["interface"] != "Ethernet" {
		t.Fatalf("first hop interface = %v, want Ethernet", sockopt["interface"])
	}
	second := outbounds[1].(map[string]interface{})
	if _, ok := second["streamSettings"]; ok {
		t.Fatalf("proxied second hop must not bind interface independently: %#v", second)
	}
}
