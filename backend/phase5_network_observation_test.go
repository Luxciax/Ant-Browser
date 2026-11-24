package backend

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestSanitizeObservationURLStripsSensitiveParts(t *testing.T) {
	parsed, publicURL, err := sanitizeObservationURL("https://user:pass@example.com/path?q=secret#fragment")
	if err == nil {
		t.Fatal("URLs containing credentials must be rejected")
	}
	if parsed != nil || publicURL != "" {
		t.Fatalf("rejected URL should not return sanitized output: %#v %q", parsed, publicURL)
	}

	parsed, publicURL, err = sanitizeObservationURL("https://example.com/path?q=secret#fragment")
	if err != nil {
		t.Fatalf("unexpected sanitize error: %v", err)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		t.Fatalf("query/fragment were not stripped: %q", publicURL)
	}
	if publicURL != "https://example.com/path" {
		t.Fatalf("unexpected public URL: %q", publicURL)
	}
}

func TestDisallowedObservationIPRejectsInternalRanges(t *testing.T) {
	cases := []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.1.1", "::1", "fc00::1"}
	for _, raw := range cases {
		if !disallowedObservationIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s to be rejected", raw)
		}
	}
	if disallowedObservationIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("public address should not be rejected")
	}
}

func TestValidateObservationTargetRejectsLiteralPrivateAddress(t *testing.T) {
	err := validateObservationTarget(context.Background(), "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected private target rejection, got %v", err)
	}
}
