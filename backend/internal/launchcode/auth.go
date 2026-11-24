package launchcode

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

const DefaultAPIKeyHeader = "X-Ant-Api-Key"

// APIAuthConfig 定义 LaunchServer 对 /api/* 请求的可选认证配置。
type APIAuthConfig struct {
	Enabled bool
	APIKey  string
	Header  string
	Scopes  []string
}

const (
	ScopeAll = "*"
	ScopeEnvironmentRead = "environment-read"
	ScopePageRead = "page-read"
	ScopePageAction = "page-action"
	ScopeScript = "script"
	ScopeEnvironmentAdmin = "environment-admin"
	ScopeBackupCredentialExport = "backup-credential-export"
)

func normalizeAPIAuthConfig(cfg APIAuthConfig) APIAuthConfig {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Header = strings.TrimSpace(cfg.Header)
	cfg.Scopes = normalizeAPIScopes(cfg.Scopes)
	if cfg.Header == "" {
		cfg.Header = DefaultAPIKeyHeader
	}
	return cfg
}

func (cfg APIAuthConfig) Requested() bool {
	return cfg.Enabled
}

func (cfg APIAuthConfig) Configured() bool {
	return cfg.APIKey != ""
}

func (cfg APIAuthConfig) Active() bool {
	return cfg.Requested() && cfg.Configured()
}

func normalizeAPIScopes(values []string) []string {
	if len(values) == 0 { return []string{ScopeAll} }
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" { continue }
		if _, ok := seen[value]; ok { continue }
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 { return []string{ScopeAll} }
	return out
}

func scopeAllowed(scopes []string, required string) bool {
	required = strings.ToLower(strings.TrimSpace(required))
	if required == "" { return true }
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == ScopeAll || scope == required { return true }
	}
	return false
}

func (s *LaunchServer) APIAuthScopes() []string {
	cfg := s.apiAuthConfig()
	return append([]string{}, cfg.Scopes...)
}

func (s *LaunchServer) AllowsScope(scope string) bool {
	cfg := s.apiAuthConfig()
	if !cfg.Requested() { return true }
	return scopeAllowed(cfg.Scopes, scope)
}

func requiredScopeForRequest(r *http.Request) string {
	if r == nil { return "" }
	path := strings.TrimSpace(r.URL.Path)
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	switch {
	case strings.HasPrefix(path, "/mcp"):
		return ""
	case path == "/api/diagnostics":
		return ScopeEnvironmentRead
	case strings.HasPrefix(path, "/api/batch/"):
		if method == http.MethodGet { return ScopeEnvironmentRead }
		if strings.HasSuffix(path, "/start") || strings.HasSuffix(path, "/stop") { return ScopePageAction }
		return ScopeEnvironmentAdmin
	case strings.HasPrefix(path, "/api/automation"):
		if method == http.MethodGet { return ScopeEnvironmentRead }
		return ScopePageAction
	case strings.HasPrefix(path, "/api/runtime"):
		if method == http.MethodGet || strings.HasSuffix(path, "/status") { return ScopeEnvironmentRead }
		return ScopePageAction
	case strings.HasPrefix(path, "/api/launch"):
		if strings.Contains(path, "/logs") || strings.Contains(path, "/tasks") { return ScopeEnvironmentRead }
		return ScopePageAction
	case strings.HasPrefix(path, "/api/profiles"):
		if strings.HasSuffix(path, "/runtime-verify") { return ScopePageRead }
		if strings.HasSuffix(path, "/runtime-evidence") || strings.HasSuffix(path, "/environment") ||
			strings.HasSuffix(path, "/launch-state") || strings.HasSuffix(path, "/launch-receipt") ||
			strings.HasSuffix(path, "/capabilities") || strings.HasSuffix(path, "/status") || strings.HasSuffix(path, "/view") {
			return ScopeEnvironmentRead
		}
		if method == http.MethodGet { return ScopeEnvironmentRead }
		if strings.HasSuffix(path, "/stop") { return ScopePageAction }
		return ScopeEnvironmentAdmin
	case path == "/api/health":
		return ScopeEnvironmentRead
	default:
		return ScopePageAction
	}
}

func (s *LaunchServer) SetAPIAuthConfig(cfg APIAuthConfig) {
	s.authMu.Lock()
	s.apiAuth = normalizeAPIAuthConfig(cfg)
	s.authMu.Unlock()
}

func (s *LaunchServer) apiAuthConfig() APIAuthConfig {
	s.authMu.RLock()
	cfg := s.apiAuth
	s.authMu.RUnlock()
	return cfg
}

func (s *LaunchServer) APIAuthHeader() string {
	return s.apiAuthConfig().Header
}

func (s *LaunchServer) APIAuthRequested() bool {
	return s.apiAuthConfig().Requested()
}

func (s *LaunchServer) APIAuthConfigured() bool {
	return s.apiAuthConfig().Configured()
}

func (s *LaunchServer) APIAuthEnabled() bool {
	return s.apiAuthConfig().Active()
}

func (s *LaunchServer) apiAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.requiresAPIAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		cfg := s.apiAuthConfig()
		if !cfg.Requested() {
			next.ServeHTTP(w, r)
			return
		}
		if !cfg.Configured() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"ok": false, "error": "API authentication is enabled but not configured",
			})
			return
		}

		values := r.Header.Values(cfg.Header)
		if len(values) != 1 || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(values[0])), []byte(cfg.APIKey)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"ok":         false,
				"error":      "unauthorized: invalid api key",
				"authHeader": cfg.Header,
			})
			return
		}

		required := requiredScopeForRequest(r)
		if required != "" && !scopeAllowed(cfg.Scopes, required) {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok": false,
				"error": fmt.Sprintf("forbidden: scope %s is required", required),
				"requiredScope": required,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Every route is a control API or falls through to the CDP reverse proxy.
// Do not exempt unrecognized paths from configured authentication.
func (s *LaunchServer) requiresAPIAuth(_ string) bool {
	return true
}
