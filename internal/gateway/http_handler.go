package gateway

import (
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPHTTPHandler returns an http.Handler that serves the given MCP server
// over the streamable HTTP transport.
//
// OAuth discovery paths return 404 Not Found, signalling to MCP clients that
// no OAuth is configured. Without this, the MCP SDK's catch-all handler returns
// 405 for these paths, which some clients (Claude Code) interpret as an auth
// challenge and start an unwanted OAuth flow.
//
// Both exact paths and sub-path variants are handled. RFC 9728 appends the
// resource path to the well-known URL, so a client configured with
// http://host/mcp fetches /.well-known/oauth-protected-resource/mcp, not
// /.well-known/oauth-protected-resource. The {path...} wildcard routes cover
// these sub-path variants; Go 1.22 wildcards require separate exact routes
// since {path...} does not match an empty segment.
func NewMCPHTTPHandler(srv *mcpsdk.Server) http.Handler {
	mcpHandler := mcpsdk.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcpsdk.Server { return srv },
		nil,
	)
	notFound := func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
	mux := http.NewServeMux()
	// Return 404 for all OAuth/OIDC discovery paths (exact + sub-path variants).
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", notFound)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/{path...}", notFound)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", notFound)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server/{path...}", notFound)
	mux.HandleFunc("GET /.well-known/openid-configuration", notFound)
	mux.HandleFunc("GET /.well-known/openid-configuration/{path...}", notFound)
	mux.Handle("/", mcpHandler)
	return mux
}
