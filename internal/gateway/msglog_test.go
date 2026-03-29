package gateway_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/valpere/aga2aga/internal/gateway"
	"github.com/valpere/aga2aga/pkg/admin"
)

// --- stub MessageLogStore for EmbeddedMessageLogger tests ---

type stubLogStore struct {
	mu   sync.Mutex
	logs []admin.MessageLog
}

func (s *stubLogStore) AppendMessageLog(_ context.Context, m *admin.MessageLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, *m)
	return nil
}

func (s *stubLogStore) ListMessageLogs(_ context.Context, _ string, _ admin.MessageLogFilter) ([]admin.MessageLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]admin.MessageLog, len(s.logs))
	copy(out, s.logs)
	return out, nil
}

func (s *stubLogStore) DeleteMessageLogsBefore(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

func (s *stubLogStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.logs)
}

// --- tests ---

func TestNoopMessageLogger(t *testing.T) {
	logger := gateway.NewNoopMessageLogger()
	// Must not panic and must return immediately.
	entry := gateway.MessageLogEntry{
		EnvelopeID: "e1", FromAgent: "a", ToAgent: "b",
		MsgType: "agent.message", Direction: "send", ToolName: "send_message",
		BodySize: 5, Body: "hello",
	}
	logger.Log(context.Background(), entry)
}

func TestEmbeddedMessageLogger_LogsEntry(t *testing.T) {
	store := &stubLogStore{}
	logger := gateway.NewEmbeddedMessageLogger(store, "org-1")
	defer logger.Close()

	entry := gateway.MessageLogEntry{
		EnvelopeID: "env-1", ThreadID: "thr-1",
		FromAgent: "agent-alpha", ToAgent: "agent-beta",
		MsgType: "agent.message", Direction: "send", ToolName: "send_message",
		BodySize: 5, Body: "hello",
	}
	logger.Log(context.Background(), entry)

	// Wait for async drain.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if store.count() == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if store.count() != 1 {
		t.Fatalf("expected 1 log entry, got %d", store.count())
	}

	got := store.logs[0]
	if got.OrgID != "org-1" {
		t.Errorf("OrgID = %q, want %q", got.OrgID, "org-1")
	}
	if got.FromAgent != "agent-alpha" {
		t.Errorf("FromAgent = %q, want %q", got.FromAgent, "agent-alpha")
	}
	if got.ToolName != "send_message" {
		t.Errorf("ToolName = %q, want %q", got.ToolName, "send_message")
	}
	if got.Body != "hello" {
		t.Errorf("Body = %q, want %q", got.Body, "hello")
	}
}

func TestEmbeddedMessageLogger_DropsWhenFull(t *testing.T) {
	// Use a store that blocks so the channel fills.
	blocked := make(chan struct{})
	blockingStore := &blockingLogStore{block: blocked}
	logger := gateway.NewEmbeddedMessageLoggerWithCap(blockingStore, "org-1", 2)
	defer func() {
		close(blocked)
		logger.Close()
	}()

	entry := gateway.MessageLogEntry{FromAgent: "a", ToAgent: "b", MsgType: "m", Direction: "send", ToolName: "t"}

	// Send enough to fill the buffer; excess must be dropped without blocking.
	for i := 0; i < 10; i++ {
		logger.Log(context.Background(), entry)
	}
	// Test passes as long as Log() returns without deadlocking.
}

// blockingLogStore blocks on AppendMessageLog until unblocked.
type blockingLogStore struct {
	block <-chan struct{}
}

func (b *blockingLogStore) AppendMessageLog(ctx context.Context, _ *admin.MessageLog) error {
	select {
	case <-b.block:
	case <-ctx.Done():
	}
	return nil
}

func (b *blockingLogStore) ListMessageLogs(_ context.Context, _ string, _ admin.MessageLogFilter) ([]admin.MessageLog, error) {
	return nil, nil
}

func (b *blockingLogStore) DeleteMessageLogsBefore(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}
