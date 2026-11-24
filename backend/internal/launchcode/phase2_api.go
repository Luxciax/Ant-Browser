package launchcode

import (
	"fmt"
	"net/http"
	"strings"
)

type BrowserPhase2Inspector interface {
	BrowserControlEnvironmentContract(profileID string) (map[string]interface{}, error)
	BrowserControlLaunchPlanState(profileID string) (map[string]interface{}, error)
	BrowserControlLaunchReceipt(profileID string) (map[string]interface{}, error)
	BrowserControlKernelCapabilities(profileID string) (map[string]interface{}, error)
	BrowserControlRuntimeEvidence(profileID string) (map[string]interface{}, error)
	BrowserControlRuntimeVerify(profileID string) (map[string]interface{}, error)
}

func (s *LaunchServer) phase2Inspector() (BrowserPhase2Inspector, error) {
	if s == nil || s.starter == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, "phase2 inspection is unavailable")
	}
	inspector, ok := s.starter.(BrowserPhase2Inspector)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "phase2 inspection is unavailable")
	}
	return inspector, nil
}

func phase2ServiceError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	status := http.StatusInternalServerError
	switch {
	case strings.Contains(strings.ToLower(message), "not found"),
		strings.Contains(message, "不存在"):
		status = http.StatusNotFound
	case strings.Contains(strings.ToLower(message), "not available"),
		strings.Contains(strings.ToLower(message), "unavailable"),
		strings.Contains(message, "未初始化"):
		status = http.StatusServiceUnavailable
	case strings.Contains(strings.ToLower(message), "required"),
		strings.Contains(message, "不能为空"):
		status = http.StatusBadRequest
	}
	return newServiceError(status, message)
}

func (s *LaunchServer) EnvironmentContract(profileID string) (map[string]interface{}, error) {
	inspector, err := s.phase2Inspector()
	if err != nil {
		return nil, err
	}
	value, err := inspector.BrowserControlEnvironmentContract(profileID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) LaunchPlanState(profileID string) (map[string]interface{}, error) {
	inspector, err := s.phase2Inspector()
	if err != nil {
		return nil, err
	}
	value, err := inspector.BrowserControlLaunchPlanState(profileID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) LaunchReceipt(profileID string) (map[string]interface{}, error) {
	inspector, err := s.phase2Inspector()
	if err != nil {
		return nil, err
	}
	value, err := inspector.BrowserControlLaunchReceipt(profileID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) KernelCapabilities(profileID string) (map[string]interface{}, error) {
	inspector, err := s.phase2Inspector()
	if err != nil {
		return nil, err
	}
	value, err := inspector.BrowserControlKernelCapabilities(profileID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) RuntimeEvidence(profileID string) (map[string]interface{}, error) {
	inspector, err := s.phase2Inspector()
	if err != nil {
		return nil, err
	}
	value, err := inspector.BrowserControlRuntimeEvidence(profileID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) VerifyRuntime(profileID string) (map[string]interface{}, error) {
	inspector, err := s.phase2Inspector()
	if err != nil {
		return nil, err
	}
	value, err := inspector.BrowserControlRuntimeVerify(profileID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) handlePhase2ProfileAction(w http.ResponseWriter, r *http.Request, profileID, action string) bool {
	var (
		value map[string]interface{}
		err error
	)
	switch action {
	case "environment":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method not allowed"})
			return true
		}
		value, err = s.EnvironmentContract(profileID)
	case "launch-state":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method not allowed"})
			return true
		}
		value, err = s.LaunchPlanState(profileID)
	case "launch-receipt":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method not allowed"})
			return true
		}
		value, err = s.LaunchReceipt(profileID)
	case "capabilities":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method not allowed"})
			return true
		}
		value, err = s.KernelCapabilities(profileID)
	case "runtime-evidence":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method not allowed"})
			return true
		}
		value, err = s.RuntimeEvidence(profileID)
	case "runtime-verify":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false, "error": "method not allowed"})
			return true
		}
		value, err = s.VerifyRuntime(profileID)
	default:
		return false
	}
	if err != nil {
		status := http.StatusInternalServerError
		if serviceErr, ok := err.(*ServiceError); ok {
			status = serviceErr.Status
		}
		writeJSON(w, status, map[string]interface{}{"ok": false, "error": err.Error()})
		return true
	}
	if value == nil {
		value = map[string]interface{}{}
	}
	value["ok"] = true
	writeJSON(w, http.StatusOK, value)
	return true
}

var _ = fmt.Sprintf
