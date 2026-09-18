package backend

import (
	"net/http"
	"testing"
	"time"
)

func TestAutomationHTTPClientUsesDedicatedBoundedTransport(t *testing.T) {
	client := newAutomationHTTPClient()
	if client == nil || client == http.DefaultClient {
		t.Fatal("automation client must not use http.DefaultClient")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if transport.DialContext == nil {
		t.Fatal("automation client is missing bounded dialer")
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("TLSHandshakeTimeout = %s, want 10s", transport.TLSHandshakeTimeout)
	}
	if transport.IdleConnTimeout <= 0 {
		t.Fatal("automation client must bound idle connections")
	}
	if client.Timeout != 0 {
		t.Fatalf("client timeout = %s, total deadline must remain request-context controlled", client.Timeout)
	}
}
