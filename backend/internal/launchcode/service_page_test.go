package launchcode

import (
	"errors"
	"net/http"
	"testing"
)

type testPageDriver struct {
	runReq PageRequest
	closed LaunchSelector
}

func (d *testPageDriver) RunPageSteps(req PageRequest) (*PageResult, error) {
	d.runReq = req
	return &PageResult{
		ProfileID: "profile-a",
		OK:        true,
		Reused:    true,
		Steps: []PageStepOutcome{{
			Action: req.Steps[0].Action,
			OK:     true,
			Result: map[string]any{"url": "https://example.com"},
		}},
	}, nil
}

func (d *testPageDriver) ClosePageSession(selector LaunchSelector) (string, error) {
	d.closed = selector
	return selector.ProfileID, nil
}

func TestPageFacadeValidatesStepsBeforeDriver(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	_, err := server.RunPageSteps(PageRequest{})
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) || serviceErr.Status != http.StatusBadRequest {
		t.Fatalf("RunPageSteps error = %v, want bad request ServiceError", err)
	}
}

func TestPageFacadeUnavailableWithoutDriver(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	_, err := server.RunPageSteps(PageRequest{Steps: []PageStep{{Action: "snapshot"}}})
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) || !serviceErr.Unavailable() {
		t.Fatalf("RunPageSteps error = %v, want unavailable ServiceError", err)
	}
}

func TestPageFacadeDelegatesAndNormalizesSelector(t *testing.T) {
	driver := &testPageDriver{}
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetPageDriver(driver)
	result, err := server.RunPageSteps(PageRequest{
		Selector: LaunchSelector{ProfileID: " profile-a "},
		Steps:    []PageStep{{Action: "snapshot"}},
	})
	if err != nil || result == nil || !result.OK {
		t.Fatalf("RunPageSteps = %+v, %v", result, err)
	}
	if driver.runReq.Selector.ProfileID != "profile-a" {
		t.Fatalf("normalized profile id = %q", driver.runReq.Selector.ProfileID)
	}
	profileID, err := server.ClosePageSession(LaunchSelector{ProfileID: " profile-a "})
	if err != nil || profileID != "profile-a" || driver.closed.ProfileID != "profile-a" {
		t.Fatalf("ClosePageSession = %q, %v, selector=%+v", profileID, err, driver.closed)
	}
}
