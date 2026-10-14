package proxy

import "testing"

func TestPinnedBridgesKeepEndpointDuringTransientUnavailability(t *testing.T) {
	t.Run("xray", func(t *testing.T) {
		b := &XrayBridge{Port: 32123, RefCount: 2, Restarting: true}
		m := &XrayManager{Bridges: map[string]*XrayBridge{"node": b}}
		url, ok := m.tryReuseBridge("node", false)
		if !ok || url != "socks5://127.0.0.1:32123" || m.Bridges["node"] != b || b.Stopping || b.RefCount != 2 {
			t.Fatalf("active endpoint replaced: %#v %q", b, url)
		}
		m.tryReuseBridge("node", true)
		if b.RefCount != 3 {
			t.Fatal("new browser reference lost during recovery")
		}
	})
	t.Run("singbox", func(t *testing.T) {
		b := &SingBoxBridge{Port: 32124, RefCount: 2, Restarting: true}
		m := &SingBoxManager{Bridges: map[string]*SingBoxBridge{"node": b}}
		url, ok := m.tryReuseBridge("node", false)
		if !ok || url != "socks5://127.0.0.1:32124" || m.Bridges["node"] != b || b.Stopping || b.RefCount != 2 {
			t.Fatalf("active endpoint replaced: %#v %q", b, url)
		}
	})
	t.Run("mihomo", func(t *testing.T) {
		b := &MihomoNodeBridge{Port: 32125, RefCount: 2, Restarting: true}
		m := &ClashManager{NodeBridges: map[string]*MihomoNodeBridge{"node": b}}
		url, ok := m.tryReuseMihomoNodeBridge("node", false)
		if !ok || url != "http://127.0.0.1:32125" || m.NodeBridges["node"] != b || b.RefCount != 2 {
			t.Fatalf("active endpoint replaced: %#v %q", b, url)
		}
	})
}

func TestXrayRegistrationPreservesConcurrentReferencesDuringRecovery(t *testing.T) {
	old := &XrayBridge{RefCount: 3, Restarting: true}
	m := &XrayManager{Bridges: map[string]*XrayBridge{"node": old}}
	next := &XrayBridge{Port: 32123}
	m.registerBridge("node", next, true)
	if next.RefCount != 4 {
		t.Fatalf("reference count=%d, want 4", next.RefCount)
	}
}
