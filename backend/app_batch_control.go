package backend

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

func (a *App) BrowserControlBatchCreate(inputs []BrowserProfileInput, clientKeys []string, concurrency int) (map[string]interface{}, error) {
	items := make([]BrowserBatchCreateItem, 0, len(inputs))
	for index, input := range inputs {
		clientKey := ""
		if index < len(clientKeys) { clientKey = clientKeys[index] }
		items = append(items, BrowserBatchCreateItem{ClientKey:clientKey, Config:input})
	}
	op, err := a.BrowserBatchCreate(BrowserBatchCreateRequest{Items:items, Concurrency:concurrency})
	if err != nil { return nil, err }
	return phase2PublicMap(op)
}
