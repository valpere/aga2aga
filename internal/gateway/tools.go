package gateway

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
)

// --- input/output types for MCP tool handlers ----------------------------

type getTaskIn struct {
	Agent string `json:"agent"`
}

type getTaskOut struct {
	TaskID string `json:"task_id"`
	Body   string `json:"body"`
}

type completeTaskIn struct {
	TaskID string `json:"task_id"`
	Agent  string `json:"agent"`
	Result string `json:"result"`
}

type completeTaskOut struct {
	Status string `json:"status"`
}

type failTaskIn struct {
	TaskID string `json:"task_id"`
	Agent  string `json:"agent"`
	Error  string `json:"error"`
}

type failTaskOut struct {
	Status string `json:"status"`
}

type heartbeatIn struct {
	Agent string `json:"agent"`
}

type heartbeatOut struct {
	Status string `json:"status"`
}

// --- stub handlers (replaced task by task) --------------------------------

func (g *Gateway) handleGetTask(ctx context.Context, _ *mcpsdk.CallToolRequest, in getTaskIn) (*mcpsdk.CallToolResult, getTaskOut, error) {
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
		taskID := delivery.Doc.ID
		g.pending.Store(taskID, topic, delivery.MsgID)
		return nil, getTaskOut{TaskID: taskID, Body: delivery.Doc.Body}, nil
	case <-tctx.Done():
		return nil, getTaskOut{}, nil
	}
}

func (g *Gateway) handleCompleteTask(ctx context.Context, _ *mcpsdk.CallToolRequest, in completeTaskIn) (*mcpsdk.CallToolResult, completeTaskOut, error) {
	allowed, err := g.enforcer.Allowed(ctx, in.Agent, "orchestrator")
	if err != nil {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: policy check: %w", err)
	}
	if !allowed {
		return nil, completeTaskOut{}, fmt.Errorf("gateway: agent %q not allowed", in.Agent)
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
	allowed, err := g.enforcer.Allowed(ctx, in.Agent, "orchestrator")
	if err != nil {
		return nil, failTaskOut{}, fmt.Errorf("gateway: policy check: %w", err)
	}
	if !allowed {
		return nil, failTaskOut{}, fmt.Errorf("gateway: agent %q not allowed", in.Agent)
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

func (g *Gateway) handleHeartbeat(_ context.Context, _ *mcpsdk.CallToolRequest, _ heartbeatIn) (*mcpsdk.CallToolResult, heartbeatOut, error) {
	return nil, heartbeatOut{Status: "ok"}, nil
}
