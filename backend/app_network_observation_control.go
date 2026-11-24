package backend

func (a *App) BrowserControlNetworkObserve(profileID, url string, timeoutMs int) (map[string]interface{}, error) {
	value, err := a.BrowserNetworkObserve(BrowserNetworkObservationRequest{ProfileID:profileID,URL:url,TimeoutMs:timeoutMs})
	if value == nil { return nil, err }
	public, mapErr := phase2PublicMap(value)
	if mapErr != nil { return nil, mapErr }
	return public, err
}
func (a *App) BrowserControlNetworkObservationGet(id string) (map[string]interface{}, error) {
	value, err := a.BrowserNetworkObservationGet(id)
	if err != nil { return nil, err }
	return phase2PublicMap(value)
}
