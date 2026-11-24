package backend

import "fmt"

type BrowserEnvironmentViewV2 struct {
	SchemaVersion int                              `json:"schemaVersion"`
	Contract      map[string]interface{}           `json:"contract"`
	LaunchState   map[string]interface{}           `json:"launchState"`
	LaunchReceipt map[string]interface{}           `json:"launchReceipt,omitempty"`
	Capabilities  map[string]interface{}           `json:"capabilities"`
	RuntimeEvidence map[string]interface{}         `json:"runtimeEvidence,omitempty"`
	Templates     []BrowserEnvironmentTemplate     `json:"templates"`
	TemplateCompatibility *BrowserTemplateCompatibility `json:"templateCompatibility,omitempty"`
}

func (a *App) BrowserProfileEnvironmentView(profileID string) (*BrowserEnvironmentViewV2, error) {
	if a == nil { return nil, fmt.Errorf("application is unavailable") }
	contract, err := a.BrowserControlEnvironmentContract(profileID)
	if err != nil { return nil, err }
	state, err := a.BrowserControlLaunchPlanState(profileID)
	if err != nil { return nil, err }
	capabilities, err := a.BrowserControlKernelCapabilities(profileID)
	if err != nil { return nil, err }
	view := &BrowserEnvironmentViewV2{
		SchemaVersion: 2,
		Contract: contract,
		LaunchState: state,
		Capabilities: capabilities,
		Templates: a.BrowserEnvironmentTemplateList(),
	}
	if receipt, receiptErr := a.BrowserControlLaunchReceipt(profileID); receiptErr == nil {
		view.LaunchReceipt = receipt
	}
	if evidence, evidenceErr := a.BrowserControlRuntimeEvidence(profileID); evidenceErr == nil {
		view.RuntimeEvidence = evidence
	}
	if compatibility, compatibilityErr := a.BrowserProfileTemplateCompatibility(profileID); compatibilityErr == nil {
		view.TemplateCompatibility = compatibility
	}
	return view, nil
}

func (a *App) BrowserControlEnvironmentView(profileID string) (map[string]interface{}, error) {
	view, err := a.BrowserProfileEnvironmentView(profileID)
	if err != nil { return nil, err }
	return phase2PublicMap(view)
}

func (a *App) BrowserControlDiagnostics() (map[string]interface{}, error) {
	if a == nil { return nil, fmt.Errorf("application is unavailable") }
	info := a.GetLaunchServerInfo()
	a.batchMu.RLock()
	batchCount := len(a.batchOperations)
	a.batchMu.RUnlock()
	return map[string]interface{}{
		"ok": true,
		"schemaVersion": 1,
		"launchServer": info,
		"secretStoreReady": a.secretStore != nil,
		"batchOperationCount": batchCount,
		"runtimeEvidenceCount": func() int {
			a.runtimeEvidenceMu.RLock(); defer a.runtimeEvidenceMu.RUnlock()
			return len(a.runtimeEvidence)
		}(),
	}, nil
}
