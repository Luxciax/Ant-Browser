package launchcode

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type PageStep struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

type PageStepOutcome struct {
	Action string         `json:"action"`
	OK     bool           `json:"ok"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type PageRequest struct {
	Selector  LaunchSelector
	Steps     []PageStep
	TimeoutMs int
}

type PageResult struct {
	ProfileID   string                    `json:"profileId"`
	LaunchCode  string                    `json:"launchCode,omitempty"`
	OK          bool                      `json:"ok"`
	Reused      bool                      `json:"reused"`
	Steps       []PageStepOutcome         `json:"steps"`
	Error       string                    `json:"error,omitempty"`
	Screenshots map[string]PageScreenshot `json:"-"`
}

type PageScreenshot struct {
	Data     []byte
	MIMEType string
}

// PageDriver is intentionally transport-neutral. The browser automation
// runtime implements it; HTTP/MCP adapters consume it through LaunchServer.
type PageDriver interface {
	RunPageSteps(req PageRequest) (*PageResult, error)
	ClosePageSession(selector LaunchSelector) (string, error)
}

const pageAPIUnavailable = "page automation api is unavailable"

func (s *LaunchServer) SetPageDriver(driver PageDriver) {
	s.pageMu.Lock()
	s.page = driver
	s.pageMu.Unlock()
}

func (s *LaunchServer) pageDriver() PageDriver {
	s.pageMu.RLock()
	defer s.pageMu.RUnlock()
	return s.page
}

func (s *LaunchServer) RunPageSteps(req PageRequest) (*PageResult, error) {
	if len(req.Steps) == 0 {
		return nil, newServiceError(http.StatusBadRequest, "at least one page step is required")
	}
	for index, step := range req.Steps {
		if strings.TrimSpace(step.Action) == "" {
			return nil, newServiceError(http.StatusBadRequest, fmt.Sprintf("page step %d is missing action", index+1))
		}
	}

	driver := s.pageDriver()
	if driver == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, pageAPIUnavailable)
	}
	req.Selector = normalizeLaunchSelector(req.Selector)
	result, err := driver.RunPageSteps(req)
	if err != nil {
		return nil, asServiceError(err)
	}
	if result == nil {
		return nil, newServiceError(http.StatusInternalServerError, "page automation returned no result")
	}
	return result, nil
}

func (s *LaunchServer) ClosePageSession(selector LaunchSelector) (string, error) {
	driver := s.pageDriver()
	if driver == nil {
		return "", newServiceError(http.StatusServiceUnavailable, pageAPIUnavailable)
	}
	profileID, err := driver.ClosePageSession(normalizeLaunchSelector(selector))
	if err != nil {
		return "", asServiceError(err)
	}
	return profileID, nil
}

func asServiceError(err error) error {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr
	}
	return newServiceError(http.StatusInternalServerError, err.Error())
}
