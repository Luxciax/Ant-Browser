package backend

import (
	"encoding/json"
	"fmt"
)

func phase2PublicMap(value interface{}) (map[string]interface{}, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *App) BrowserControlEnvironmentContract(profileID string) (map[string]interface{}, error) {
	value, err := a.BrowserProfileEnvironmentContract(profileID)
	if err != nil {
		return nil, err
	}
	return phase2PublicMap(value)
}

func (a *App) BrowserControlLaunchPlanState(profileID string) (map[string]interface{}, error) {
	return a.BrowserProfileLaunchPlanState(profileID)
}

func (a *App) BrowserControlLaunchReceipt(profileID string) (map[string]interface{}, error) {
	value, err := a.BrowserProfileLaunchReceipt(profileID)
	if err != nil {
		return nil, err
	}
	return phase2PublicMap(value)
}

func (a *App) BrowserControlKernelCapabilities(profileID string) (map[string]interface{}, error) {
	value, err := a.BrowserProfileKernelCapabilityManifest(profileID)
	if err != nil {
		return nil, err
	}
	return phase2PublicMap(value)
}

func (a *App) BrowserControlRuntimeEvidence(profileID string) (map[string]interface{}, error) {
	value, err := a.BrowserProfileRuntimeEvidence(profileID)
	if err != nil {
		return nil, err
	}
	return phase2PublicMap(value)
}

func (a *App) BrowserControlRuntimeVerify(profileID string) (map[string]interface{}, error) {
	value, err := a.BrowserProfileRuntimeVerify(profileID)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("runtime verification returned no evidence")
	}
	return phase2PublicMap(value)
}
