//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	goredis "github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/valpere/aga2aga/internal/gateway"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
	redistransport "github.com/valpere/aga2aga/pkg/transport/redis"
)

// noopEnforcer always allows. Identical to the one in internal/gateway/gateway_test.go.
type noopEnforcer struct{}

func (noopEnforcer) Allowed(_ context.Context, _, _ string) (bool, error) { return true, nil }

// startRedis launches a Redis container and returns a connected goredis client.
// The container is stopped when the test ends.
func startRedis(t *testing.T) *goredis.Client {
	t.Helper()
	ctx := context.Background()
	c, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	addr, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis connection string: %v", err)
	}
	// ConnectionString returns "redis://host:port" — strip the scheme for goredis.
	rdb := goredis.NewClient(&goredis.Options{Addr: addr[len("redis://"):]})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestGatewayIntegration_RoundTrip(t *testing.T) {
	rdb := startRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	trans := redistransport.New(rdb, redistransport.Options{})
	t.Cleanup(func() { _ = trans.Close() })

	cfg := gateway.DefaultConfig()
	cfg.AgentID = "mcp-gateway"
	cfg.TaskReadTimeout = 5 * time.Second
	gw := gateway.New(trans, noopEnforcer{}, cfg)

	t1, t2 := mcpsdk.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() { errCh <- gw.Run(ctx, t1) }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// Step 1: call get_task in background — blocks until a task arrives.
	type getResult struct {
		result *mcpsdk.CallToolResult
		err    error
	}
	getCh := make(chan getResult, 1)
	go func() {
		r, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "get_task",
			Arguments: map[string]any{"agent": "test-agent"},
		})
		getCh <- getResult{r, err}
	}()

	// Step 2: orchestrator publishes task after subscribe is set up.
	// XGroupCreateMkStream uses "$" (new messages only) so publish must arrive
	// after the group exists — the 100ms gap ensures that.
	time.Sleep(100 * time.Millisecond)
	taskDoc, err := document.NewBuilder(protocol.TaskRequest).
		ID("task-001").
		From("orchestrator").
		To("test-agent").
		ExecID("exec-001").
		Field("step", "run").
		Body("do something").
		Build()
	if err != nil {
		t.Fatalf("build task doc: %v", err)
	}
	orchTrans := redistransport.New(rdb, redistransport.Options{})
	t.Cleanup(func() { _ = orchTrans.Close() })
	if err := orchTrans.Publish(ctx, "agent.tasks.test-agent", taskDoc); err != nil {
		t.Fatalf("publish task: %v", err)
	}

	// Step 3: wait for get_task to return.
	gr := <-getCh
	if gr.err != nil {
		t.Fatalf("get_task: %v", gr.err)
	}
	if len(gr.result.Content) == 0 {
		t.Fatal("get_task: empty content")
	}
	var taskOut struct {
		TaskID string `json:"task_id"`
	}
	text, ok := gr.result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_task: content[0] is not *TextContent")
	}
	if err := json.Unmarshal([]byte(text.Text), &taskOut); err != nil {
		t.Fatalf("parse get_task result: %v", err)
	}
	if taskOut.TaskID == "" {
		t.Fatal("get_task: empty task_id")
	}

	// Step 4: complete the task.
	completeResult, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "complete_task",
		Arguments: map[string]any{
			"task_id": taskOut.TaskID,
			"agent":   "test-agent",
			"result":  "done",
		},
	})
	if err != nil {
		t.Fatalf("complete_task: %v", err)
	}
	if len(completeResult.Content) == 0 {
		t.Fatal("complete_task: empty content")
	}

	// Step 5: verify agent.events.completed stream has the result document.
	streams, err := rdb.XRange(ctx, "agent.events.completed", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRANGE completed: %v", err)
	}
	if len(streams) == 0 {
		t.Fatal("agent.events.completed: no messages")
	}
	raw, ok := streams[0].Values["doc"].(string)
	if !ok {
		t.Fatal("completed stream entry missing 'doc' field")
	}
	doc, err := document.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse completed doc: %v", err)
	}
	if doc.Type != protocol.TaskResult {
		t.Errorf("completed doc type = %q, want %q", doc.Type, protocol.TaskResult)
	}

	// Step 6: verify PEL is empty — message was XACK'd.
	pending, err := rdb.XPending(ctx, "agent.tasks.test-agent", redistransport.DefaultConsumerGroup).Result()
	if err != nil {
		t.Fatalf("XPENDING: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("PEL count = %d, want 0 (message should be ACK'd)", pending.Count)
	}
}

func TestGatewayIntegration_FailTask(t *testing.T) {
	rdb := startRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	trans := redistransport.New(rdb, redistransport.Options{})
	t.Cleanup(func() { _ = trans.Close() })

	cfg := gateway.DefaultConfig()
	cfg.AgentID = "mcp-gateway"
	cfg.TaskReadTimeout = 5 * time.Second
	gw := gateway.New(trans, noopEnforcer{}, cfg)

	t1, t2 := mcpsdk.NewInMemoryTransports()
	go func() { _ = gw.Run(ctx, t1) }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// get_task in background.
	getCh := make(chan struct {
		r   *mcpsdk.CallToolResult
		err error
	}, 1)
	go func() {
		r, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "get_task",
			Arguments: map[string]any{"agent": "fail-agent"},
		})
		getCh <- struct {
			r   *mcpsdk.CallToolResult
			err error
		}{r, err}
	}()

	// Publish task after subscribe is ready.
	time.Sleep(100 * time.Millisecond)
	taskDoc, err := document.NewBuilder(protocol.TaskRequest).
		ID("fail-task-001").
		From("orchestrator").
		To("fail-agent").
		ExecID("exec-002").
		Field("step", "run").
		Body("will fail").
		Build()
	if err != nil {
		t.Fatalf("build task doc: %v", err)
	}
	orchTrans := redistransport.New(rdb, redistransport.Options{})
	t.Cleanup(func() { _ = orchTrans.Close() })
	if err := orchTrans.Publish(ctx, "agent.tasks.fail-agent", taskDoc); err != nil {
		t.Fatalf("publish task: %v", err)
	}

	gr := <-getCh
	if gr.err != nil {
		t.Fatalf("get_task: %v", gr.err)
	}
	if len(gr.r.Content) == 0 {
		t.Fatal("get_task: empty content")
	}
	text, ok := gr.r.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_task: content[0] is not *TextContent")
	}
	var taskOut struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(text.Text), &taskOut); err != nil {
		t.Fatalf("parse get_task result: %v", err)
	}
	if taskOut.TaskID == "" {
		t.Fatal("get_task: empty task_id")
	}

	// Call fail_task.
	_, err = cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "fail_task",
		Arguments: map[string]any{
			"task_id": taskOut.TaskID,
			"agent":   "fail-agent",
			"error":   "something went wrong",
		},
	})
	if err != nil {
		t.Fatalf("fail_task: %v", err)
	}

	// Verify agent.events.failed stream.
	streams, err := rdb.XRange(ctx, "agent.events.failed", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRANGE failed: %v", err)
	}
	if len(streams) == 0 {
		t.Fatal("agent.events.failed: no messages")
	}
	raw, ok := streams[0].Values["doc"].(string)
	if !ok {
		t.Fatal("failed stream entry missing 'doc' field")
	}
	doc, err := document.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed doc: %v", err)
	}
	if doc.Type != protocol.TaskFail {
		t.Errorf("failed doc type = %q, want %q", doc.Type, protocol.TaskFail)
	}
}
