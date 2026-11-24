package mcpserver

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName = "ant-browser"
	DefaultPath = "/mcp"
	sessionTimeout = 30 * time.Minute
	toolCount = 49
)

type Server struct {
	provider Provider
	version string
}

func New(provider Provider, version string) *Server {
	if version == "" {
		version = "unknown"
	}
	return &Server{provider: provider, version: version}
}

type scopeProvider interface { AllowsScope(scope string) bool }

func providerAllowsScope(provider interface{}, scope string) bool {
	if scoped, ok := provider.(scopeProvider); ok {
		return scoped.AllowsScope(scope)
	}
	return true
}

func (s *Server) buildMCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: ServerName, Version: s.version}, nil)
	if providerAllowsScope(s.provider, "environment-read") || providerAllowsScope(s.provider, "environment-admin") || providerAllowsScope(s.provider, "page-action") || providerAllowsScope(s.provider, "page-read") {
		registerInstanceTools(srv, s.provider)
	}
	if providerAllowsScope(s.provider, "environment-read") || providerAllowsScope(s.provider, "script") {
		registerAutomationTools(srv, s.provider)
	}
	if providerAllowsScope(s.provider, "environment-read") || providerAllowsScope(s.provider, "environment-admin") || providerAllowsScope(s.provider, "script") {
		registerWorkflowTools(srv, s.provider)
	}
	if providerAllowsScope(s.provider, "environment-read") || providerAllowsScope(s.provider, "environment-admin") {
		registerProxyTools(srv, s.provider)
	}
	if providerAllowsScope(s.provider, "page-read") || providerAllowsScope(s.provider, "page-action") || providerAllowsScope(s.provider, "script") {
		registerPageTools(srv, s.provider)
	}
	if providerAllowsScope(s.provider, "environment-read") || providerAllowsScope(s.provider, "environment-admin") {
		registerPhase5ControlTools(srv, s.provider)
	}
	if providerAllowsScope(s.provider, "page-read") {
		registerNetworkObservationTools(srv, s.provider)
	}
	return srv
}

type Options struct {
	Stateless bool
}

func (s *Server) Handler(opts Options) http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.buildMCPServer() },
		&mcp.StreamableHTTPOptions{Stateless: opts.Stateless, SessionTimeout: sessionTimeout},
	)
}

func (s *Server) MCPServer() *mcp.Server { return s.buildMCPServer() }
func ToolCount() int { return toolCount }
