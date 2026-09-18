package launchcode

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLaunchServerStartRejectsEmptyAPIKeyWhenAuthEnabled(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: true})
	err := server.Start()
	if err == nil || !strings.Contains(err.Error(), "api_key is empty") {
		t.Fatalf("Start() error = %v, want empty api key rejection", err)
	}
}

func TestLaunchServerAuthProtectsAPIAndCDP(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: true, APIKey: "secret"})
	handler := NewTestHandler(server)
	for _, path := range []string{"/api/health", "/json/version", "/devtools/browser/test"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s without auth status = %d, want %d", path, resp.Code, http.StatusUnauthorized)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set(DefaultAPIKeyHeader, "secret")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("health with auth status = %d, want %d", resp.Code, http.StatusOK)
	}
}
