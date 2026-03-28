package gateway

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/pkg/admin"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
)

// --- input/output types for MCP tool handlers ----------------------------

type getTaskIn struct {
	Agent  string `json:"agent"`
	APIKey string `json:"api_key"`
}

type getTaskOut struct {
	TaskID string `json:"task_id"`
	Body   string `json:"body"`
}

type completeTaskIn struct {
	TaskID string `json:"task_id"`
	Agent  string `json:"agent"`
	Result string `json:"result"`
	APIKey string `json:"api_key"`
}

type completeTaskOut struct {
	Status string `json:"status"`
}

type failTaskIn struct {
	TaskID string `json:"task_id"`
	Agent  string `json:"agent"`
	Error  string `json:"error"`
	APIKey string `json:"api_key"`
}

type failTaskOut struct {
	Status string `json:"status"`
}

type heartbeatIn struct {
	Agent  string `json:"agent"`
	APIKey string `json:"api_key"`
}

type heartbeatOut struct {
	Status string `json:"status"`
}

// --- tool handlers --------------------------------------------------------

func (g *Gateway) handleGetTask(ctx context.Context, _ *mcpsdk.CallToolRequest, in getTaskIn) (*mcpsdk.CallToolResult, getTaskOut, error) {
	if !admin.IsValidAgentID(in.Agent) {
		return nil, getTaskOut{}, fmt.Errorf("gateway: invalid agent id %q", in.Agent)
	}

	// Verify the presented API key is bound to the claimed agent ID.
	// Verified by AgentAuthenticator when configured; full Ed25519 in Phase 3. CWE-287.
	if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
		return nil, getTaskOut{}, err
	}

	allowed, err := g.enforcer.Allowed(ctx, in.Agent, "orchestrator")
	if err != nil {
		return nil, getTaskOut{}, fmt.Errorf("gateway: policy check: %w", err)
	}
	if !allowed {
		return nil, getTaskOut{}, fmt.Errorf("gateway: agent %q not allowed", in.Agent)
	}

	topic := "agent.tasks." + in.Agent
	ch, err := g.trans.Subscribe(ctx, topic)
	if err != nil {
		return nil, getTaskOut{}, fmt.Errorf("gateway: subscribe %q: %w", topic, err)
	}

	tctx, cancel := context.WithTimeout(ctx, g.cfg.TaskReadTimeout)
	defer cancel()

	select {
	case delivery, ok := <-ch:
		if !ok {
			return nil, getTaskOut{}, nil
		}
		// SECURITY: taskID is the transport-layer entry ID assigned by Redis, not
		// Envelope.ID or any other document field — per PendingMap security contract.
		taskID := delivery.MsgID
		g.pending.Store(taskID, topic, delivery.MsgID)
		return nil, getTaskOut{TaskID: taskID, Body: delivery.Doc.Body}, nil
	case <-tctx.Done():
		return nil, getTaskOut{}, nil
	}
}

func (g *Gateway) handleCompleteTask(ctx context.Context, _ *mcpsdk.CallToolRequest, in completeTaskIn) (*mcpsdk.CallToolResult, completeTaskOut, error) {
	if !admin.IsValidAgentID(in.Agent) {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: invalid agent id %q", in.Agent)
	}

	// Verified by AgentAuthenticator when configured; full Ed25519 in Phase 3. CWE-287.
	if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
		return nil, completeTaskOut{}, err
	}

	allowed, err := g.enforcer.Allowed(ctx, in.Agent, "orchestrator")
	if err != nil {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: policy check: %w", err)
	}
	if !allowed {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: agent %q not allowed", in.Agent)
	}

	if len(in.Result) > document.MaxDocumentBytes {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: result body exceeds maximum size")
	}

	topic, msgID, ok := g.pending.LoadAndDelete(in.TaskID)
	if !ok {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: unknown task_id %q", in.TaskID)
	}

	doc, err := document.NewBuilder(protocol.TaskResult).
		ID(uuid.New().String()).
		From(g.cfg.AgentID).
		To(in.Agent).
		ExecID(in.TaskID).
		Field("step", "result").
		InReplyTo(in.TaskID).
		Body(in.Result).
		Build()
	if err != nil {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: build result doc: %w", err)
	}

	if err := g.trans.Publish(ctx, "agent.events.completed", doc); err != nil {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: publish result: %w", err)
	}

	if err := g.trans.Ack(ctx, topic, msgID); err != nil {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: ack task: %w", err)
	}

	return nil, completeTaskOut{Status: "ok"}, nil
}

func (g *Gateway) handleFailTask(ctx context.Context, _ *mcpsdk.CallToolRequest, in failTaskIn) (*mcpsdk.CallToolResult, failTaskOut, error) {
	if !admin.IsValidAgentID(in.Agent) {
		return nil, failTaskOut{}, fmt.Errorf("gateway: invalid agent id %q", in.Agent)
	}

	// Verified by AgentAuthenticator when configured; full Ed25519 in Phase 3. CWE-287.
	if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
		return nil, failTaskOut{}, err
	}

	allowed, err := g.enforcer.Allowed(ctx, in.Agent, "orchestrator")
	if err != nil {
		return nil, failTaskOut{}, fmt.Errorf("gateway: policy check: %w", err)
	}
	if !allowed {
		return nil, failTaskOut{}, fmt.Errorf("gateway: agent %q not allowed", in.Agent)
	}

	if len(in.Error) > document.MaxDocumentBytes {
		return nil, failTaskOut{}, fmt.Errorf("gateway: error body exceeds maximum size")
	}

	topic, msgID, ok := g.pending.LoadAndDelete(in.TaskID)
	if !ok {
		return nil, failTaskOut{}, fmt.Errorf("gateway: unknown task_id %q", in.TaskID)
	}

	doc, err := document.NewBuilder(protocol.TaskFail).
		ID(uuid.New().String()).
		From(g.cfg.AgentID).
		To(in.Agent).
		ExecID(in.TaskID).
		Field("step", "fail").
		InReplyTo(in.TaskID).
		Body(in.Error).
		Build()
	if err != nil {
		return nil, failTaskOut{}, fmt.Errorf("gateway: build fail doc: %w", err)
	}

	if err := g.trans.Publish(ctx, "agent.events.failed", doc); err != nil {
		return nil, failTaskOut{}, fmt.Errorf("gateway: publish fail: %w", err)
	}

	if err := g.trans.Ack(ctx, topic, msgID); err != nil {
		return nil, failTaskOut{}, fmt.Errorf("gateway: ack task: %w", err)
	}

	return nil, failTaskOut{Status: "ok"}, nil
}

func (g *Gateway) handleHeartbeat(ctx context.Context, _ *mcpsdk.CallToolRequest, in heartbeatIn) (*mcpsdk.CallToolResult, heartbeatOut, error) {
	// Verified by AgentAuthenticator when configured; full Ed25519 in Phase 3. CWE-287.
	// When auth is enabled, agent must be identified — an unauthenticated probe cannot
	// bypass the authentication gate via an empty agent field.
	if g.auth != nil && in.Agent == "" {
		return nil, heartbeatOut{}, fmt.Errorf("gateway: agent is required when authentication is enabled")
	}
	if in.Agent != "" {
		if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
			return nil, heartbeatOut{}, err
		}
	}
	return nil, heartbeatOut{Status: "ok"}, nil
}

// --- send_message / receive_message types --------------------------------

type sendMessageIn struct {
	Agent  string `json:"agent"`
	To     string `json:"to"`
	Body   string `json:"body"`
	APIKey string `json:"api_key"`
}

type sendMessageOut struct {
	Status string `json:"status"`
}

type receiveMessageIn struct {
	Agent  string `json:"agent"`
	APIKey string `json:"api_key"`
}

type receiveMessageOut struct {
	From string `json:"from"`
	Body string `json:"body"`
}

// --- send_message / receive_message handlers ------------------------------

func (g *Gateway) handleSendMessage(ctx context.Context, _ *mcpsdk.CallToolRequest, in sendMessageIn) (*mcpsdk.CallToolResult, sendMessageOut, error) {
	if !admin.IsValidAgentID(in.Agent) {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: invalid agent id %q", in.Agent)
	}
	if !admin.IsValidAgentID(in.To) {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: invalid recipient id %q", in.To)
	}

	// Verified by AgentAuthenticator when configured; full Ed25519 in Phase 3. CWE-287.
	if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
		return nil, sendMessageOut{}, err
	}

	allowed, err := g.enforcer.Allowed(ctx, in.Agent, in.To)
	if err != nil {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: policy check: %w", err)
	}
	if !allowed {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: agent %q not allowed", in.Agent)
	}

	if len(in.Body) > document.MaxDocumentBytes {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: message body exceeds maximum size")
	}

	doc, err := document.NewBuilder(protocol.AgentMessage).
		ID(uuid.New().String()).
		From(in.Agent).
		To(in.To).
		Body(in.Body).
		Build()
	if err != nil {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: build message doc: %w", err)
	}

	if err := g.trans.Publish(ctx, "agent.messages."+in.To, doc); err != nil {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: publish message: %w", err)
	}

	return nil, sendMessageOut{Status: "ok"}, nil
}

func (g *Gateway) handleReceiveMessage(ctx context.Context, _ *mcpsdk.CallToolRequest, in receiveMessageIn) (*mcpsdk.CallToolResult, receiveMessageOut, error) {
	if !admin.IsValidAgentID(in.Agent) {
		return nil, receiveMessageOut{}, fmt.Errorf("gateway: invalid agent id %q", in.Agent)
	}

	// Verified by AgentAuthenticator when configured; full Ed25519 in Phase 3. CWE-287.
	if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
		return nil, receiveMessageOut{}, err
	}

	allowed, err := g.enforcer.Allowed(ctx, in.Agent, "orchestrator")
	if err != nil {
		return nil, receiveMessageOut{}, fmt.Errorf("gateway: policy check: %w", err)
	}
	if !allowed {
		return nil, receiveMessageOut{}, fmt.Errorf("gateway: agent %q not allowed", in.Agent)
	}

	topic := "agent.messages." + in.Agent
	ch, err := g.trans.Subscribe(ctx, topic)
	if err != nil {
		return nil, receiveMessageOut{}, fmt.Errorf("gateway: subscribe %q: %w", topic, err)
	}

	tctx, cancel := context.WithTimeout(ctx, g.cfg.TaskReadTimeout)
	defer cancel()

	select {
	case delivery, ok := <-ch:
		if !ok {
			return nil, receiveMessageOut{}, nil
		}
		if err := g.trans.Ack(ctx, topic, delivery.MsgID); err != nil {
			return nil, receiveMessageOut{}, fmt.Errorf("gateway: ack message: %w", err)
		}
		// Guard inbound body size: the transport does not enforce this on received
		// documents, so a malformed or oversized message must be rejected here (CWE-400).
		if len(delivery.Doc.Body) > document.MaxDocumentBytes {
			return nil, receiveMessageOut{}, fmt.Errorf("gateway: received message body exceeds maximum size")
		}
		// SECURITY: delivery.Doc.From is self-reported wire data (unverified until
		// Phase 3 / Ed25519). MCP callers MUST NOT make authorization decisions based
		// on this field alone. CWE-287.
		return nil, receiveMessageOut{From: delivery.Doc.From, Body: delivery.Doc.Body}, nil
	case <-tctx.Done():
		return nil, receiveMessageOut{}, nil
	}
}
