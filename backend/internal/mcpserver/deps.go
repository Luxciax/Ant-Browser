package mcpserver

import (
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
)

type InstanceProvider interface {
	ListProfiles() ([]browser.Profile, error)
	FindProfiles(selector launchcode.LaunchSelector) ([]browser.Profile, error)
	FindProfile(selector launchcode.LaunchSelector) (*browser.Profile, error)
	CreateProfile(input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error)
	UpdateProfile(profileID string, input browser.ProfileInput, requestedCode string) (*browser.Profile, string, error)
	DeleteProfile(profileID string) error
	StartProfile(selector launchcode.LaunchSelector, params launchcode.LaunchRequestParams) (*browser.Profile, string, error)
	StopProfile(profileID string) (*browser.Profile, error)
	StatusProfile(profileID string) (*browser.Profile, error)
	OpenRuntimeSession(selector launchcode.LaunchSelector, params launchcode.LaunchRequestParams, timeout time.Duration) (*launchcode.RuntimeSession, error)
	ActiveRuntimeSession() (*launchcode.RuntimeSession, error)
}

type AutomationProvider interface {
	ListScripts() ([]automation.ScriptRecord, error)
	GetScript(scriptID string) (*automation.ScriptRecord, error)
	RunScript(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error)
	ListScriptRuns(limit int) ([]automation.ScriptRunRecord, error)
}

type ProxyProvider interface {
	ListProxies() ([]config.BrowserProxy, error)
	TestProxySpeed(proxyID string) (*launchcode.ProxySpeedResult, error)
	CheckProxyHealth(proxyID string) (*launchcode.ProxyHealthResult, error)
	ListCores() ([]config.BrowserCore, error)
}

type PageProvider interface {
	RunPageSteps(req launchcode.PageRequest) (*launchcode.PageResult, error)
	ClosePageSession(selector launchcode.LaunchSelector) (string, error)
}

type Provider interface {
	InstanceProvider
	AutomationProvider
	ProxyProvider
	PageProvider
}

var _ Provider = (*launchcode.LaunchServer)(nil)
