package mcpserver

import (
	"context"
	"testing"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeProvider struct{}

func (fakeProvider) ListProfiles() ([]browser.Profile, error) { return nil, nil }
func (fakeProvider) FindProfiles(launchcode.LaunchSelector) ([]browser.Profile, error) { return nil, nil }
func (fakeProvider) FindProfile(launchcode.LaunchSelector) (*browser.Profile, error) { return &browser.Profile{ProfileId: "p1", ProfileName: "test"}, nil }
func (fakeProvider) CreateProfile(input browser.ProfileInput, code string) (*browser.Profile, string, error) { return &browser.Profile{ProfileId: "p1", ProfileName: input.ProfileName, LaunchCode: code}, code, nil }
func (fakeProvider) UpdateProfile(id string, input browser.ProfileInput, code string) (*browser.Profile, string, error) { return &browser.Profile{ProfileId: id, ProfileName: input.ProfileName, LaunchCode: code}, code, nil }
func (fakeProvider) DeleteProfile(string) error { return nil }
func (fakeProvider) StartProfile(launchcode.LaunchSelector, launchcode.LaunchRequestParams) (*browser.Profile, string, error) { return &browser.Profile{ProfileId: "p1", Running: true}, "CODE1", nil }
func (fakeProvider) StopProfile(id string) (*browser.Profile, error) { return &browser.Profile{ProfileId: id}, nil }
func (fakeProvider) StatusProfile(id string) (*browser.Profile, error) { return &browser.Profile{ProfileId: id, Running: true}, nil }
func (fakeProvider) OpenRuntimeSession(launchcode.LaunchSelector, launchcode.LaunchRequestParams, time.Duration) (*launchcode.RuntimeSession, error) { return &launchcode.RuntimeSession{Profile: &browser.Profile{ProfileId: "p1"}}, nil }
func (fakeProvider) ActiveRuntimeSession() (*launchcode.RuntimeSession, error) { return nil, nil }
func (fakeProvider) ListScripts() ([]automation.ScriptRecord, error) { return nil, nil }
func (fakeProvider) GetScript(string) (*automation.ScriptRecord, error) { return &automation.ScriptRecord{}, nil }
func (fakeProvider) RunScript(automation.ScriptRunRequest) (*automation.ScriptRunRecord, error) { return &automation.ScriptRunRecord{Status: "success"}, nil }
func (fakeProvider) ListScriptRuns(int) ([]automation.ScriptRunRecord, error) { return nil, nil }
func (fakeProvider) ListProxies() ([]config.BrowserProxy, error) { return nil, nil }
func (fakeProvider) TestProxySpeed(id string) (*launchcode.ProxySpeedResult, error) { return &launchcode.ProxySpeedResult{ProxyID: id, Ok: true}, nil }
func (fakeProvider) CheckProxyHealth(id string) (*launchcode.ProxyHealthResult, error) { return &launchcode.ProxyHealthResult{ProxyID: id, Ok: true}, nil }
func (fakeProvider) ListCores() ([]config.BrowserCore, error) { return nil, nil }
func (fakeProvider) RunPageSteps(req launchcode.PageRequest) (*launchcode.PageResult, error) {
	steps := make([]launchcode.PageStepOutcome, 0, len(req.Steps))
	for _, step := range req.Steps {
		steps = append(steps, launchcode.PageStepOutcome{Action: step.Action, OK: true, Result: map[string]any{"url":"https://example.com","title":"Example"}})
	}
	return &launchcode.PageResult{ProfileID:"p1",OK:true,Steps:steps}, nil
}
func (fakeProvider) ClosePageSession(launchcode.LaunchSelector) (string, error) { return "p1", nil }

func newTestClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server := New(fakeProvider{}, "test").MCPServer()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil { t.Fatalf("server connect: %v", err) }
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil { t.Fatalf("client connect: %v", err) }
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestToolsAreRegistered(t *testing.T) {
	session := newTestClient(t)
	result, err := session.ListTools(context.Background(), nil)
	if err != nil { t.Fatalf("list tools: %v", err) }
	want := map[string]bool{
		"ant_instance_list":true,"ant_instance_get":true,"ant_instance_create":true,"ant_instance_update":true,"ant_instance_delete":true,
		"ant_instance_start":true,"ant_instance_stop":true,"ant_runtime_session":true,"ant_runtime_status":true,"ant_runtime_active":true,
		"ant_script_list":true,"ant_script_get":true,"ant_script_run":true,"ant_script_runs":true,
		"ant_proxy_list":true,"ant_proxy_test_speed":true,"ant_proxy_check_health":true,"ant_core_list":true,
		"ant_page_goto":true,"ant_page_wait":true,"ant_page_tabs":true,"ant_page_click":true,"ant_page_fill":true,"ant_page_press":true,
		"ant_page_select":true,"ant_page_snapshot":true,"ant_page_screenshot":true,"ant_page_extract":true,"ant_page_evaluate":true,"ant_page_release":true,
	}
	for _, tool := range result.Tools { delete(want, tool.Name) }
	if len(want) != 0 { t.Fatalf("missing tools: %v", want) }
	if len(result.Tools) != ToolCount() { t.Fatalf("tool count=%d ToolCount=%d", len(result.Tools), ToolCount()) }
}

func TestProxyProtocolExtractionDoesNotExposeConfig(t *testing.T) {
	node := toProxyNode(config.BrowserProxy{ProxyId:"p1",ProxyName:"node",ProxyConfig:"socks5://user:pass@example.com:1080"})
	if node.Protocol != "socks5" { t.Fatalf("protocol=%q", node.Protocol) }
}
