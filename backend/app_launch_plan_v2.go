package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

type BrowserLaunchPlanV2 struct {
	SchemaVersion        int                             `json:"schemaVersion"`
	ProfileID            string                          `json:"profileId"`
	EnvironmentID        string                          `json:"environmentId"`
	EnvironmentRevision  string                          `json:"environmentRevision"`
	CreatedAt            string                          `json:"createdAt"`
	State                string                          `json:"state"`
	Kernel               BrowserKernelCapabilityManifest `json:"kernel"`
	Network              BrowserLaunchNetworkSummary     `json:"network"`
	Arguments            BrowserLaunchArgumentSummary    `json:"arguments"`
	Start                BrowserLaunchTargetSummary      `json:"start"`
	Warnings             []string                        `json:"warnings,omitempty"`
	Digest               string                          `json:"digest"`
}

type BrowserLaunchNetworkSummary struct {
	ConfiguredMode string `json:"configuredMode"`
	EffectiveMode  string `json:"effectiveMode"`
	ProxyID        string `json:"proxyId,omitempty"`
	Temporary      bool   `json:"temporary"`
	ConnectorType  string `json:"connectorType"`
}

type BrowserLaunchArgumentSummary struct {
	ProfileConfigured int `json:"profileConfigured"`
	TemporaryConfigured int `json:"temporaryConfigured"`
	PersonaConfigured int `json:"personaConfigured"`
	EffectiveCount int `json:"effectiveCount"`
}

type BrowserLaunchTargetSummary struct {
	ImmediateCount int  `json:"immediateCount"`
	DeferredCount  int  `json:"deferredCount"`
	RestoreLastSession bool `json:"restoreLastSession"`
}

type BrowserLaunchReceiptV2 struct {
	Plan       BrowserLaunchPlanV2 `json:"plan"`
	State      string              `json:"state"`
	StartedAt  string              `json:"startedAt,omitempty"`
	FinishedAt string              `json:"finishedAt,omitempty"`
	PID        int                 `json:"pid,omitempty"`
	DebugPort  int                 `json:"debugPort,omitempty"`
	DebugReady bool                `json:"debugReady"`
	Error      string              `json:"error,omitempty"`
}

func launchPlanDigest(plan BrowserLaunchPlanV2) string {
	copyPlan := plan
	copyPlan.Digest = ""
	payload, err := json.Marshal(copyPlan)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func configuredNetworkMode(profile *BrowserProfile, input browserStartInput) string {
	if input.ForceDirectProxy {
		return "direct"
	}
	if input.hasTemporaryProxy() {
		return "temporary_proxy"
	}
	if profile != nil && strings.TrimSpace(profile.ProxyId) != "" {
		return "proxy_pool"
	}
	if profile != nil && strings.TrimSpace(profile.ProxyConfig) != "" {
		return "custom_proxy"
	}
	return "direct"
}

func effectiveNetworkMode(effectiveProxy string) string {
	effectiveProxy = strings.TrimSpace(effectiveProxy)
	if effectiveProxy == "" || effectiveProxy == "direct://" {
		return "direct"
	}
	return "proxy"
}

func (a *App) buildLaunchPlanV2(input browserStartInput, plan *browserStartPlan, fingerprintArgCount int, immediateTargets int, restore bool) BrowserLaunchPlanV2 {
	out := BrowserLaunchPlanV2{
		SchemaVersion: 2,
		ProfileID: input.ProfileID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		State: "effective",
		Kernel: a.browserKernelCapabilityManifest(plan.profile),
		Network: BrowserLaunchNetworkSummary{
			ConfiguredMode: configuredNetworkMode(plan.profile, input),
			EffectiveMode: effectiveNetworkMode(plan.effectiveProxy),
			ProxyID: strings.TrimSpace(plan.profile.ProxyId),
			Temporary: input.hasTemporaryProxy() || input.ForceDirectProxy,
			ConnectorType: config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType),
		},
		Arguments: BrowserLaunchArgumentSummary{
			ProfileConfigured: len(normalizeNonEmptyStrings(plan.profile.LaunchArgs)),
			TemporaryConfigured: len(normalizeNonEmptyStrings(input.ExtraLaunchArgs)),
			PersonaConfigured: fingerprintArgCount,
			EffectiveCount: len(plan.args),
		},
		Start: BrowserLaunchTargetSummary{
			ImmediateCount: immediateTargets,
			DeferredCount: len(plan.deferredStartTargets),
			RestoreLastSession: restore,
		},
	}
	if contract, err := a.browserMgr.EnvironmentContract(input.ProfileID); err == nil && contract != nil {
		out.EnvironmentID = contract.Environment.EnvironmentID
		out.EnvironmentRevision = contract.Environment.Revision
	} else if err != nil {
		out.Warnings = append(out.Warnings, "environment contract unavailable: "+err.Error())
	}
	if plan.extensionWarning != "" {
		out.Warnings = append(out.Warnings, plan.extensionWarning)
	}
	out.Digest = launchPlanDigest(out)
	return out
}

func (a *App) storeLaunchReceipt(profileID string, receipt BrowserLaunchReceiptV2) {
	if a == nil {
		return
	}
	a.launchPlanMu.Lock()
	if a.launchReceipts == nil {
		a.launchReceipts = make(map[string]BrowserLaunchReceiptV2)
	}
	a.launchReceipts[strings.TrimSpace(profileID)] = receipt
	a.launchPlanMu.Unlock()
}

func (a *App) updateLaunchReceipt(profileID string, update func(*BrowserLaunchReceiptV2)) {
	if a == nil || update == nil {
		return
	}
	a.launchPlanMu.Lock()
	receipt, ok := a.launchReceipts[strings.TrimSpace(profileID)]
	if ok {
		update(&receipt)
		a.launchReceipts[strings.TrimSpace(profileID)] = receipt
	}
	a.launchPlanMu.Unlock()
}

func (a *App) BrowserProfileLaunchReceipt(profileID string) (*BrowserLaunchReceiptV2, error) {
	if a == nil {
		return nil, fmt.Errorf("application is unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, fmt.Errorf("profile id is required")
	}
	a.launchPlanMu.RLock()
	receipt, ok := a.launchReceipts[profileID]
	a.launchPlanMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("launch receipt not found")
	}
	copyReceipt := receipt
	copyReceipt.Plan.Warnings = append([]string{}, receipt.Plan.Warnings...)
	copyReceipt.Plan.Kernel.Warnings = append([]string{}, receipt.Plan.Kernel.Warnings...)
	copyReceipt.Plan.Kernel.Capabilities = append([]BrowserKernelCapabilityStatus{}, receipt.Plan.Kernel.Capabilities...)
	return &copyReceipt, nil
}

// BrowserProfileLaunchPlanState reports whether configuration changes are
// effective now or will apply on the next process start.
func (a *App) BrowserProfileLaunchPlanState(profileID string) (map[string]interface{}, error) {
	if a == nil || a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is unavailable")
	}
	contract, err := a.browserMgr.EnvironmentContract(profileID)
	if err != nil {
		return nil, err
	}
	state := "configured"
	if contract.Runtime.Running {
		state = "pending-next-launch"
		if receipt, receiptErr := a.BrowserProfileLaunchReceipt(profileID); receiptErr == nil &&
			receipt.Plan.EnvironmentRevision == contract.Environment.Revision {
			state = "effective"
		}
	}
	return map[string]interface{}{
		"profileId": profileID,
		"environmentId": contract.Environment.EnvironmentID,
		"environmentRevision": contract.Environment.Revision,
		"state": state,
	}, nil
}

var _ = browser.EnvironmentContractSchemaVersion
