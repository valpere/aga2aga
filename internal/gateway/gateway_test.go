package gateway_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/internal/gateway"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/transport"
)

// noopTransport satisfies transport.Transport with no-op operations.
type noopTransport struct{}

func (noopTransport) Publish(_ context.Context, _ string, _ *document.Document) error {
	return nil
}
func (noopTransport) Subscribe(_ context.Context, _ string) (<-chan transport.Delivery, error) {
	return make(chan transport.Delivery), nil
}
func (noopTransport) Ack(_ context.Context, _, _ string) error { return nil }
func (noopTransport) Close() error                             { return nil }

// noopEnforcer always allows.
type noopEnforcer struct{}

func (noopEnforcer) Allowed(_ context.Context, _, _ string) (bool, error) { return true, nil }

func TestGateway_RegisterTools(t *testing.T) {
	g := gateway.New(noopTransport{}, noopEnforcer{}, gateway.DefaultConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t1, t2 := mcpsdk.NewInMemoryTransports()

	errCh := make(chan error, 1)
	go func() { errCh <- g.Run(ctx, t1) }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	var names []string
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("cs.Tools: %v", err)
		}
		names = append(names, tool.Name)
	}

	want := []string{"complete_task", "fail_task", "get_task", "heartbeat", "receive_message", "send_message"}
	if len(names) != len(want) {
		t.Errorf("registered %d tools; want %d: got %v", len(names), len(want), names)
	}
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tool %q not registered; got %v", w, names)
		}
	}
}

func TestGateway_Server_NotNil(t *testing.T) {
	g := gateway.New(noopTransport{}, noopEnforcer{}, gateway.DefaultConfig())
	if g.Server() == nil {
		t.Error("Server() returned nil; want non-nil MCP server")
	}
}

func TestGateway_StartCleanup_DoesNotBlock(t *testing.T) {
	g := gateway.New(noopTransport{}, noopEnforcer{}, gateway.DefaultConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		g.StartCleanup(ctx)
		close(done)
	}()
	select {
	case <-done:
		// StartCleanup returned immediately — good (it starts a goroutine internally)
	case <-time.After(500 * time.Millisecond):
		t.Error("StartCleanup did not return promptly")
	}
}

func TestGateway_Run_ExitsOnContextCancel(t *testing.T) {
	g := gateway.New(noopTransport{}, noopEnforcer{}, gateway.DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())

	t1, _ := mcpsdk.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() { errCh <- g.Run(ctx, t1) }()

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() = %v; want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after context cancel")
	}
}
