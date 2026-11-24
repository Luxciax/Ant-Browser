package backend

type BrowserSystemIntegrationStatus struct {
	SchemaVersion int      `json:"schemaVersion"`
	Supported     bool     `json:"supported"`
	Protocol      string   `json:"protocol"`
	Registered    bool     `json:"registered"`
	Executable    string   `json:"executable,omitempty"`
	Actions       []string `json:"actions"`
	Message       string   `json:"message,omitempty"`
}

func (a *App) BrowserSystemIntegrationStatusGet() interface{} {
	return browserSystemIntegrationStatus()
}
func (a *App) BrowserSystemIntegrationSetProtocol(enabled bool) (interface{}, error) {
	if err := setBrowserURLProtocolRegistration(enabled); err != nil {
		return browserSystemIntegrationStatus(), err
	}
	return browserSystemIntegrationStatus(), nil
}
