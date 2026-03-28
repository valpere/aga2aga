package gateway

import (
	"context"
	"crypto/subtle"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/pkg/transport"
)

// Gateway wires the MCP server, Redis transport, PendingMap, policy enforcer,
// and optional agent authenticator. Call New to create one, then Run to serve.
type Gateway struct {
	server   *mcpsdk.Server
	trans    transport.Transport
	pending  *PendingMap
	enforcer PolicyEnforcer
	auth     AgentAuthenticator
	cfg      Config
}

// New creates a Gateway with all 6 MCP tools registered. auth may be nil to
// disable agent key authentication (legacy/optional mode). The MCP server is
// ready to accept connections after New returns — call Run to start serving.
func New(t transport.Transport, e PolicyEnforcer, auth AgentAuthenticator, cfg Config) *Gateway {
	srv := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "aga2aga-gateway", Version: "v1"},
		nil,
	)
	g := &Gateway{
		server:   srv,
		trans:    t,
		pending:  NewPendingMap(),
		enforcer: e,
		auth:     auth,
		cfg:      cfg,
	}
	g.registerTools()
	return g
}

// authenticateAgent validates the raw API key against the claimed agent ID.
// If auth is nil (legacy mode), it returns nil immediately.
func (g *Gateway) authenticateAgent(ctx context.Context, claimedAgent, rawKey string) error {
	if g.auth == nil {
		return nil
	}
	boundID, err := g.auth.Authenticate(ctx, rawKey)
	if err != nil {
		return fmt.Errorf("gateway: authentication failed: %w", err)
	}
	// SECURITY: constant-time comparison prevents timing oracle on the bound agent ID (CWE-208).
	if subtle.ConstantTimeCompare([]byte(boundID), []byte(claimedAgent)) != 1 {
		return fmt.Errorf("gateway: api_key is not bound to the claimed agent")
	}
	return nil
}

// registerTools adds the 6 MCP tools to the server. Called once by New.
func (g *Gateway) registerTools() {
	mcpsdk.AddTool(g.server,
		&mcpsdk.Tool{Name: "get_task", Description: "Fetch the next task from the agent's task stream."},
		g.handleGetTask,
	)
	mcpsdk.AddTool(g.server,
		&mcpsdk.Tool{Name: "complete_task", Description: "Report successful task completion."},
		g.handleCompleteTask,
	)
	mcpsdk.AddTool(g.server,
		&mcpsdk.Tool{Name: "fail_task", Description: "Report task failure."},
		g.handleFailTask,
	)
	mcpsdk.AddTool(g.server,
		&mcpsdk.Tool{Name: "heartbeat", Description: "Health check — returns ok."},
		g.handleHeartbeat,
	)
	mcpsdk.AddTool(g.server,
		&mcpsdk.Tool{Name: "send_message", Description: "Send a free-form message to another agent."},
		g.handleSendMessage,
	)
	mcpsdk.AddTool(g.server,
		&mcpsdk.Tool{Name: "receive_message", Description: "Fetch the next message from the agent's message stream."},
		g.handleReceiveMessage,
	)
}

// Run starts pending-map cleanup and serves MCP over the given transport.
// Blocks until ctx is cancelled or a fatal error occurs.
func (g *Gateway) Run(ctx context.Context, mcpTransport mcpsdk.Transport) error {
	g.pending.StartCleanup(ctx, g.cfg.PendingTTL)
	return g.server.Run(ctx, mcpTransport)
}

// Server returns the underlying MCP server. Use this when integrating with
// alternative transports (e.g. StreamableHTTPHandler) that require direct
// access to the server rather than using Run.
func (g *Gateway) Server() *mcpsdk.Server {
	return g.server
}

// StartCleanup starts the PendingMap eviction goroutine. It is called
// automatically by Run; call it explicitly when using an alternative
// transport that bypasses Run.
func (g *Gateway) StartCleanup(ctx context.Context) {
	g.pending.StartCleanup(ctx, g.cfg.PendingTTL)
}
