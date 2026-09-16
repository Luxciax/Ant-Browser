package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"ant-chrome/backend"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpStdioFlag = "--mcp-stdio"
	mcpBridgeConnectTimeout = 10 * time.Second
)

func isMCPStdioInvocation(args []string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == mcpStdioFlag {
			return true
		}
	}
	return false
}

func runMCPStdioBridge(appRoot string) int {
	cfg, err := backend.LoadConfig(backend.ResolveRuntimePath(appRoot, "config.yaml"))
	if err != nil {
		cfg = backend.DefaultConfig()
	}
	endpoint := backend.ResolveMCPClientEndpoint(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := bridgeStdioToHTTP(ctx, endpoint); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Ant Browser MCP stdio bridge stopped: %v\n", err)
		return 1
	}
	return 0
}

func bridgeStdioToHTTP(ctx context.Context, endpoint backend.MCPClientEndpoint) error {
	return bridgeToHTTP(ctx, endpoint, &mcp.StdioTransport{})
}

func bridgeToHTTP(ctx context.Context, endpoint backend.MCPClientEndpoint, downstreamTransport mcp.Transport) error {
	if err := probeMCPEndpoint(ctx, endpoint.URL); err != nil {
		return err
	}

	httpTransport := &mcp.StreamableClientTransport{
		Endpoint:   endpoint.URL,
		HTTPClient: newMCPBridgeHTTPClient(endpoint.AuthHeader, endpoint.AuthValue),
	}
	connectCtx, cancel := context.WithTimeout(ctx, mcpBridgeConnectTimeout)
	defer cancel()
	upstream, err := httpTransport.Connect(connectCtx)
	if err != nil {
		return fmt.Errorf("connect Ant Browser MCP endpoint %s: %w", endpoint.URL, err)
	}
	defer upstream.Close()

	downstream, err := downstreamTransport.Connect(ctx)
	if err != nil {
		return fmt.Errorf("initialize stdio MCP transport: %w", err)
	}
	defer downstream.Close()
	return pumpMCPBothDirections(ctx, downstream, upstream)
}

func probeMCPEndpoint(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid MCP endpoint %q: %w", rawURL, err)
	}
	host := parsed.Host
	if parsed.Port() == "" {
		host = net.JoinHostPort(parsed.Hostname(), "80")
	}
	dialCtx, cancel := context.WithTimeout(ctx, mcpBridgeConnectTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", host)
	if err != nil {
		return fmt.Errorf("cannot reach Ant Browser MCP endpoint %s: %w; start Ant Browser first", rawURL, err)
	}
	return conn.Close()
}

func pumpMCPBothDirections(ctx context.Context, downstream, upstream mcp.Connection) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2)
	go func() { errs <- pumpMCP(ctx, downstream, upstream) }()
	go func() { errs <- pumpMCP(ctx, upstream, downstream) }()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func pumpMCP(ctx context.Context, from, to mcp.Connection) error {
	for {
		message, err := from.Read(ctx)
		if err != nil {
			return err
		}
		if err := to.Write(ctx, message); err != nil {
			return err
		}
	}
}

func newMCPBridgeHTTPClient(header, value string) *http.Client {
	if strings.TrimSpace(header) == "" || strings.TrimSpace(value) == "" {
		return http.DefaultClient
	}
	return &http.Client{Transport: &mcpAuthRoundTripper{header: header, value: value, base: http.DefaultTransport}}
}

type mcpAuthRoundTripper struct {
	header string
	value  string
	base   http.RoundTripper
}

func (rt *mcpAuthRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	cloned.Header.Set(rt.header, rt.value)
	return rt.base.RoundTrip(cloned)
}
