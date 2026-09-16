package launchcode

import (
	"net/http"
	"strings"

	"ant-chrome/backend/internal/config"
)

// ProxyProvider exposes proxy-pool and browser-core capabilities to protocol
// adapters without adding duplicate exported methods to the Wails App type.
type ProxyProvider interface {
	ListProxyNodes() []config.BrowserProxy
	TestProxyNodeSpeed(proxyID string) ProxySpeedResult
	CheckProxyNodeHealth(proxyID string) ProxyHealthResult
	ListBrowserCores() []config.BrowserCore
}

type ProxySpeedResult struct {
	ProxyID   string `json:"proxyId"`
	Ok        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Engine    string `json:"engine"`
	Error     string `json:"error"`
}

type ProxyHealthResult struct {
	ProxyID        string `json:"proxyId"`
	Ok             bool   `json:"ok"`
	Source         string `json:"source"`
	Error          string `json:"error"`
	IP             string `json:"ip"`
	FraudScore     int64  `json:"fraudScore"`
	IsResidential  bool   `json:"isResidential"`
	Country        string `json:"country"`
	Region         string `json:"region"`
	City           string `json:"city"`
	AsOrganization string `json:"asOrganization"`
	UpdatedAt      string `json:"updatedAt"`
}

const proxyAPIUnavailable = "proxy api is unavailable"

func (s *LaunchServer) ListProxies() ([]config.BrowserProxy, error) {
	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}
	return provider.ListProxyNodes(), nil
}

func (s *LaunchServer) TestProxySpeed(proxyID string) (*ProxySpeedResult, error) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return nil, newServiceError(http.StatusBadRequest, "proxyId is required")
	}
	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}
	result := provider.TestProxyNodeSpeed(proxyID)
	return &result, nil
}

func (s *LaunchServer) CheckProxyHealth(proxyID string) (*ProxyHealthResult, error) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return nil, newServiceError(http.StatusBadRequest, "proxyId is required")
	}
	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}
	result := provider.CheckProxyNodeHealth(proxyID)
	return &result, nil
}

func (s *LaunchServer) ListCores() ([]config.BrowserCore, error) {
	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, "browser core api is unavailable")
	}
	return provider.ListBrowserCores(), nil
}

func (s *LaunchServer) SetProxyProvider(provider ProxyProvider) {
	s.proxyMu.Lock()
	s.proxy = provider
	s.proxyMu.Unlock()
}

func (s *LaunchServer) proxyProvider() ProxyProvider {
	s.proxyMu.RLock()
	defer s.proxyMu.RUnlock()
	return s.proxy
}
