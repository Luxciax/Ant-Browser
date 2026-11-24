package launchcode

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// localhostMiddleware 只允许 127.0.0.1 访问
func (s *LaunchServer) localhostMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || host != "127.0.0.1" {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok":    false,
				"error": "forbidden: only localhost is allowed",
			})
			return
		}
		// RemoteAddr alone is insufficient: a hostile hostname can resolve to
		// loopback. Only the listener's literal local authorities are valid.
		requestHost, requestPort, ok := localControlAuthority(r.Host)
		if !ok || requestPort != s.Port() {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok": false, "error": "forbidden: invalid local control Host",
			})
			return
		}
		origins := r.Header.Values("Origin")
		if len(origins) > 0 {
			valid := false
			if len(origins) == 1 {
				origin, err := url.Parse(origins[0])
				if err == nil && origin.Scheme == "http" && origin.User == nil &&
					origin.Path == "" && origin.RawQuery == "" && origin.Fragment == "" {
					originHost, originPort, originOK := localControlAuthority(origin.Host)
					valid = originOK && originHost == requestHost && originPort == requestPort
				}
			}
			if !valid {
				writeJSON(w, http.StatusForbidden, map[string]interface{}{
					"ok": false, "error": "forbidden: cross-origin local control request",
				})
				return
			}
		}
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok": false, "error": "forbidden: cross-site local control request",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func localControlAuthority(authority string) (string, int, bool) {
	host, rawPort, err := net.SplitHostPort(authority)
	if err != nil {
		return "", 0, false
	}
	host = strings.ToLower(host)
	if host != "localhost" && host != "127.0.0.1" {
		return "", 0, false
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}
	return host, port, true
}

// handleCDPProxy 将统一端口上的非 /api 请求转发到当前活动实例的 CDP 端口。
func (s *LaunchServer) handleCDPProxy(w http.ResponseWriter, r *http.Request) {
	debugPort, profileID, profileName := s.activeTarget()
	if debugPort <= 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":          false,
			"error":       "no active browser debug target",
			"profileId":   profileID,
			"profileName": profileName,
		})
		return
	}

	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", debugPort))
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid cdp target: %v", err), http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	direct := proxy.Director
	auth := s.apiAuthConfig()
	proxy.Director = func(out *http.Request) {
		direct(out)
		// The gateway credential is not a browser/CDP credential.
		if auth.Requested() {
			out.Header.Del(auth.Header)
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
		http.Error(w, fmt.Sprintf("cdp proxy error: %v", proxyErr), http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeLaunchRequestParams(params LaunchRequestParams) LaunchRequestParams {
	params.LaunchArgs = normalizeStringSlice(params.LaunchArgs)
	params.StartURLs = normalizeStringSlice(params.StartURLs)
	params.ProxyId = strings.TrimSpace(params.ProxyId)
	params.ProxyConfig = strings.TrimSpace(params.ProxyConfig)
	return params
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
