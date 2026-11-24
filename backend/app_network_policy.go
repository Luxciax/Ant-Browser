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

func mergeNetworkPolicyLaunchArgs(args []string, policy BrowserNetworkPolicy) []string {
	out := append([]string{}, args...)
	if policy.WebRTCMode != browser.NetworkWebRTCRestricted {
		return out
	}
	for _, arg := range out {
		if strings.TrimSpace(arg) == "--disable-non-proxied-udp" {
			return out
		}
	}
	return append(out, "--disable-non-proxied-udp")
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
