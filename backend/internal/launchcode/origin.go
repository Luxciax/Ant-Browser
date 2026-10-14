package launchcode

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

func loopbackHTTPHost(raw string) bool {
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// A loopback peer address alone does not prevent a hostile website from using
// DNS rebinding. Native clients have no Origin; browser clients must be local.
func localOriginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHTTPHost(r.Host) {
			writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "forbidden: non-local Host"})
			return
		}
		if raw := r.Header.Get("Origin"); raw != "" {
			origin, err := url.Parse(raw)
			if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || !loopbackHTTPHost(origin.Host) || origin.User != nil {
				writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "forbidden: non-local Origin"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
