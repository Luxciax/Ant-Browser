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
	toolCount = 30
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

func (s *Server) buildMCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: ServerName, Version: s.version}, nil)
	registerInstanceTools(srv, s.provider)
	registerAutomationTools(srv, s.provider)
	registerProxyTools(srv, s.provider)
	registerPageTools(srv, s.provider)
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
