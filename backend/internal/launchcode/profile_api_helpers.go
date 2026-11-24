package launchcode

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"ant-chrome/backend/internal/browser"
)

func decodeProfileWriteRequest(r *http.Request) (ProfileWriteRequest, int, string) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		return ProfileWriteRequest{}, http.StatusMethodNotAllowed, "method not allowed"
	}

	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		return ProfileWriteRequest{}, http.StatusBadRequest, "invalid request body"
	}
	if len(body) > maxBody {
		return ProfileWriteRequest{}, http.StatusRequestEntityTooLarge, "request body exceeds limit"
	}
	var req ProfileWriteRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return ProfileWriteRequest{}, http.StatusBadRequest, "invalid request body"
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return ProfileWriteRequest{}, http.StatusBadRequest, "invalid trailing request data"
	}
	if req.Profile == nil {
		return ProfileWriteRequest{}, http.StatusBadRequest, "profile is required"
	}
	if r.Method == http.MethodPut {
		// GET returns an allowlisted view, not a complete replacement input.
		// Require every writable field to be present on PUT, so sending a
		// partial/redacted view cannot silently clear hidden configuration.
		var envelope struct {
			Profile map[string]json.RawMessage `json:"profile"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return ProfileWriteRequest{}, http.StatusBadRequest, "invalid profile object"
		}
		required := []string{
			"profileName", "userDataDir", "coreId", "restoreLastSession",
			"fingerprintArgs", "proxyId", "proxyConfig", "memoryLimitMb",
			"launchArgs", "tags", "keywords", "groupId",
		}
		for _, field := range required {
			value, ok := envelope.Profile[field]
			if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return ProfileWriteRequest{}, http.StatusBadRequest, "PUT requires explicit non-null profile field " + field + "; use PATCH for partial updates"
			}
		}
	}
	return req, http.StatusOK, ""
}

func normalizeProfileInput(input browser.ProfileInput) browser.ProfileInput {
	return browser.ProfileInput{
		ProfileName:        strings.TrimSpace(input.ProfileName),
		UserDataDir:        strings.TrimSpace(input.UserDataDir),
		CoreId:             strings.TrimSpace(input.CoreId),
		RestoreLastSession: browser.NormalizeRestoreLastSessionMode(input.RestoreLastSession),
		FingerprintArgs:    normalizeStringSlice(input.FingerprintArgs),
		ProxyId:            strings.TrimSpace(input.ProxyId),
		ProxyConfig:        strings.TrimSpace(input.ProxyConfig),
		MemoryLimitMB:      input.MemoryLimitMB,
		LaunchArgs:         normalizeStringSlice(input.LaunchArgs),
		Tags:               normalizeStringSlice(input.Tags),
		Keywords:           normalizeStringSlice(input.Keywords),
		GroupId:            strings.TrimSpace(input.GroupId),
	}
}

func profileToInput(profile *browser.Profile) browser.ProfileInput {
	if profile == nil {
		return browser.ProfileInput{}
	}
	return browser.ProfileInput{
		ProfileName:        strings.TrimSpace(profile.ProfileName),
		UserDataDir:        strings.TrimSpace(profile.UserDataDir),
		CoreId:             strings.TrimSpace(profile.CoreId),
		RestoreLastSession: browser.NormalizeRestoreLastSessionMode(profile.RestoreLastSession),
		FingerprintArgs:    append([]string{}, profile.FingerprintArgs...),
		ProxyId:            strings.TrimSpace(profile.ProxyId),
		ProxyConfig:        strings.TrimSpace(profile.ProxyConfig),
		MemoryLimitMB:      profile.MemoryLimitMB,
		LaunchArgs:         append([]string{}, profile.LaunchArgs...),
		Tags:               append([]string{}, profile.Tags...),
		Keywords:           append([]string{}, profile.Keywords...),
		GroupId:            strings.TrimSpace(profile.GroupId),
	}
}

func mergeProfileRuntime(target, runtimeProfile *browser.Profile) {
	if target == nil || runtimeProfile == nil {
		return
	}
	target.RuntimeState = runtimeProfile.RuntimeState
	target.Running = runtimeProfile.Running
	target.DebugPort = runtimeProfile.DebugPort
	target.DebugReady = runtimeProfile.DebugReady
	target.Pid = runtimeProfile.Pid
	target.RuntimeWarning = runtimeProfile.RuntimeWarning
	target.LastError = runtimeProfile.LastError
	target.LastStartAt = runtimeProfile.LastStartAt
	target.LastStopAt = runtimeProfile.LastStopAt
}

func parseProfilePath(path string) (string, string, bool) {
	path = strings.TrimPrefix(path, "/api/profiles/")
	path = strings.Trim(path, "/")
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "", false
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), "", strings.TrimSpace(parts[0]) != ""
	}
	if len(parts) == 2 {
		profileID := strings.TrimSpace(parts[0])
		action := strings.ToLower(strings.TrimSpace(parts[1]))
		if profileID == "" || action == "" {
			return "", "", false
		}
		switch action {
		case "status", "stop",
			"environment", "launch-state", "launch-receipt",
			"capabilities", "runtime-evidence", "runtime-verify":
			return profileID, action, true
		default:
			return "", "", false
		}
	}
	return "", "", false
}

func parseProfilePathID(path string) (string, bool) {
	profileID, action, ok := parseProfilePath(path)
	if !ok || action != "" {
		return "", false
	}
	return profileID, true
}

func mapProfileWriteErrorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case msg == strings.ToLower(strings.TrimSpace(http.ErrNotSupported.Error())):
		return http.StatusServiceUnavailable
	case strings.Contains(msg, "profile not found"):
		return http.StatusNotFound
	case strings.Contains(msg, "running profile cannot be deleted"), strings.Contains(msg, "profile runtime is "):
		return http.StatusConflict
	case strings.Contains(msg, "profile name is required"):
		return http.StatusBadRequest
	case strings.Contains(msg, "launch code already exists"):
		return http.StatusConflict
	case strings.Contains(msg, "launch code format invalid"),
		strings.Contains(msg, "launch code must be"):
		return http.StatusBadRequest
	case strings.Contains(msg, "proxy id not found"),
		strings.Contains(msg, "代理id不存在"):
		return http.StatusBadRequest
	case strings.Contains(msg, "实例数量已达上限"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
