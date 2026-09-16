package backend

import "ant-chrome/backend/internal/launchcode"

// appProxyProvider keeps LaunchServer protocol adapters independent from Wails
// bindings while reusing the application's existing proxy/core business APIs.
type appProxyProvider struct {
	app *App
}

func newAppProxyProvider(app *App) launchcode.ProxyProvider {
	return appProxyProvider{app: app}
}

func (p appProxyProvider) ListProxyNodes() []BrowserProxy {
	if p.app == nil || p.app.browserMgr == nil || p.app.config == nil {
		return []BrowserProxy{}
	}
	return p.app.BrowserProxyList()
}

func (p appProxyProvider) TestProxyNodeSpeed(proxyID string) launchcode.ProxySpeedResult {
	if p.app == nil {
		return launchcode.ProxySpeedResult{ProxyID: proxyID}
	}
	result := p.app.BrowserProxyTestSpeed(proxyID)
	return launchcode.ProxySpeedResult{
		ProxyID:   result.ProxyId,
		Ok:        result.Ok,
		LatencyMs: result.LatencyMs,
		Engine:    result.Engine,
		Error:     result.Error,
	}
}

func (p appProxyProvider) CheckProxyNodeHealth(proxyID string) launchcode.ProxyHealthResult {
	if p.app == nil {
		return launchcode.ProxyHealthResult{ProxyID: proxyID}
	}
	result := p.app.BrowserProxyCheckIPHealth(proxyID)
	return launchcode.ProxyHealthResult{
		ProxyID:        result.ProxyId,
		Ok:             result.Ok,
		Source:         result.Source,
		Error:          result.Error,
		IP:             result.IP,
		FraudScore:     result.FraudScore,
		IsResidential:  result.IsResidential,
		Country:        result.Country,
		Region:         result.Region,
		City:           result.City,
		AsOrganization: result.AsOrganization,
		UpdatedAt:      result.UpdatedAt,
	}
}

func (p appProxyProvider) ListBrowserCores() []BrowserCore {
	if p.app == nil || p.app.browserMgr == nil {
		return []BrowserCore{}
	}
	return p.app.BrowserCoreList()
}

var _ launchcode.ProxyProvider = appProxyProvider{}
