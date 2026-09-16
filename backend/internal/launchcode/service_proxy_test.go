package launchcode

import (
	"errors"
	"net/http"
	"testing"

	"ant-chrome/backend/internal/config"
)

type testProxyProvider struct{}

func (testProxyProvider) ListProxyNodes() []config.BrowserProxy {
	return []config.BrowserProxy{{ProxyId: "proxy-a", ProxyName: "Proxy A", ProxyConfig: "socks5://127.0.0.1:1080"}}
}

func (testProxyProvider) TestProxyNodeSpeed(proxyID string) ProxySpeedResult {
	return ProxySpeedResult{ProxyID: proxyID, Ok: true, LatencyMs: 42, Engine: "xray"}
}

func (testProxyProvider) CheckProxyNodeHealth(proxyID string) ProxyHealthResult {
	return ProxyHealthResult{ProxyID: proxyID, Ok: true, IP: "203.0.113.1", Country: "US"}
}

func (testProxyProvider) ListBrowserCores() []config.BrowserCore {
	return []config.BrowserCore{{CoreId: "core-a", CoreName: "Core A"}}
}

func TestProxyFacadeUnavailableWithoutProvider(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	_, err := server.ListProxies()
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) || !serviceErr.Unavailable() {
		t.Fatalf("ListProxies error = %v, want unavailable ServiceError", err)
	}
}

func TestProxyFacadeValidatesProxyID(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetProxyProvider(testProxyProvider{})
	_, err := server.TestProxySpeed(" ")
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) || serviceErr.Status != http.StatusBadRequest {
		t.Fatalf("TestProxySpeed error = %v, want bad request ServiceError", err)
	}
}

func TestProxyFacadeDelegatesToProvider(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetProxyProvider(testProxyProvider{})

	proxies, err := server.ListProxies()
	if err != nil || len(proxies) != 1 || proxies[0].ProxyId != "proxy-a" {
		t.Fatalf("ListProxies = %+v, %v", proxies, err)
	}
	speed, err := server.TestProxySpeed("proxy-a")
	if err != nil || !speed.Ok || speed.LatencyMs != 42 {
		t.Fatalf("TestProxySpeed = %+v, %v", speed, err)
	}
	health, err := server.CheckProxyHealth("proxy-a")
	if err != nil || !health.Ok || health.IP != "203.0.113.1" {
		t.Fatalf("CheckProxyHealth = %+v, %v", health, err)
	}
	cores, err := server.ListCores()
	if err != nil || len(cores) != 1 || cores[0].CoreId != "core-a" {
		t.Fatalf("ListCores = %+v, %v", cores, err)
	}
}
