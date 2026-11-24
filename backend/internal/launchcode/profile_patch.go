package launchcode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
)

type profilePatcher interface {
	PatchBrowserProfile(profileID string, input browser.ProfilePatchInput) (*browser.Profile, error)
}

// ProfilePatchWriteRequest is separate from ProfileWriteRequest: PUT remains a
// full replacement and PATCH never treats omitted fields as zero values.
type ProfilePatchWriteRequest struct {
	Profile    *browser.ProfilePatchInput `json:"profile"`
	LaunchCode string                     `json:"launchCode,omitempty"`
}

func (s *LaunchServer) patchProfileInternal(profileID string, input browser.ProfilePatchInput) (*browser.Profile, error) {
	if patcher, ok := s.starter.(profilePatcher); ok {
		return patcher.PatchBrowserProfile(profileID, input)
	}
	if s.browserMgr != nil {
		return s.browserMgr.Patch(profileID, input)
	}
	return nil, http.ErrNotSupported
}

// PatchProfile is the common sparse-update operation for MCP and HTTP. The
// profile manager performs the update under its lock, not from a stale
// caller-side copy of the complete profile.
func (s *LaunchServer) PatchProfile(profileID string, input browser.ProfilePatchInput, requestedCode string) (*browser.Profile, string, error) {
	profileID = strings.TrimSpace(profileID)
	previous, status, errMsg := s.profileSnapshotByID(profileID)
	if err := serviceErrorFrom(status, errMsg); err != nil {
		return nil, "", err
	}
	if previous == nil {
		return nil, "", newServiceError(http.StatusNotFound, "profile not found")
	}
	if browser.ProfileRuntimeMutationBlocked(previous) {
		return nil, "", newServiceError(http.StatusConflict, "profile runtime is active; stop it before changing configuration")
	}
	currentCode := strings.TrimSpace(previous.LaunchCode)
	requestedCode = strings.TrimSpace(requestedCode)
	launchCode := currentCode

	// Apply the optional code change first. A rejected code cannot cause an
	// unrelated configuration write. If the sparse update fails, roll back
	// only the code rather than replacing the entire profile.
	if requestedCode != "" {
		launchCode, status, errMsg = s.applyRequestedLaunchCode(profileID, currentCode, requestedCode)
		if err := serviceErrorFrom(status, errMsg); err != nil {
			return nil, "", err
		}
	}
	profile, err := s.patchProfileInternal(profileID, input)
	if err == nil && profile == nil {
		err = fmt.Errorf("profile patch returned nil profile")
	}
	if err != nil {
		if launchCode != currentCode && s.service != nil {
			var restoreErr error
			if currentCode == "" {
				restoreErr = s.service.Remove(profileID)
			} else {
				_, restoreErr = s.service.SetCode(profileID, currentCode)
			}
			if restoreErr != nil {
				logger.New("LaunchServer").Warn("Profile patch launch-code rollback failed",
					logger.F("profile_id", profileID))
				err = errors.Join(err, fmt.Errorf("launch code rollback failed: %w", restoreErr))
			}
		}
		return nil, "", newServiceError(mapProfileWriteErrorStatus(err), err.Error())
	}
	if requestedCode == "" {
		launchCode = s.resolveProfileLaunchCode(profileID, profile.LaunchCode)
	}
	profile.LaunchCode = launchCode
	return profile, launchCode, nil
}

func (s *LaunchServer) handlePatchProfile(w http.ResponseWriter, r *http.Request, profileID string) {
	var req ProfilePatchWriteRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, (1<<20)+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Profile == nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "invalid request body: profile is required",
		})
		return
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok": false, "error": "invalid trailing request data",
		})
		return
	}
	profile, launchCode, err := s.PatchProfile(profileID, *req.Profile, req.LaunchCode)
	if err != nil {
		status := mapProfileWriteErrorStatus(err)
		var serviceErr *ServiceError
		if errors.As(err, &serviceErr) {
			status = serviceErr.Status
		}
		writeJSON(w, status, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.profileWriteSuccessPayload(profile, launchCode, false, true, false))
}
