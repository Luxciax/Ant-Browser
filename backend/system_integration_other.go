//go:build !windows

package backend

import "fmt"

func browserSystemIntegrationStatus() BrowserSystemIntegrationStatus {
	return BrowserSystemIntegrationStatus{
		SchemaVersion: 1, Supported: false, Protocol: "antbrowser",
		Actions: []string{}, Message: "optional URL protocol registration is currently supported on Windows only",
	}
}
func setBrowserURLProtocolRegistration(enabled bool) error {
	return fmt.Errorf("URL protocol registration is unsupported on this platform")
}
