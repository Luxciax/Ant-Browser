package launchcode

import (
	"net/http"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
)

// ServiceError is the protocol-neutral error returned by the LaunchServer
// service facade. HTTP handlers, MCP tools and future local transports can map
// the preserved status code without duplicating business error classification.
type ServiceError struct {
	Status  int
	Message string
}

func (e *ServiceError) Error() string { return e.Message }

func (e *ServiceError) NotFound() bool  { return e != nil && e.Status == http.StatusNotFound }
func (e *ServiceError) Ambiguous() bool { return e != nil && e.Status == http.StatusConflict }
func (e *ServiceError) Unavailable() bool {
	return e != nil && e.Status == http.StatusServiceUnavailable
}

func newServiceError(status int, message string) *ServiceError {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	return &ServiceError{Status: status, Message: message}
}

// NewServiceError lets injected service providers preserve a meaningful status
// classification without depending on an HTTP response writer.
func NewServiceError(status int, message string) *ServiceError {
	return newServiceError(status, message)
}

func serviceErrorFrom(status int, message string) *ServiceError {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	return newServiceError(status, message)
}

// RuntimeSession is the transport-neutral result of opening or querying a
// browser takeover session.
type RuntimeSession struct {
	Profile    *browser.Profile `json:"profile,omitempty"`
	Ready      bool             `json:"ready"`
	LaunchCode string           `json:"launchCode,omitempty"`
	CDPURL     string           `json:"cdpUrl,omitempty"`
}

// ListProfiles returns all profile snapshots and their launch codes.
func (s *LaunchServer) ListProfiles() ([]browser.Profile, error) {
	items, status, errMsg := s.listProfiles()
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return items, nil
}

// FindProfiles resolves all profiles matching selector.
func (s *LaunchServer) FindProfiles(selector LaunchSelector) ([]browser.Profile, error) {
	items, status, errMsg := s.findProfilesBySelector(normalizeLaunchSelector(selector))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return items, nil
}

// FindProfile resolves one profile. Ambiguous selectors remain distinguishable
// through ServiceError.Ambiguous().
func (s *LaunchServer) FindProfile(selector LaunchSelector) (*browser.Profile, error) {
	profile, status, errMsg := s.findProfileBySelector(normalizeLaunchSelector(selector))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return &profile, nil
}

// CreateProfile creates a profile and allocates a launch code when none is
// explicitly requested.
func (s *LaunchServer) CreateProfile(input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error) {
	profile, launchCode, status, errMsg := s.createProfile(input, requestedCode)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, "", err
	}
	return profile, launchCode, nil
}

// UpdateProfile updates a profile while retaining the existing rollback
// behaviour for launch-code updates.
func (s *LaunchServer) UpdateProfile(profileID string, input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error) {
	previous, status, errMsg := s.profileSnapshotByID(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, "", err
	}
	profile, launchCode, status, errMsg := s.updateProfile(profileID, input, requestedCode, previous)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, "", err
	}
	return profile, launchCode, nil
}

// DeleteProfile removes a stopped profile.
func (s *LaunchServer) DeleteProfile(profileID string) error {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return newServiceError(http.StatusNotFound, "profile not found")
	}
	snapshot, status, errMsg := s.profileSnapshotByID(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return err
	}
	if snapshot != nil && browser.ProfileRuntimeMutationBlocked(snapshot) {
		return newServiceError(http.StatusConflict, "running profile cannot be deleted")
	}
	if err := s.deleteProfileWithLaunchCode(profileID, snapshot); err != nil {
		return newServiceError(mapProfileWriteErrorStatus(err), err.Error())
	}
	return nil
}

// StartProfile starts a profile without waiting for its CDP endpoint.
func (s *LaunchServer) StartProfile(selector LaunchSelector, params LaunchRequestParams) (*browser.Profile, string, error) {
	profile, launchCode, status, errMsg := s.launchBySelector(normalizeLaunchSelector(selector), normalizeLaunchRequestParams(params))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, launchCode, err
	}
	return profile, launchCode, nil
}

// StatusProfile reads runtime state without launching the profile.
func (s *LaunchServer) StatusProfile(profileID string) (*browser.Profile, error) {
	profile, status, errMsg := s.statusProfile(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return profile, nil
}

// StopProfile stops a profile and keeps the unified CDP target in sync.
func (s *LaunchServer) StopProfile(profileID string) (*browser.Profile, error) {
	profile, status, errMsg := s.stopProfile(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return profile, nil
}

// OpenRuntimeSession starts a matching profile and waits for CDP readiness.
// Readiness timeout is returned as Ready=false rather than a transport failure.
func (s *LaunchServer) OpenRuntimeSession(selector LaunchSelector, params LaunchRequestParams, timeout time.Duration) (*RuntimeSession, error) {
	normalized := normalizeRuntimeSelector(selector)
	if normalized.IsEmpty() {
		return nil, newServiceError(http.StatusBadRequest, "selector is required")
	}
	if err := validateRuntimeSelector(normalized); err != nil {
		return nil, newServiceError(http.StatusBadRequest, err.Error())
	}

	profile, launchCode, status, errMsg := s.launchBySelector(normalized, normalizeLaunchRequestParams(params))
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = defaultRuntimeSessionTimeout
	}
	profile, ready, err := s.prepareRuntimeSession(profile, timeout)
	if err != nil {
		return nil, newServiceError(mapProfileWriteErrorStatus(err), err.Error())
	}
	if profile == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, "runtime session is not available")
	}
	if launchCode != "" && profile.LaunchCode == "" {
		profile.LaunchCode = launchCode
	}
	session := &RuntimeSession{Profile: profile, Ready: ready, LaunchCode: profile.LaunchCode}
	if ready {
		session.CDPURL = s.CDPURL()
	}
	return session, nil
}

// ActiveRuntimeSession returns the profile currently attached to the unified
// CDP endpoint. No active profile is represented by a nil session.
func (s *LaunchServer) ActiveRuntimeSession() (*RuntimeSession, error) {
	profileID, _, port := s.ActiveProfile()
	if strings.TrimSpace(profileID) == "" || port <= 0 {
		return nil, nil
	}
	profile, status, errMsg := s.statusProfile(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, err
	}
	return &RuntimeSession{
		Profile:    profile,
		Ready:      profile != nil && profile.DebugReady,
		LaunchCode: profile.LaunchCode,
		CDPURL:     s.CDPURL(),
	}, nil
}

// ListScripts returns automation script metadata.
func (s *LaunchServer) ListScripts() ([]automation.ScriptRecord, error) {
	lister, ok := s.starter.(AutomationScriptLister)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}
	items, err := lister.AutomationScriptList()
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	return items, nil
}

// GetScript returns one automation script.
func (s *LaunchServer) GetScript(scriptID string) (*automation.ScriptRecord, error) {
	scriptID = strings.TrimSpace(scriptID)
	if scriptID == "" {
		return nil, newServiceError(http.StatusBadRequest, "scriptId is required")
	}
	getter, ok := s.starter.(AutomationScriptGetter)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}
	record, err := getter.AutomationScriptGet(scriptID)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	if record == nil {
		return nil, newServiceError(http.StatusNotFound, "automation script not found")
	}
	return record, nil
}

// RunScript executes an automation script synchronously from the caller's
// perspective.
func (s *LaunchServer) RunScript(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error) {
	if strings.TrimSpace(input.ScriptID) == "" {
		return nil, newServiceError(http.StatusBadRequest, "scriptId is required")
	}
	runner, ok := s.starter.(AutomationScriptRunner)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}
	record, err := runner.AutomationScriptRunWithOptions(input)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	if record == nil {
		return nil, newServiceError(http.StatusInternalServerError, "automation script run returned no record")
	}
	return record, nil
}

// ListScriptRuns returns recent automation script runs.
func (s *LaunchServer) ListScriptRuns(limit int) ([]automation.ScriptRunRecord, error) {
	lister, ok := s.starter.(AutomationScriptRunLister)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "automation script api is unavailable")
	}
	items, err := lister.AutomationScriptRunList(limit)
	if err != nil {
		return nil, newServiceError(http.StatusInternalServerError, err.Error())
	}
	return items, nil
}
