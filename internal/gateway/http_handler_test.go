package gateway_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/valpere/aga2aga/internal/gateway"
)

// TestNewMCPHTTPHandler_OAuthDiscovery checks that OAuth discovery paths return
// 404 Not Found, so MCP clients know no OAuth is configured. Without this, the
// MCP SDK's catch-all handler returns 405 for these paths, which Claude Code
// interprets as an authentication challenge and starts an unwanted OAuth flow.
func TestNewMCPHTTPHandler_OAuthDiscovery(t *testing.T) {
	g := gateway.New(noopTransport{}, noopEnforcer{}, nil, gateway.NewNoopMessageLogger(), gateway.NewNoopLimitEnforcer(), gateway.DefaultConfig())
	srv := httptest.NewServer(gateway.NewMCPHTTPHandler(g.Server()))
	defer srv.Close()

	oauthPaths := []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-authorization-server",
	}
	for _, path := range oauthPaths {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s returned %d; want 404", path, resp.StatusCode)
		}
	}
}

// TestNewMCPHTTPHandler_MCPInitialize checks that a valid MCP initialize request
// succeeds, confirming the MCP handler is still wired behind the mux.
func TestNewMCPHTTPHandler_MCPInitialize(t *testing.T) {
	g := gateway.New(noopTransport{}, noopEnforcer{}, nil, gateway.NewNoopMessageLogger(), gateway.NewNoopLimitEnforcer(), gateway.DefaultConfig())
	srv := httptest.NewServer(gateway.NewMCPHTTPHandler(g.Server()))
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST / returned %d; want 200\nbody: %s", resp.StatusCode, b)
	}
}
