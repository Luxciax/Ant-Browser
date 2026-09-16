package launchcode

import (
	"errors"
	"net/http"
	"testing"
)

func TestServiceErrorClassifications(t *testing.T) {
	tests := []struct {
		status      int
		wantMissing bool
		wantBusy    bool
		wantDown    bool
	}{
		{status: http.StatusNotFound, wantMissing: true},
		{status: http.StatusConflict, wantBusy: true},
		{status: http.StatusServiceUnavailable, wantDown: true},
	}

	for _, tt := range tests {
		err := NewServiceError(tt.status, "test")
		if err.NotFound() != tt.wantMissing || err.Ambiguous() != tt.wantBusy || err.Unavailable() != tt.wantDown {
			t.Fatalf("status %d classification mismatch: %+v", tt.status, err)
		}
	}
}

func TestServiceFacadePreservesUnavailableProfileCatalog(t *testing.T) {
	server := NewLaunchServer(NewLaunchCodeService(NewMemoryLaunchCodeDAO()), nil, nil, 0)
	_, err := server.ListProfiles()
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("ListProfiles error = %v, want ServiceError", err)
	}
	if !serviceErr.Unavailable() {
		t.Fatalf("ListProfiles status = %d, want service unavailable", serviceErr.Status)
	}
}

func TestServiceFacadeValidatesRuntimeSelectorBeforeLaunch(t *testing.T) {
	server := NewLaunchServer(NewLaunchCodeService(NewMemoryLaunchCodeDAO()), nil, nil, 0)
	_, err := server.OpenRuntimeSession(LaunchSelector{}, LaunchRequestParams{}, 0)
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("OpenRuntimeSession error = %v, want ServiceError", err)
	}
	if serviceErr.Status != http.StatusBadRequest {
		t.Fatalf("OpenRuntimeSession status = %d, want %d", serviceErr.Status, http.StatusBadRequest)
	}
}

func TestServiceFacadeReportsMissingAutomationProvider(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	_, err := server.ListScripts()
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) || !serviceErr.Unavailable() {
		t.Fatalf("ListScripts error = %v, want unavailable ServiceError", err)
	}
}
