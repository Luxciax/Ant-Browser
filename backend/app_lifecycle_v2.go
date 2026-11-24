package backend

import (
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"github.com/google/uuid"
)

type BrowserLifecycleCloneRequest struct {
	ProfileID      string `json:"profileId"`
	NewName        string `json:"newName,omitempty"`
	EnvironmentMode string `json:"environmentMode"`
}

type BrowserLifecycleCloneResult struct {
	Profile       *BrowserProfile `json:"profile"`
	EnvironmentID string          `json:"environmentId"`
	SourceEnvironmentID string    `json:"sourceEnvironmentId"`
	Mode          string          `json:"mode"`
}

type BrowserKernelUpgradePreview struct {
	ProfileID       string                          `json:"profileId"`
	CurrentCoreID   string                          `json:"currentCoreId,omitempty"`
	TargetCoreID    string                          `json:"targetCoreId"`
	Current         BrowserKernelCapabilityManifest `json:"current"`
	Target          BrowserKernelCapabilityManifest `json:"target"`
	RequiresRestart bool                            `json:"requiresRestart"`
	Warnings        []string                        `json:"warnings,omitempty"`
}

func (a *App) BrowserProfileCloneLifecycle(req BrowserLifecycleCloneRequest) (*BrowserLifecycleCloneResult, error) {
	if a == nil || a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is unavailable")
	}
	if a.browserMgr.EnvironmentDAO == nil || a.browserMgr.NetworkPolicyDAO == nil {
		return nil, fmt.Errorf("lifecycle persistence is unavailable")
	}
	sourceID := strings.TrimSpace(req.ProfileID)
	mode := strings.ToLower(strings.TrimSpace(req.EnvironmentMode))
	if sourceID == "" {
		return nil, fmt.Errorf("profile id is required")
	}
	if mode == "" {
		mode = "new-environment"
	}
	if mode != "new-environment" && mode != "preserve-environment" {
		return nil, fmt.Errorf("environmentMode must be new-environment or preserve-environment")
	}
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	sourceIdentity, err := a.browserMgr.EnvironmentDAO.Ensure(sourceID)
	if err != nil {
		return nil, err
	}
	cloned, err := a.browserMgr.CopyWithOptions(sourceID, req.NewName, browser.ProfileCopyOptions{Mode: "regular"})
	if err != nil {
		return nil, err
	}
	targetIdentity, err := a.browserMgr.EnvironmentDAO.Ensure(cloned.ProfileId)
	if err != nil {
		return nil, err
	}
	if mode == "preserve-environment" {
		setter, ok := a.browserMgr.EnvironmentDAO.(interface {
			Set(profileID, environmentID string) (browser.EnvironmentIdentity, error)
		})
		if !ok {
			return nil, fmt.Errorf("environment identity update is unavailable")
		}
		targetIdentity, err = setter.Set(cloned.ProfileId, sourceIdentity.EnvironmentID)
		if err != nil {
			return nil, err
		}
	}
	sourcePolicy, policyErr := a.browserMgr.GetNetworkPolicy(sourceID)
	if policyErr != nil {
		return nil, policyErr
	}
	sourcePolicy.ProfileID = cloned.ProfileId
	sourcePolicy.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := a.browserMgr.NetworkPolicyDAO.Upsert(sourcePolicy); err != nil {
		return nil, err
	}
	if a.browserMgr.TemplateBindingDAO != nil {
		if binding, bindErr := a.browserMgr.TemplateBindingDAO.Get(sourceID); bindErr == nil {
			binding.ProfileID = cloned.ProfileId
			binding.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := a.browserMgr.TemplateBindingDAO.Upsert(binding); err != nil {
				return nil, err
			}
		}
	}
	return &BrowserLifecycleCloneResult{
		Profile: a.snapshotManagedBrowserProfile(cloned),
		EnvironmentID: targetIdentity.EnvironmentID,
		SourceEnvironmentID: sourceIdentity.EnvironmentID,
		Mode: mode,
	}, nil
}

func (a *App) BrowserProfileRefreshEnvironmentID(profileID string) (*browser.EnvironmentIdentity, error) {
	if a == nil || a.browserMgr == nil || a.browserMgr.EnvironmentDAO == nil {
		return nil, fmt.Errorf("environment identity persistence is unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileID)
	defer unlockRuntimeOp()
	a.browserMgr.Mutex.Lock()
	profile := a.browserMgr.Profiles[profileID]
	if profile == nil {
		a.browserMgr.Mutex.Unlock()
		return nil, fmt.Errorf("profile not found")
	}
	if browser.ProfileRuntimeMutationBlocked(profile) {
		state := browser.NormalizeProfileRuntimeState(profile)
		a.browserMgr.Mutex.Unlock()
		return nil, fmt.Errorf("profile runtime is %s; stop it before refreshing environment id", state)
	}
	profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if a.browserMgr.ProfileDAO != nil {
		if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
			a.browserMgr.Mutex.Unlock()
			return nil, err
		}
	}
	a.browserMgr.Mutex.Unlock()
	setter, ok := a.browserMgr.EnvironmentDAO.(interface {
		Set(profileID, environmentID string) (browser.EnvironmentIdentity, error)
	})
	if !ok {
		return nil, fmt.Errorf("environment identity update is unavailable")
	}
	identity, err := setter.Set(profileID, uuid.NewString())
	if err != nil {
		return nil, err
	}
	a.launchPlanMu.Lock()
	delete(a.launchReceipts, profileID)
	a.launchPlanMu.Unlock()
	a.runtimeEvidenceMu.Lock()
	delete(a.runtimeEvidence, profileID)
	a.runtimeEvidenceMu.Unlock()
	return &identity, nil
}

func (a *App) BrowserProfileKernelUpgradePreview(profileID, targetCoreID string) (*BrowserKernelUpgradePreview, error) {
	if a == nil || a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager is unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	targetCoreID = strings.TrimSpace(targetCoreID)
	if targetCoreID == "" {
		return nil, fmt.Errorf("target core id is required")
	}
	a.browserMgr.Mutex.Lock()
	profile := a.browserMgr.Profiles[profileID]
	if profile == nil {
		a.browserMgr.Mutex.Unlock()
		return nil, fmt.Errorf("profile not found")
	}
	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	if _, ok := a.browserMgr.GetCore(targetCoreID); !ok {
		return nil, fmt.Errorf("target core not found")
	}
	currentManifest := a.browserKernelCapabilityManifest(snapshot)
	targetProfile := *snapshot
	targetProfile.CoreId = targetCoreID
	targetManifest := a.browserKernelCapabilityManifest(&targetProfile)
	preview := &BrowserKernelUpgradePreview{
		ProfileID: profileID,
		CurrentCoreID: snapshot.CoreId,
		TargetCoreID: targetCoreID,
		Current: currentManifest,
		Target: targetManifest,
		RequiresRestart: snapshot.Running || snapshot.CoreId != targetCoreID,
	}
	if targetManifest.BinarySHA256 == "" {
		preview.Warnings = append(preview.Warnings, "target kernel binary identity could not be established")
	}
	if currentManifest.BrowserVersion != "" && targetManifest.BrowserVersion != "" && currentManifest.BrowserVersion != targetManifest.BrowserVersion {
		preview.Warnings = append(preview.Warnings, "browser version changes; runtime observations should be re-verified after restart")
	}
	return preview, nil
}

func (a *App) BrowserProfileKernelUpgradeApply(profileID, targetCoreID string) (*BrowserProfile, error) {
	preview, err := a.BrowserProfileKernelUpgradePreview(profileID, targetCoreID)
	if err != nil {
		return nil, err
	}
	if preview.Target.BinarySHA256 == "" {
		return nil, fmt.Errorf("target kernel executable identity is unavailable")
	}
	coreID := strings.TrimSpace(targetCoreID)
	return a.PatchBrowserProfile(profileID, browser.ProfilePatchInput{CoreId: &coreID})
}
