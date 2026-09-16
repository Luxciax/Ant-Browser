package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ant-chrome/backend"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIsMCPStdioInvocation(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--mcp-stdio"}, true},
		{[]string{"-other", "  --mcp-stdio  "}, true},
		{[]string{"--mcp"}, false},
		{[]string{"--mcp-stdio-extra"}, false},
	}
	for _, test := range tests {
		if got := isMCPStdioInvocation(test.args); got != test.want {
			t.Errorf("isMCPStdioInvocation(%v)=%v want %v", test.args, got, test.want)
		}
	}
}

func newBridgeUpstream(t *testing.T, expectedHeader, expectedValue string) string {
	t.Helper()
	type echoInput struct { Text string `json:"text"` }
	type echoOutput struct { Echo string `json:"echo"` }
	build := func() *mcp.Server {
		server := mcp.NewServer(&mcp.Implementation{Name: "bridge-upstream", Version: "v1"}, nil)
		mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo"}, func(_ context.Context, _ *mcp.CallToolRequest, input echoInput) (*mcp.CallToolResult, echoOutput, error) {
			return nil, echoOutput{Echo: input.Text}, nil
		})
		return server
	}
	handler := mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		if expectedHeader != "" && request.Header.Get(expectedHeader) != expectedValue {
			t.Errorf("bridge auth header mismatch")
		}
		return build()
	}, nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func TestBridgeForwardsMCPMessages(t *testing.T) {
	const headerName = "X-Test-Authorization"
	const headerValue = "bridge-fixture"
	upstreamURL := newBridgeUpstream(t, headerName, headerValue)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientTransport, bridgeTransport := mcp.NewInMemoryTransports()
	go func() {
		_ = bridgeToHTTP(ctx, backend.MCPClientEndpoint{
			URL: upstreamURL, AuthHeader: headerName, AuthValue: headerValue,
		}, bridgeTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "bridge-test", Version: "v1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil { t.Fatalf("connect: %v", err) }
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil { t.Fatalf("list tools: %v", err) }
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", tools.Tools)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"echo", Arguments:map[string]any{"text":"hello"}})
	if err != nil || result.IsError { t.Fatalf("echo call failed: err=%v result=%+v", err, result) }
}

func TestBridgeRejectsUnreachableEndpoint(t *testing.T) {
	server := httptest.NewServer(nil)
	deadURL := server.URL
	server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, bridgeTransport := mcp.NewInMemoryTransports()
	if err := bridgeToHTTP(ctx, backend.MCPClientEndpoint{URL: deadURL}, bridgeTransport); err == nil {
		t.Fatal("expected connection failure")
	}
}
