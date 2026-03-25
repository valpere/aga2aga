package redistransport_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
	"github.com/valpere/aga2aga/pkg/transport"
	redistransport "github.com/valpere/aga2aga/pkg/transport/redis"
)

func TestRedisTransport_ImplementsInterface(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	var _ transport.Transport = redistransport.New(client, redistransport.Options{})
}

// buildTaskRequest creates a minimal valid task.request document for testing.
func buildTaskRequest(t *testing.T) *document.Document {
	t.Helper()
	doc, err := document.NewBuilder(protocol.TaskRequest).
		ID("task-test-001").
		From("orchestrator").
		To("codegen").
		ExecID("exec-001").
		Field("step", "hello-world").
		Body("## Task\nWrite a hello world function.").
		Build()
	if err != nil {
		t.Fatalf("buildTaskRequest: %v", err)
	}
	return doc
}

func TestRedisTransport_Publish(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	rt := redistransport.New(client, redistransport.Options{})
	defer rt.Close()

	doc := buildTaskRequest(t)
	if err := rt.Publish(context.Background(), "agent.tasks.test", doc); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Verify via go-redis XRange (mr.Stream has []string Values, not map)
	entries, err := client.XRange(context.Background(), "agent.tasks.test", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("stream is empty after Publish")
	}
	if _, ok := entries[0].Values["doc"]; !ok {
		t.Error("stream entry missing 'doc' field")
	}
}

func TestRedisTransport_Subscribe_RoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	rt := redistransport.New(client, redistransport.Options{
		BlockTimeout: 100 * time.Millisecond,
	})
	defer rt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Subscribe before publishing — XGROUP CREATE "$" only sees new messages.
	ch, err := rt.Subscribe(ctx, "agent.tasks.test")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// XADD triggers miniredis signal.Broadcast() which unblocks XREADGROUP.
	doc := buildTaskRequest(t)
	if err := rt.Publish(context.Background(), "agent.tasks.test", doc); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case d, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before delivery")
		}
		if d.Doc.Envelope.ID != doc.Envelope.ID {
			t.Errorf("delivery doc ID = %q, want %q", d.Doc.Envelope.ID, doc.Envelope.ID)
		}
		if d.MsgID == "" {
			t.Error("delivery MsgID is empty")
		}
		if d.RecvedAt.IsZero() {
			t.Error("delivery RecvedAt is zero")
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for delivery")
	}
}

func TestRedisTransport_Ack_RemovesFromPEL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	rt := redistransport.New(client, redistransport.Options{
		BlockTimeout: 100 * time.Millisecond,
	})
	defer rt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := rt.Subscribe(ctx, "agent.tasks.test")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if err := rt.Publish(context.Background(), "agent.tasks.test", buildTaskRequest(t)); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	d, ok := <-ch
	if !ok {
		t.Fatal("channel closed before delivery")
	}

	// PEL should have 1 entry before Ack (miniredis has no mr.XPending method;
	// use go-redis wire call instead).
	pendingBefore, err := client.XPendingExt(context.Background(), &goredis.XPendingExtArgs{
		Stream: "agent.tasks.test",
		Group:  redistransport.DefaultConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		t.Fatalf("XPendingExt before Ack: %v", err)
	}
	if len(pendingBefore) != 1 {
		t.Fatalf("PEL before Ack: got %d, want 1", len(pendingBefore))
	}

	if err := rt.Ack(context.Background(), "agent.tasks.test", d.MsgID); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}

	pendingAfter, err := client.XPendingExt(context.Background(), &goredis.XPendingExtArgs{
		Stream: "agent.tasks.test",
		Group:  redistransport.DefaultConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		t.Fatalf("XPendingExt after Ack: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Errorf("PEL after Ack: got %d, want 0", len(pendingAfter))
	}
}

func TestRedisTransport_Subscribe_ConsumerGroupIdempotent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	rt := redistransport.New(client, redistransport.Options{BlockTimeout: 50 * time.Millisecond})
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// First Subscribe creates the consumer group.
	if _, err := rt.Subscribe(ctx, "agent.tasks.test"); err != nil {
		t.Fatalf("first Subscribe() error = %v", err)
	}
	// Second Subscribe must not error (BUSYGROUP is idempotent).
	if _, err := rt.Subscribe(ctx, "agent.tasks.test"); err != nil {
		t.Errorf("second Subscribe() (idempotent) error = %v", err)
	}
}

func TestRedisTransport_Subscribe_ContextCancellation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	rt := redistransport.New(client, redistransport.Options{BlockTimeout: 50 * time.Millisecond})
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := rt.Subscribe(ctx, "agent.tasks.cancel")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after context cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel was not closed after context cancel")
	}
}

func TestRedisTransport_Subscribe_MalformedEntrySkipped(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	rt := redistransport.New(client, redistransport.Options{BlockTimeout: 100 * time.Millisecond})
	defer rt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := rt.Subscribe(ctx, "agent.tasks.test")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Inject malformed entry (missing "doc" field) via the raw Redis client.
	if err := client.XAdd(context.Background(), &goredis.XAddArgs{
		Stream: "agent.tasks.test",
		Values: map[string]any{"notdoc": "garbage"},
	}).Err(); err != nil {
		t.Fatalf("XAdd malformed: %v", err)
	}

	// Publish a valid entry after — it must arrive despite the malformed one.
	if err := rt.Publish(context.Background(), "agent.tasks.test", buildTaskRequest(t)); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case d, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if d.Doc == nil {
			t.Error("received nil Doc")
		}
	case <-ctx.Done():
		t.Fatal("timeout — malformed entry may not have been skipped")
	}
}

func TestRedisTransport_Close_StopsSubscription(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer client.Close()

	rt := redistransport.New(client, redistransport.Options{BlockTimeout: 50 * time.Millisecond})

	ctx := context.Background()
	ch, err := rt.Subscribe(ctx, "agent.tasks.close")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	rt.Close()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after Close()")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel was not closed after Close()")
	}
}
