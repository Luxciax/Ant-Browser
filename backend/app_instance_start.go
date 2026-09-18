package backend

import "context"

func (a *App) BrowserInstanceStart(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, nil, nil, false, false, false, "", "")
}

func shouldPreferVisibleWindowForStartWithParams(startURLs []string) bool {
	return len(normalizeNonEmptyStrings(startURLs)) > 0
}

// BrowserInstanceStartDirect 仅本次启动走直连，不落库修改实例代理配置。
func (a *App) BrowserInstanceStartDirect(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, nil, nil, false, false, true, "", "")
}

// BrowserInstanceStartWithParams 通过额外参数启动实例（仅本次启动生效，不落库）
func (a *App) BrowserInstanceStartWithParams(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool) (*BrowserProfile, error) {
	preferVisibleWindow := shouldPreferVisibleWindowForStartWithParams(startURLs)
	return a.browserInstanceStartInternal(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, false, "", "")
}

func (a *App) browserInstanceStartInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyId string, proxyConfig string) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()
	return a.browserInstanceStartWithRuntimeLock(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, forceDirectProxy, proxyId, proxyConfig)
}

// browserInstanceStartWithRuntimeLock executes one start transaction while the
// caller owns the per-profile lifecycle lock. Restart uses this helper so Stop
// -> Start is one atomic lifecycle operation with no interleaving window.
func (a *App) browserInstanceStartWithRuntimeLock(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyId string, proxyConfig string) (*BrowserProfile, error) {
	input := newBrowserStartInput(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, forceDirectProxy, proxyId, proxyConfig)
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	profile, handled, err := a.resolveBrowserStartProfile(input)
	if err != nil || handled {
		a.browserMgr.Mutex.Unlock()
		return profile, err
	}
	a.markProfileStartingLocked(profile)
	startingSnapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	// A stopped/failed profile may still have a stale Node page session from a
	// previous browser generation. Drop it before preparing the new runtime.
	a.closeProfilePageSession(profileId)
	a.emitBrowserInstanceUpdated(startingSnapshot)

	ctx, cancel := context.WithTimeout(context.Background(), browserStartTransactionTimeout(a.config))
	defer cancel()

	plan, err := a.prepareBrowserStartPlan(ctx, input, profile)
	if err == errBrowserStartHandledByRecoveredRuntime {
		a.emitBrowserInstanceStarted(profile, true)
		return profile, nil
	}
	if err != nil {
		a.markProfileStartFailed(input.ProfileID, profile, err)
		return profile, err
	}
	defer plan.releaseBridgeIfNeeded(a)

	startedProfile, err := a.startBrowserProfileWithPlan(ctx, input, plan)
	if err != nil {
		a.markProfileStartFailed(input.ProfileID, profile, err)
		return profile, err
	}
	return startedProfile, nil
}
