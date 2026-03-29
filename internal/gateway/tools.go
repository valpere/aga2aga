package gateway

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/pkg/admin"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
	"github.com/valpere/aga2aga/pkg/transport"
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
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
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

	if err := g.limiter.CheckPendingTasks(ctx, in.Agent, g.pending.CountByAgent(in.Agent)); err != nil {
		return nil, getTaskOut{}, err
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
		g.logger.Log(ctx, MessageLogEntry{
			EnvelopeID: delivery.Doc.ID,
			ThreadID:   delivery.Doc.ThreadID,
			FromAgent:  delivery.Doc.From,
			ToAgent:    in.Agent,
			MsgType:    string(delivery.Doc.Type),
			Direction:  "receive",
			ToolName:   "get_task",
			BodySize:   len(delivery.Doc.Body),
			Body:       delivery.Doc.Body,
		})
		return nil, getTaskOut{TaskID: taskID, Body: delivery.Doc.Body}, nil
	case <-tctx.Done():
		return nil, getTaskOut{}, nil
	}
}

func (g *Gateway) handleCompleteTask(ctx context.Context, _ *mcpsdk.CallToolRequest, in completeTaskIn) (*mcpsdk.CallToolResult, completeTaskOut, error) {
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
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
	if err := g.limiter.CheckSend(ctx, in.Agent, len(in.Result)); err != nil {
		return nil, completeTaskOut{}, err
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

	maxLen := g.limiter.GetStreamMaxLen(ctx, in.Agent)
	if err := g.trans.Publish(ctx, "agent.events.completed", doc, transport.PublishOptions{MaxLen: maxLen}); err != nil {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: publish result: %w", err)
	}
	g.limiter.RecordSend(ctx, in.Agent)

	if err := g.trans.Ack(ctx, topic, msgID); err != nil {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: ack task: %w", err)
	}

	g.logger.Log(ctx, MessageLogEntry{
		EnvelopeID: doc.ID,
		ThreadID:   doc.ThreadID,
		FromAgent:  in.Agent,
		ToAgent:    "orchestrator",
		MsgType:    string(protocol.TaskResult),
		Direction:  "send",
		ToolName:   "complete_task",
		BodySize:   len(in.Result),
		Body:       in.Result,
	})
	return nil, completeTaskOut{Status: "ok"}, nil
}

func (g *Gateway) handleFailTask(ctx context.Context, _ *mcpsdk.CallToolRequest, in failTaskIn) (*mcpsdk.CallToolResult, failTaskOut, error) {
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
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
	if err := g.limiter.CheckSend(ctx, in.Agent, len(in.Error)); err != nil {
		return nil, failTaskOut{}, err
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

	maxLen := g.limiter.GetStreamMaxLen(ctx, in.Agent)
	if err := g.trans.Publish(ctx, "agent.events.failed", doc, transport.PublishOptions{MaxLen: maxLen}); err != nil {
		return nil, failTaskOut{}, fmt.Errorf("gateway: publish fail: %w", err)
	}
	g.limiter.RecordSend(ctx, in.Agent)

	if err := g.trans.Ack(ctx, topic, msgID); err != nil {
		return nil, failTaskOut{}, fmt.Errorf("gateway: ack task: %w", err)
	}

	g.logger.Log(ctx, MessageLogEntry{
		EnvelopeID: doc.ID,
		ThreadID:   doc.ThreadID,
		FromAgent:  in.Agent,
		ToAgent:    "orchestrator",
		MsgType:    string(protocol.TaskFail),
		Direction:  "send",
		ToolName:   "fail_task",
		BodySize:   len(in.Error),
		Body:       in.Error,
	})
	return nil, failTaskOut{Status: "ok"}, nil
}

func (g *Gateway) handleHeartbeat(ctx context.Context, _ *mcpsdk.CallToolRequest, in heartbeatIn) (*mcpsdk.CallToolResult, heartbeatOut, error) {
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
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
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
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
	if err := g.limiter.CheckSend(ctx, in.Agent, len(in.Body)); err != nil {
		return nil, sendMessageOut{}, err
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

	maxLen := g.limiter.GetStreamMaxLen(ctx, in.To)
	if err := g.trans.Publish(ctx, "agent.messages."+in.To, doc, transport.PublishOptions{MaxLen: maxLen}); err != nil {
		return nil, sendMessageOut{}, fmt.Errorf("gateway: publish message: %w", err)
	}
	g.limiter.RecordSend(ctx, in.Agent)

	g.logger.Log(ctx, MessageLogEntry{
		EnvelopeID: doc.ID,
		ThreadID:   doc.ThreadID,
		FromAgent:  in.Agent,
		ToAgent:    in.To,
		MsgType:    string(protocol.AgentMessage),
		Direction:  "send",
		ToolName:   "send_message",
		BodySize:   len(in.Body),
		Body:       in.Body,
	})
	return nil, sendMessageOut{Status: "ok"}, nil
}

func (g *Gateway) handleReceiveMessage(ctx context.Context, _ *mcpsdk.CallToolRequest, in receiveMessageIn) (*mcpsdk.CallToolResult, receiveMessageOut, error) {
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
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
		g.logger.Log(ctx, MessageLogEntry{
			EnvelopeID: delivery.Doc.ID,
			ThreadID:   delivery.Doc.ThreadID,
			FromAgent:  delivery.Doc.From,
			ToAgent:    in.Agent,
			MsgType:    string(delivery.Doc.Type),
			Direction:  "receive",
			ToolName:   "receive_message",
			BodySize:   len(delivery.Doc.Body),
			Body:       delivery.Doc.Body,
		})
		return nil, receiveMessageOut{From: delivery.Doc.From, Body: delivery.Doc.Body}, nil
	case <-tctx.Done():
		return nil, receiveMessageOut{}, nil
	}
}

// --- get_my_limits / get_my_policies types ----------------------------------

type getMyLimitsIn struct {
	Agent  string `json:"agent"`
	APIKey string `json:"api_key"`
}

// getMyLimitsOut mirrors LimitInfo as an MCP output struct.
type getMyLimitsOut struct {
	MaxBodyBytes    int   `json:"max_body_bytes"`
	MaxSendPerMin   int   `json:"max_send_per_min"`
	MaxPendingTasks int   `json:"max_pending_tasks"`
	MaxStreamLen    int64 `json:"max_stream_len"`
}

type getMyPoliciesIn struct {
	Agent  string `json:"agent"`
	APIKey string `json:"api_key"`
}

// getMyPoliciesOut wraps the policy list in an object so the MCP SDK
// can generate an "object" output schema.
type getMyPoliciesOut struct {
	Policies []policyInfo `json:"policies"`
}

// policyInfo is the JSON shape returned to agents by get_my_policies.
type policyInfo struct {
	ID        string `json:"id"`
	SourceID  string `json:"source_id"`
	TargetID  string `json:"target_id"`
	Direction string `json:"direction"`
	Action    string `json:"action"`
	Priority  int    `json:"priority"`
}

// --- get_my_limits / get_my_policies handlers --------------------------------

func (g *Gateway) handleGetMyLimits(ctx context.Context, _ *mcpsdk.CallToolRequest, in getMyLimitsIn) (*mcpsdk.CallToolResult, getMyLimitsOut, error) {
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
	if !admin.IsValidAgentID(in.Agent) {
		return nil, getMyLimitsOut{}, fmt.Errorf("gateway: invalid agent id %q", in.Agent)
	}
	if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
		return nil, getMyLimitsOut{}, err
	}

	info, err := g.limiter.GetEffectiveLimits(ctx, in.Agent)
	if err != nil {
		return nil, getMyLimitsOut{}, fmt.Errorf("gateway: get limits: %w", err)
	}

	return nil, getMyLimitsOut{
		MaxBodyBytes:    info.MaxBodyBytes,
		MaxSendPerMin:   info.MaxSendPerMin,
		MaxPendingTasks: info.MaxPendingTasks,
		MaxStreamLen:    info.MaxStreamLen,
	}, nil
}

func (g *Gateway) handleGetMyPolicies(ctx context.Context, _ *mcpsdk.CallToolRequest, in getMyPoliciesIn) (*mcpsdk.CallToolResult, getMyPoliciesOut, error) {
	in.Agent, in.APIKey = g.applyDefaults(in.Agent, in.APIKey)
	if !admin.IsValidAgentID(in.Agent) {
		return nil, getMyPoliciesOut{}, fmt.Errorf("gateway: invalid agent id %q", in.Agent)
	}
	if err := g.authenticateAgent(ctx, in.Agent, in.APIKey); err != nil {
		return nil, getMyPoliciesOut{}, err
	}

	querier, ok := g.enforcer.(PolicyQuerier)
	if !ok {
		// Enforcer does not support policy listing (e.g. HTTPEnforcer stub).
		// Return empty list rather than an error — agents can proceed.
		return nil, getMyPoliciesOut{Policies: []policyInfo{}}, nil
	}

	policies, err := querier.ListPoliciesFor(ctx, in.Agent)
	if err != nil {
		return nil, getMyPoliciesOut{}, fmt.Errorf("gateway: list policies: %w", err)
	}

	infos := make([]policyInfo, 0, len(policies))
	for _, p := range policies {
		infos = append(infos, policyInfo{
			ID:        p.ID,
			SourceID:  p.SourceID,
			TargetID:  p.TargetID,
			Direction: string(p.Direction),
			Action:    string(p.Action),
			Priority:  p.Priority,
		})
	}
	return nil, getMyPoliciesOut{Policies: infos}, nil
}
