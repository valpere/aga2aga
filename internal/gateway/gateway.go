package gateway

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/pkg/transport"
)

// Gateway wires the MCP server, Redis transport, PendingMap, and policy
// enforcer. Call New to create one, then Run to start serving.
type Gateway struct {
	server   *mcpsdk.Server
	trans    transport.Transport
	pending  *PendingMap
	enforcer PolicyEnforcer
	cfg      Config
}

// New creates a Gateway with all 4 MCP tools registered. The MCP server is
// ready to accept connections after New returns — call Run to start serving.
func New(t transport.Transport, e PolicyEnforcer, cfg Config) *Gateway {
	srv := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "aga2aga-gateway", Version: "v1"},
		nil,
	)
	g := &Gateway{
		server:   srv,
		trans:    t,
		pending:  NewPendingMap(),
		enforcer: e,
		cfg:      cfg,
	}
	g.registerTools()
	return g
}

// registerTools adds the 4 MCP tools to the server. Called once by New.
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
}

// Run starts pending-map cleanup and serves MCP over the given transport.
// Blocks until ctx is cancelled or a fatal error occurs.
func (g *Gateway) Run(ctx context.Context, mcpTransport mcpsdk.Transport) error {
	g.pending.StartCleanup(ctx, g.cfg.PendingTTL)
	return g.server.Run(ctx, mcpTransport)
}
