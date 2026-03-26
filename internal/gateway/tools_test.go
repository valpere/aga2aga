package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/transport"
)

// --- shared mocks ---------------------------------------------------------

// mockTransport records Publish calls and can serve pre-loaded deliveries.
type mockTransport struct {
	publishTopic string
	ch           map[string]chan transport.Delivery
	acked        bool
}

func (m *mockTransport) Publish(_ context.Context, topic string, _ *document.Document) error {
	m.publishTopic = topic
	return nil
}

func (m *mockTransport) Subscribe(_ context.Context, topic string) (<-chan transport.Delivery, error) {
	if ch, ok := m.ch[topic]; ok {
		return ch, nil
	}
	return make(chan transport.Delivery), nil
}

func (m *mockTransport) Ack(_ context.Context, _, _ string) error {
	m.acked = true
	return nil
}

func (m *mockTransport) Close() error { return nil }

// mockEnforcer returns a fixed (allowed, err) pair.
type mockEnforcer struct {
	allowed bool
	err     error
}

func (m *mockEnforcer) Allowed(_ context.Context, _, _ string) (bool, error) {
	return m.allowed, m.err
}

// newTestGateway builds a Gateway with fast timeouts suitable for unit tests.
func newTestGateway(t *testing.T, trans transport.Transport, enf PolicyEnforcer) *Gateway {
	t.Helper()
	cfg := DefaultConfig()
	cfg.TaskReadTimeout = 50 * time.Millisecond
	return New(trans, enf, cfg)
}

// --- heartbeat tests ------------------------------------------------------

func TestHandleHeartbeat(t *testing.T) {
	tests := []struct {
		name       string
		agent      string
		wantStatus string
	}{
		{name: "valid agent returns ok", agent: "agent-a", wantStatus: "ok"},
		{name: "empty agent still ok", agent: "", wantStatus: "ok"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGateway(t,
				&mockTransport{ch: map[string]chan transport.Delivery{}},
				&mockEnforcer{allowed: true},
			)
			in := heartbeatIn{Agent: tc.agent}
			_, out, err := g.handleHeartbeat(context.Background(), &mcpsdk.CallToolRequest{}, in)
			if err != nil {
				t.Errorf("handleHeartbeat() error = %v; want nil", err)
			}
			if out.Status != tc.wantStatus {
				t.Errorf("status = %q; want %q", out.Status, tc.wantStatus)
			}
		})
	}
}

// --- get_task tests -------------------------------------------------------

func TestHandleGetTask(t *testing.T) {
	testDoc := &document.Document{
		Envelope: document.Envelope{ID: "task-123", Type: "task.request"},
		Body:     "## Task\nDo the thing.",
	}

	tests := []struct {
		name        string
		agent       string
		delivery    *transport.Delivery
		allowed     bool
		enforcerErr error
		wantErr     bool
		wantTaskID  string
		wantBody    string
		wantStored  bool
	}{
		{
			name:       "task delivered and stored in pending",
			agent:      "agent-a",
			delivery:   &transport.Delivery{Doc: testDoc, MsgID: "redis-1-0"},
			allowed:    true,
			wantTaskID: "task-123",
			wantBody:   testDoc.Body,
			wantStored: true,
		},
		{
			name:    "policy denial returns error",
			agent:   "agent-a",
			allowed: false,
			wantErr: true,
		},
		{
			name:        "enforcer error propagates",
			agent:       "agent-a",
			enforcerErr: errors.New("store down"),
			wantErr:     true,
		},
		{
			name:       "no task available returns empty task_id",
			agent:      "agent-a",
			allowed:    true,
			wantTaskID: "",
			wantBody:   "",
			wantStored: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan transport.Delivery, 1)
			if tc.delivery != nil {
				ch <- *tc.delivery
			}
			trans := &mockTransport{
				ch: map[string]chan transport.Delivery{
					"agent.tasks.agent-a": ch,
				},
			}
			enf := &mockEnforcer{allowed: tc.allowed, err: tc.enforcerErr}
			g := newTestGateway(t, trans, enf)

			in := getTaskIn{Agent: tc.agent}
			_, out, err := g.handleGetTask(context.Background(), nil, in)

			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v; wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if out.TaskID != tc.wantTaskID {
					t.Errorf("task_id = %q; want %q", out.TaskID, tc.wantTaskID)
				}
				if out.Body != tc.wantBody {
					t.Errorf("body = %q; want %q", out.Body, tc.wantBody)
				}
				if tc.wantStored {
					_, _, ok := g.pending.Load(tc.wantTaskID)
					if !ok {
						t.Error("task not stored in PendingMap")
					}
				}
			}
		})
	}
}

// --- complete_task tests --------------------------------------------------

func TestHandleCompleteTask(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		agent       string
		result      string
		prepPending bool
		allowed     bool
		wantErr     bool
		wantAcked   bool
		wantPublish string
	}{
		{
			name:        "success",
			taskID:      "task-123",
			agent:       "agent-a",
			result:      "done",
			prepPending: true,
			allowed:     true,
			wantAcked:   true,
			wantPublish: "agent.events.completed",
		},
		{
			name:    "policy denial returns error",
			taskID:  "task-123",
			agent:   "agent-a",
			allowed: false,
			wantErr: true,
		},
		{
			name:    "unknown task_id returns error",
			taskID:  "no-such-task",
			agent:   "agent-a",
			allowed: true,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trans := &mockTransport{ch: map[string]chan transport.Delivery{}}
			enf := &mockEnforcer{allowed: tc.allowed}
			g := newTestGateway(t, trans, enf)

			if tc.prepPending {
				g.pending.Store(tc.taskID, "agent.tasks.agent-a", "redis-1-0")
			}

			in := completeTaskIn{TaskID: tc.taskID, Agent: tc.agent, Result: tc.result}
			_, out, err := g.handleCompleteTask(context.Background(), nil, in)

			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v; wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if out.Status != "ok" {
					t.Errorf("status = %q; want ok", out.Status)
				}
				if trans.publishTopic != tc.wantPublish {
					t.Errorf("publishTopic = %q; want %q", trans.publishTopic, tc.wantPublish)
				}
				if trans.acked != tc.wantAcked {
					t.Errorf("acked = %v; want %v", trans.acked, tc.wantAcked)
				}
			}
		})
	}
}

// --- fail_task tests ------------------------------------------------------

func TestHandleFailTask(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		agent       string
		errMsg      string
		prepPending bool
		allowed     bool
		wantErr     bool
		wantPublish string
		wantAcked   bool
	}{
		{
			name:        "success",
			taskID:      "task-456",
			agent:       "agent-a",
			errMsg:      "timeout",
			prepPending: true,
			allowed:     true,
			wantPublish: "agent.events.failed",
			wantAcked:   true,
		},
		{
			name:    "policy denial returns error",
			taskID:  "task-456",
			agent:   "agent-a",
			allowed: false,
			wantErr: true,
		},
		{
			name:    "unknown task_id returns error",
			taskID:  "no-such",
			agent:   "agent-a",
			allowed: true,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trans := &mockTransport{ch: map[string]chan transport.Delivery{}}
			enf := &mockEnforcer{allowed: tc.allowed}
			g := newTestGateway(t, trans, enf)

			if tc.prepPending {
				g.pending.Store(tc.taskID, "agent.tasks.agent-a", "redis-2-0")
			}

			in := failTaskIn{TaskID: tc.taskID, Agent: tc.agent, Error: tc.errMsg}
			_, out, err := g.handleFailTask(context.Background(), nil, in)

			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v; wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if out.Status != "ok" {
					t.Errorf("status = %q; want ok", out.Status)
				}
				if trans.publishTopic != tc.wantPublish {
					t.Errorf("publishTopic = %q; want %q", trans.publishTopic, tc.wantPublish)
				}
				if trans.acked != tc.wantAcked {
					t.Errorf("acked = %v; want %v", trans.acked, tc.wantAcked)
				}
			}
		})
	}
}
