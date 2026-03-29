package gateway

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPHTTPHandler returns an http.Handler that serves the given MCP server
// over the streamable HTTP transport.
//
// OAuth discovery paths (/.well-known/oauth-protected-resource and
// /.well-known/oauth-authorization-server) return 404 Not Found, signalling
// to MCP clients that no OAuth is configured. Without this, the MCP SDK's
// catch-all handler returns 405 for these paths, which some clients (Claude
// Code) interpret as an authentication challenge and start an unwanted OAuth
// flow, causing connection failures even when the server requires no auth.
func NewMCPHTTPHandler(srv *mcpsdk.Server) http.Handler {
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server { return srv },
		nil,
	)
	mux := http.NewServeMux()
	// Intercept OAuth discovery before the MCP catch-all. Returning 404 tells
	// clients that this server has no OAuth authorization server — they should
	// connect without OAuth.
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.Handle("/", mcpHandler)
	return mux
}
