package backend

import (
	"context"
	"net"
	"testing"
)

type staticRemoteResolver map[string][]net.IPAddr

func (r staticRemoteResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return r[host], nil
}

func TestResolveSafeRemoteIPRejectsPrivateAndMixedDNSResults(t *testing.T) {
	resolver := staticRemoteResolver{
		"private.example": {{IP: net.ParseIP("127.0.0.1")}},
		"mixed.example": {
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("10.0.0.8")},
		},
	}
	for _, host := range []string{"private.example", "mixed.example"} {
		if _, err := resolveSafeRemoteIP(context.Background(), host, resolver); err == nil {
			t.Fatalf("resolveSafeRemoteIP(%q) accepted private DNS result", host)
		}
	}
}

func TestResolveSafeRemoteIPAcceptsPublicDNSResults(t *testing.T) {
	resolver := staticRemoteResolver{
		"public.example": {
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("2606:2800:220:1:248:1893:25c8:1946")},
		},
	}
	ip, err := resolveSafeRemoteIP(context.Background(), "public.example", resolver)
	if err != nil {
		t.Fatalf("resolveSafeRemoteIP returned error: %v", err)
	}
	if got := ip.String(); got != "93.184.216.34" {
		t.Fatalf("selected IP = %s, want first public result", got)
	}
}

func TestResolveSafeRemoteIPRejectsMetadataAndPrivateLiterals(t *testing.T) {
	for _, host := range []string{"169.254.169.254", "10.0.0.1", "127.0.0.1", "::1", "fc00::1", "fe80::1"} {
		if _, err := resolveSafeRemoteIP(context.Background(), host, staticRemoteResolver{}); err == nil {
			t.Fatalf("resolveSafeRemoteIP(%q) accepted forbidden literal", host)
		}
	}
}
