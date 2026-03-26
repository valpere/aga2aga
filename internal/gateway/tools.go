package gateway

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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

func (g *Gateway) handleGetTask(_ context.Context, _ *mcpsdk.CallToolRequest, _ getTaskIn) (*mcpsdk.CallToolResult, getTaskOut, error) {
	return nil, getTaskOut{}, fmt.Errorf("not implemented")
}

func (g *Gateway) handleCompleteTask(_ context.Context, _ *mcpsdk.CallToolRequest, _ completeTaskIn) (*mcpsdk.CallToolResult, completeTaskOut, error) {
	return nil, completeTaskOut{}, fmt.Errorf("not implemented")
}

func (g *Gateway) handleFailTask(_ context.Context, _ *mcpsdk.CallToolRequest, _ failTaskIn) (*mcpsdk.CallToolResult, failTaskOut, error) {
	return nil, failTaskOut{}, fmt.Errorf("not implemented")
}

func (g *Gateway) handleHeartbeat(_ context.Context, _ *mcpsdk.CallToolRequest, _ heartbeatIn) (*mcpsdk.CallToolResult, heartbeatOut, error) {
	return nil, heartbeatOut{}, fmt.Errorf("not implemented")
}
