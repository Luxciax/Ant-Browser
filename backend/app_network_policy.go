package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/browser"
)

type BrowserNetworkPolicy = browser.NetworkPolicy

func (a *App) BrowserProfileNetworkPolicy(profileID string) (BrowserNetworkPolicy, error) {
	if a == nil || a.browserMgr == nil {
		return BrowserNetworkPolicy{}, fmt.Errorf("browser manager is unavailable")
	}
	return a.browserMgr.GetNetworkPolicy(strings.TrimSpace(profileID))
}

func (a *App) BrowserProfileSetNetworkPolicy(policy BrowserNetworkPolicy) (BrowserNetworkPolicy, error) {
	if a == nil || a.browserMgr == nil {
		return BrowserNetworkPolicy{}, fmt.Errorf("browser manager is unavailable")
	}
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(policy.ProfileID)
	defer unlockRuntimeOp()
	return a.browserMgr.SetNetworkPolicy(policy)
}

func networkPolicyLaunchArgs(policy BrowserNetworkPolicy) []string {
	args := []string{}
	if policy.WebRTCMode == browser.NetworkWebRTCRestricted {
		args = append(args, "--disable-non-proxied-udp")
	}
	return args
}

func validateEffectiveNetworkPolicy(policy BrowserNetworkPolicy, configuredMode, effectiveMode string) error {
	if !policy.FailClosed {
		return nil
	}
	configuredMode = strings.TrimSpace(configuredMode)
	effectiveMode = strings.TrimSpace(effectiveMode)
	if configuredMode != "direct" && effectiveMode == "direct" {
		return fmt.Errorf("network policy is fail-closed: configured proxy did not produce a proxied browser route")
	}
	return nil
}
