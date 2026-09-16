package backend

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/launchcode"
)

const (
	pageStepDefaultTimeout = 30 * time.Second
	pageStepMaxTimeout     = 5 * time.Minute
	pageSessionOpenTimeout = 60 * time.Second
)

type appPageDriver struct {
	app *App
}

func newAppPageDriver(app *App) launchcode.PageDriver {
	return appPageDriver{app: app}
}

func (d appPageDriver) RunPageSteps(req launchcode.PageRequest) (*launchcode.PageResult, error) {
	if d.app == nil {
		return nil, launchcode.NewServiceError(http.StatusServiceUnavailable, "page automation api is unavailable")
	}
	ctx := context.Background()
	if _, err := d.app.ensureAutomationReady(ctx); err != nil {
		return nil, launchcode.NewServiceError(http.StatusServiceUnavailable, err.Error())
	}
	profileID, launchCode, err := d.app.resolvePageTarget(req.Selector)
	if err != nil {
		return nil, err
	}
	baseURL, authHeader, authValue, err := d.app.automationDemoEndpoint()
	if err != nil {
		return nil, launchcode.NewServiceError(http.StatusServiceUnavailable, err.Error())
	}
	artifactDir := filepath.Join(d.app.automationArtifactsRootDir(), "page-session", profileID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, launchcode.NewServiceError(http.StatusInternalServerError, "create page artifact directory: "+err.Error())
	}

	commands := make([]automation.PageCommand, 0, len(req.Steps))
	for _, step := range req.Steps {
		commands = append(commands, automation.PageCommand{Action: step.Action, Args: step.Args})
	}
	outcome, err := d.app.automationMgr.RunPageCommands(ctx, automation.PageCommandRequest{
		ProfileID:        profileID,
		Selector:         map[string]any{"profileId": profileID},
		Commands:         commands,
		LaunchBaseURL:    baseURL,
		LaunchAuthHeader: authHeader,
		LaunchAuthValue:  authValue,
		ArtifactDir:      artifactDir,
		Timeout:          pageStepTimeout(req.TimeoutMs),
	})
	if err != nil {
		return nil, launchcode.NewServiceError(http.StatusInternalServerError, err.Error())
	}

	result := &launchcode.PageResult{
		ProfileID:  profileID,
		LaunchCode: launchCode,
		OK:         outcome.OK,
		Reused:     outcome.Reused,
		Error:      outcome.Error,
		Steps:      make([]launchcode.PageStepOutcome, 0, len(outcome.Results)),
	}
	for _, step := range outcome.Results {
		result.Steps = append(result.Steps, launchcode.PageStepOutcome{
			Action: step.Action,
			OK:     step.OK,
			Result: step.Result,
			Error:  step.Error,
		})
	}
	collectPageScreenshots(result)
	return result, nil
}

func (d appPageDriver) ClosePageSession(selector launchcode.LaunchSelector) (string, error) {
	if d.app == nil || d.app.automationMgr == nil {
		return "", launchcode.NewServiceError(http.StatusServiceUnavailable, "page automation api is unavailable")
	}
	profileID, err := d.app.resolvePageTargetWithoutStart(selector)
	if err != nil {
		return "", err
	}
	d.app.automationMgr.ClosePageSession(profileID)
	return profileID, nil
}

func collectPageScreenshots(result *launchcode.PageResult) {
	if result == nil {
		return
	}
	for index := range result.Steps {
		step := &result.Steps[index]
		if step.Result == nil {
			continue
		}
		rawPath, _ := step.Result["path"].(string)
		rawPath = strings.TrimSpace(rawPath)
		if rawPath == "" {
			continue
		}
		mimeType, _ := step.Result["mimeType"].(string)
		if strings.TrimSpace(mimeType) == "" {
			mimeType = "image/jpeg"
		}
		name := filepath.Base(rawPath)
		data, err := os.ReadFile(rawPath)
		if err != nil {
			step.Result["screenshotError"] = "read screenshot: " + err.Error()
			delete(step.Result, "path")
			continue
		}
		_ = os.Remove(rawPath)
		if result.Screenshots == nil {
			result.Screenshots = make(map[string]launchcode.PageScreenshot)
		}
		result.Screenshots[name] = launchcode.PageScreenshot{Data: data, MIMEType: mimeType}
		delete(step.Result, "path")
		step.Result["screenshot"] = name
		step.Result["bytes"] = len(data)
	}
}

func pageStepTimeout(timeoutMs int) time.Duration {
	if timeoutMs <= 0 {
		return pageStepDefaultTimeout
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout > pageStepMaxTimeout {
		return pageStepMaxTimeout
	}
	return timeout
}

func (a *App) ensureAutomationReady(ctx context.Context) (automation.RuntimeState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.automationMgr == nil {
		return automation.RuntimeState{}, fmt.Errorf("automation runtime manager is not initialized")
	}
	if a.config == nil || !a.config.Automation.Enabled {
		return automation.RuntimeState{}, fmt.Errorf("automation support is disabled")
	}
	if err := ctx.Err(); err != nil {
		return automation.RuntimeState{}, err
	}
	if err := a.automationMgr.EnsureInstalled(ctx); err != nil {
		return automation.RuntimeState{}, err
	}
	state := a.automationMgr.CurrentState()
	if !state.Ready {
		return automation.RuntimeState{}, fmt.Errorf("automation runtime is not ready")
	}
	return state, nil
}

func (a *App) resolvePageTarget(selector launchcode.LaunchSelector) (string, string, error) {
	if a.launchServer == nil {
		return "", "", launchcode.NewServiceError(http.StatusServiceUnavailable, "launch server is not initialized")
	}
	if selector.IsEmpty() {
		session, err := a.launchServer.ActiveRuntimeSession()
		if err != nil {
			return "", "", err
		}
		if session == nil || session.Profile == nil {
			return "", "", launchcode.NewServiceError(http.StatusBadRequest, "no active profile; selector is required")
		}
		return session.Profile.ProfileId, session.LaunchCode, nil
	}
	session, err := a.launchServer.OpenRuntimeSession(
		selector,
		launchcode.LaunchRequestParams{SkipDefaultStartURLs: true},
		pageSessionOpenTimeout,
	)
	if err != nil {
		return "", "", err
	}
	if session == nil || session.Profile == nil {
		return "", "", launchcode.NewServiceError(http.StatusServiceUnavailable, "runtime session is not available")
	}
	if !session.Ready {
		return "", "", launchcode.NewServiceError(http.StatusServiceUnavailable, "profile debug endpoint is not ready")
	}
	return session.Profile.ProfileId, session.LaunchCode, nil
}

func (a *App) resolvePageTargetWithoutStart(selector launchcode.LaunchSelector) (string, error) {
	if a.launchServer == nil {
		return "", launchcode.NewServiceError(http.StatusServiceUnavailable, "launch server is not initialized")
	}
	if selector.IsEmpty() {
		session, err := a.launchServer.ActiveRuntimeSession()
		if err != nil {
			return "", err
		}
		if session == nil || session.Profile == nil {
			return "", launchcode.NewServiceError(http.StatusBadRequest, "no active profile; selector is required")
		}
		return session.Profile.ProfileId, nil
	}
	profile, err := a.launchServer.FindProfile(selector)
	if err != nil {
		return "", err
	}
	if profile == nil {
		return "", launchcode.NewServiceError(http.StatusNotFound, "profile not found")
	}
	return profile.ProfileId, nil
}

var _ launchcode.PageDriver = appPageDriver{}
