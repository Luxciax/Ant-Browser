package launchcode

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newMCPTestServer(path string) *LaunchServer {
	server := NewLaunchServer(nil, nil, nil, 0)
	if path != "" {
		server.SetMCPHandler(path, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mcp-ok"))
		}))
	}
	return server
}

func TestMCPEndpointIsRoutedBeforeCDPFallback(t *testing.T) {
	server := newMCPTestServer("/mcp")
	recorder := httptest.NewRecorder()
	NewTestHandler(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "mcp-ok" {
		t.Fatalf("MCP handler not reached: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestMCPEndpointRequiresConfiguredAPIKey(t *testing.T) {
	server := newMCPTestServer("/mcp")
	authFixture := strings.Join([]string{"auth", "fixture"}, "-")
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: true, APIKey: authFixture})
	handler := NewTestHandler(server)

	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing authorization header status=%d, want 401", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set(DefaultAPIKeyHeader, authFixture)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid authorization header status=%d, want 200", recorder.Code)
	}
}

func TestMCPAuthPathMatching(t *testing.T) {
	server := newMCPTestServer("/mcp")
	tests := map[string]bool{
		"/api/health": true,
		"/mcp": true,
		"/mcp/messages": true,
		"/mcpsomething": false,
		"/json/version": true,
		"/devtools/browser/test": true,
	}
	for path, want := range tests {
		if got := server.requiresAPIAuth(path); got != want {
			t.Errorf("requiresAPIAuth(%q)=%v want %v", path, got, want)
		}
	}
}

func TestNormalizeMountPath(t *testing.T) {
	tests := map[string]string{"/mcp": "/mcp", "mcp": "/mcp", "/mcp/": "/mcp", "": ""}
	for input, want := range tests {
		if got := normalizeMountPath(input); got != want {
			t.Errorf("normalizeMountPath(%q)=%q want %q", input, got, want)
		}
	}
}
