package launchcode

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalOriginRejectsDNSRebindingAndExternalBrowser(t *testing.T) {
	for _, tc := range []struct {
		host, origin string
		status       int
	}{
		{"127.0.0.1:19876", "", 204}, {"localhost:19876", "http://localhost:34115", 204}, {"[::1]:19876", "http://[::1]:34115", 204},
		{"attacker.example:19876", "", 403}, {"127.0.0.1:19876", "https://attacker.example", 403}, {"localhost:19876", "null", 403}, {"127.0.0.1:19876", "http://localhost@evil.example", 403},
	} {
		h := localOriginMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
		r := httptest.NewRequest("POST", "http://127.0.0.1:19876/api/launch", nil)
		r.Host = tc.host
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s %s: got %d want %d", tc.host, tc.origin, w.Code, tc.status)
		}
	}
}
