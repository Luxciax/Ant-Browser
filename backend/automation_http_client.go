package backend

import (
	"net"
	"net/http"
	"time"
)

var automationHTTPClient = newAutomationHTTPClient()

func newAutomationHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          32,
			MaxIdleConnsPerHost:   8,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		// Request-specific contexts define the total operation deadline. This is
		// intentionally zero because public automation API calls may legitimately
		// run for much longer than the connection setup timeout.
		Timeout: 0,
	}
}
