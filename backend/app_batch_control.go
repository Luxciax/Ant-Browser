package backend

import "fmt"

func (a *App) BrowserControlBatchOperation(operationID string) (map[string]interface{}, error) {
	op, err := a.BrowserBatchOperationGet(operationID)
	if err != nil { return nil, err }
	return phase2PublicMap(op)
}

func (a *App) BrowserControlBatchStart(profileIDs []string, concurrency int) (map[string]interface{}, error) {
	op, err := a.BrowserBatchStart(BrowserBatchProfilesRequest{ProfileIDs:profileIDs, Concurrency:concurrency})
	if err != nil { return nil, err }
	return phase2PublicMap(op)
}

func (a *App) BrowserControlBatchStop(profileIDs []string, concurrency int) (map[string]interface{}, error) {
	op, err := a.BrowserBatchStop(BrowserBatchProfilesRequest{ProfileIDs:profileIDs, Concurrency:concurrency})
	if err != nil { return nil, err }
	return phase2PublicMap(op)
}

func (a *App) BrowserControlBatchCreate(items []BrowserBatchCreateItem, concurrency int) (map[string]interface{}, error) {
	if len(items) == 0 { return nil, fmt.Errorf("batch items are required") }
	op, err := a.BrowserBatchCreate(BrowserBatchCreateRequest{Items:items, Concurrency:concurrency})
	if err != nil { return nil, err }
	return phase2PublicMap(op)
}
