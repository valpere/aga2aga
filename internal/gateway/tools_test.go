package gateway

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valpere/aga2aga/pkg/admin"
	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
	"github.com/valpere/aga2aga/pkg/transport"
)

// --- shared mocks ---------------------------------------------------------

// mockTransport records Publish calls, captures the published document, and
// can serve pre-loaded deliveries. Error fields allow injection of specific
// failures to test error-handling paths.
type mockTransport struct {
	// delivery channels keyed by topic
	ch map[string]chan transport.Delivery
	// injected errors
	subscribeErr error
	publishErr   error
	ackErr       error
	// recorded calls
	publishTopic string
	publishDoc   *document.Document
	acked        bool
}

func (m *mockTransport) Publish(_ context.Context, topic string, doc *document.Document, _ ...transport.PublishOptions) error {
	m.publishTopic = topic
	m.publishDoc = doc
	return m.publishErr
}

func (m *mockTransport) Subscribe(_ context.Context, topic string) (<-chan transport.Delivery, error) {
	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}
	if ch, ok := m.ch[topic]; ok {
		return ch, nil
	}
	return make(chan transport.Delivery), nil
}

func (m *mockTransport) Ack(_ context.Context, _, _ string) error {
	m.acked = true
	return m.ackErr
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

// spyLogger records every MessageLogEntry passed to Log().
type spyLogger struct {
	mu      sync.Mutex
	entries []MessageLogEntry
}

func (s *spyLogger) Log(_ context.Context, e MessageLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
}

func (s *spyLogger) last() (MessageLogEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) == 0 {
		return MessageLogEntry{}, false
	}
	return s.entries[len(s.entries)-1], true
}

// newTestGateway builds a Gateway with fast timeouts suitable for unit tests.
func newTestGateway(t *testing.T, trans transport.Transport, enf PolicyEnforcer) *Gateway {
	t.Helper()
	cfg := DefaultConfig()
	cfg.TaskReadTimeout = 50 * time.Millisecond
	return New(trans, enf, nil, NewNoopMessageLogger(), nil, cfg)
}

// newTestGatewayWithSpy builds a Gateway that records log entries via spyLogger.
func newTestGatewayWithSpy(t *testing.T, trans transport.Transport, enf PolicyEnforcer) (*Gateway, *spyLogger) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.TaskReadTimeout = 50 * time.Millisecond
	spy := &spyLogger{}
	return New(trans, enf, nil, spy, nil, cfg), spy
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
		name         string
		agent        string
		delivery     *transport.Delivery
		allowed      bool
		enforcerErr  error
		subscribeErr error
		wantErr      bool
		wantTaskID   string
		wantBody     string
		wantStored   bool
	}{
		{
			// taskID is delivery.MsgID (transport-layer token), not Doc.ID
			name:       "task delivered and stored in pending",
			agent:      "agent-a",
			delivery:   &transport.Delivery{Doc: testDoc, MsgID: "redis-1-0"},
			allowed:    true,
			wantTaskID: "redis-1-0",
			wantBody:   testDoc.Body,
			wantStored: true,
		},
		{
			name:    "invalid agent id returns error",
			agent:   "",
			wantErr: true,
		},
		{
			name:    "agent id with newline rejected",
			agent:   "agent\nnewline",
			wantErr: true,
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
			name:         "subscribe error returns error",
			agent:        "agent-a",
			allowed:      true,
			subscribeErr: errors.New("redis down"),
			wantErr:      true,
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
				subscribeErr: tc.subscribeErr,
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
		enforcerErr error
		publishErr  error
		ackErr      error
		wantErr     bool
		wantAcked   bool
		wantPublish string
		wantDocType string
		wantDocFrom string
		wantDocTo   string
	}{
		{
			name:        "success — publishes task.result with correct envelope",
			taskID:      "task-123",
			agent:       "agent-a",
			result:      "done",
			prepPending: true,
			allowed:     true,
			wantAcked:   true,
			wantPublish: "agent.events.completed",
			wantDocType: "task.result",
			wantDocFrom: "mcp-gateway",
			wantDocTo:   "agent-a",
		},
		{
			name:    "invalid agent id returns error",
			taskID:  "task-123",
			agent:   "",
			wantErr: true,
		},
		{
			name:        "enforcer error propagates",
			taskID:      "task-123",
			agent:       "agent-a",
			enforcerErr: errors.New("store down"),
			wantErr:     true,
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
		{
			// publish failure must prevent Ack
			name:        "publish error — Ack not called",
			taskID:      "task-123",
			agent:       "agent-a",
			prepPending: true,
			allowed:     true,
			publishErr:  errors.New("redis publish failed"),
			wantErr:     true,
			wantAcked:   false,
		},
		{
			// Ack failure is still an error but publish already succeeded
			name:        "ack error returns error",
			taskID:      "task-123",
			agent:       "agent-a",
			prepPending: true,
			allowed:     true,
			ackErr:      errors.New("redis ack failed"),
			wantErr:     true,
			wantPublish: "agent.events.completed",
			wantAcked:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trans := &mockTransport{
				ch:         map[string]chan transport.Delivery{},
				publishErr: tc.publishErr,
				ackErr:     tc.ackErr,
			}
			enf := &mockEnforcer{allowed: tc.allowed, err: tc.enforcerErr}
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
			}
			if tc.wantPublish != "" && trans.publishTopic != tc.wantPublish {
				t.Errorf("publishTopic = %q; want %q", trans.publishTopic, tc.wantPublish)
			}
			if trans.acked != tc.wantAcked {
				t.Errorf("acked = %v; want %v", trans.acked, tc.wantAcked)
			}
			if tc.wantDocType != "" && trans.publishDoc != nil {
				if string(trans.publishDoc.Type) != tc.wantDocType {
					t.Errorf("doc.Type = %q; want %q", trans.publishDoc.Type, tc.wantDocType)
				}
				if trans.publishDoc.From != tc.wantDocFrom {
					t.Errorf("doc.From = %q; want %q", trans.publishDoc.From, tc.wantDocFrom)
				}
				if len(trans.publishDoc.To) == 0 || string(trans.publishDoc.To[0]) != tc.wantDocTo {
					t.Errorf("doc.To = %v; want [%q]", trans.publishDoc.To, tc.wantDocTo)
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
		enforcerErr error
		publishErr  error
		ackErr      error
		wantErr     bool
		wantAcked   bool
		wantPublish string
		wantDocType string
		wantDocFrom string
		wantDocTo   string
	}{
		{
			name:        "success — publishes task.fail with correct envelope",
			taskID:      "task-456",
			agent:       "agent-a",
			errMsg:      "timeout",
			prepPending: true,
			allowed:     true,
			wantAcked:   true,
			wantPublish: "agent.events.failed",
			wantDocType: "task.fail",
			wantDocFrom: "mcp-gateway",
			wantDocTo:   "agent-a",
		},
		{
			name:    "invalid agent id returns error",
			taskID:  "task-456",
			agent:   "",
			wantErr: true,
		},
		{
			name:        "enforcer error propagates",
			taskID:      "task-456",
			agent:       "agent-a",
			enforcerErr: errors.New("store down"),
			wantErr:     true,
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
		{
			// publish failure must prevent Ack
			name:        "publish error — Ack not called",
			taskID:      "task-456",
			agent:       "agent-a",
			prepPending: true,
			allowed:     true,
			publishErr:  errors.New("redis publish failed"),
			wantErr:     true,
			wantAcked:   false,
		},
		{
			// Ack failure is still an error but publish already succeeded
			name:        "ack error returns error",
			taskID:      "task-456",
			agent:       "agent-a",
			prepPending: true,
			allowed:     true,
			ackErr:      errors.New("redis ack failed"),
			wantErr:     true,
			wantPublish: "agent.events.failed",
			wantAcked:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trans := &mockTransport{
				ch:         map[string]chan transport.Delivery{},
				publishErr: tc.publishErr,
				ackErr:     tc.ackErr,
			}
			enf := &mockEnforcer{allowed: tc.allowed, err: tc.enforcerErr}
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
			}
			if tc.wantPublish != "" && trans.publishTopic != tc.wantPublish {
				t.Errorf("publishTopic = %q; want %q", trans.publishTopic, tc.wantPublish)
			}
			if trans.acked != tc.wantAcked {
				t.Errorf("acked = %v; want %v", trans.acked, tc.wantAcked)
			}
			if tc.wantDocType != "" && trans.publishDoc != nil {
				if string(trans.publishDoc.Type) != tc.wantDocType {
					t.Errorf("doc.Type = %q; want %q", trans.publishDoc.Type, tc.wantDocType)
				}
				if trans.publishDoc.From != tc.wantDocFrom {
					t.Errorf("doc.From = %q; want %q", trans.publishDoc.From, tc.wantDocFrom)
				}
				if len(trans.publishDoc.To) == 0 || string(trans.publishDoc.To[0]) != tc.wantDocTo {
					t.Errorf("doc.To = %v; want [%q]", trans.publishDoc.To, tc.wantDocTo)
				}
			}
		})
	}
}

// --- send_message tests ---------------------------------------------------

func TestHandleSendMessage(t *testing.T) {
	bigBody := string(make([]byte, document.MaxDocumentBytes+1))

	tests := []struct {
		name        string
		agent       string
		to          string
		body        string
		allowed     bool
		enforcerErr error
		publishErr  error
		wantErr     bool
		wantStatus  string
		wantTopic   string
		wantDocType string
		wantDocFrom string
		wantDocTo   string
		wantBody    string
	}{
		{
			name:        "success — publishes agent.message to recipient stream",
			agent:       "agent-a",
			to:          "agent-b",
			body:        "watch out for genome-789",
			allowed:     true,
			wantStatus:  "ok",
			wantTopic:   "agent.messages.agent-b",
			wantDocType: "agent.message",
			wantDocFrom: "agent-a",
			wantDocTo:   "agent-b",
			wantBody:    "watch out for genome-789",
		},
		{
			name:    "invalid sender agent id returns error",
			agent:   "",
			to:      "agent-b",
			body:    "hi",
			wantErr: true,
		},
		{
			name:    "sender with newline rejected",
			agent:   "agent\nnewline",
			to:      "agent-b",
			body:    "hi",
			wantErr: true,
		},
		{
			name:    "invalid recipient id returns error",
			agent:   "agent-a",
			to:      "bad\nid",
			body:    "hi",
			wantErr: true,
		},
		{
			name:    "policy denial returns error",
			agent:   "agent-a",
			to:      "agent-b",
			body:    "hi",
			allowed: false,
			wantErr: true,
		},
		{
			name:        "enforcer error propagates",
			agent:       "agent-a",
			to:          "agent-b",
			body:        "hi",
			enforcerErr: errors.New("store down"),
			wantErr:     true,
		},
		{
			name:    "body exceeds maximum size returns error",
			agent:   "agent-a",
			to:      "agent-b",
			body:    bigBody,
			allowed: true,
			wantErr: true,
		},
		{
			name:       "publish error returns error",
			agent:      "agent-a",
			to:         "agent-b",
			body:       "hi",
			allowed:    true,
			publishErr: errors.New("redis down"),
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trans := &mockTransport{
				ch:         map[string]chan transport.Delivery{},
				publishErr: tc.publishErr,
			}
			enf := &mockEnforcer{allowed: tc.allowed, err: tc.enforcerErr}
			g := newTestGateway(t, trans, enf)

			in := sendMessageIn{Agent: tc.agent, To: tc.to, Body: tc.body}
			_, out, err := g.handleSendMessage(context.Background(), nil, in)

			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v; wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if out.Status != tc.wantStatus {
					t.Errorf("status = %q; want %q", out.Status, tc.wantStatus)
				}
				if trans.publishTopic != tc.wantTopic {
					t.Errorf("publishTopic = %q; want %q", trans.publishTopic, tc.wantTopic)
				}
				if trans.publishDoc != nil {
					if string(trans.publishDoc.Type) != tc.wantDocType {
						t.Errorf("doc.Type = %q; want %q", trans.publishDoc.Type, tc.wantDocType)
					}
					if trans.publishDoc.From != tc.wantDocFrom {
						t.Errorf("doc.From = %q; want %q", trans.publishDoc.From, tc.wantDocFrom)
					}
					if len(trans.publishDoc.To) == 0 || string(trans.publishDoc.To[0]) != tc.wantDocTo {
						t.Errorf("doc.To = %v; want [%q]", trans.publishDoc.To, tc.wantDocTo)
					}
					if trans.publishDoc.Body != tc.wantBody {
						t.Errorf("doc.Body = %q; want %q", trans.publishDoc.Body, tc.wantBody)
					}
				}
			}
		})
	}
}

// --- receive_message tests ------------------------------------------------

func TestHandleReceiveMessage(t *testing.T) {
	msgDoc := &document.Document{
		Envelope: document.Envelope{
			ID:   "msg-1",
			Type: "agent.message",
			From: "agent-b",
		},
		Body: "hello from b",
	}

	tests := []struct {
		name         string
		agent        string
		allowed      bool
		enforcerErr  error
		subscribeErr error
		ackErr       error
		delivery     *transport.Delivery
		wantErr      bool
		wantFrom     string
		wantBody     string
		wantAcked    bool
	}{
		{
			name:      "message delivered and acked immediately",
			agent:     "agent-a",
			allowed:   true,
			delivery:  &transport.Delivery{Doc: msgDoc, MsgID: "redis-1-0"},
			wantFrom:  "agent-b",
			wantBody:  "hello from b",
			wantAcked: true,
		},
		{
			name:    "invalid agent id returns error",
			agent:   "",
			wantErr: true,
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
			name:         "subscribe error returns error",
			agent:        "agent-a",
			allowed:      true,
			subscribeErr: errors.New("redis down"),
			wantErr:      true,
		},
		{
			name:      "no message available returns empty without error",
			agent:     "agent-a",
			allowed:   true,
			wantFrom:  "",
			wantBody:  "",
			wantAcked: false,
		},
		{
			name:      "ack error returns error",
			agent:     "agent-a",
			allowed:   true,
			ackErr:    errors.New("ack failed"),
			delivery:  &transport.Delivery{Doc: msgDoc, MsgID: "redis-1-0"},
			wantErr:   true,
			wantAcked: true,
		},
		{
			name:    "oversized received body returns error",
			agent:   "agent-a",
			allowed: true,
			delivery: &transport.Delivery{
				Doc: &document.Document{
					Envelope: document.Envelope{ID: "big-1", Type: "agent.message", From: "agent-b"},
					Body:     string(make([]byte, document.MaxDocumentBytes+1)),
				},
				MsgID: "redis-2-0",
			},
			wantErr:   true,
			wantAcked: true, // Ack fires before the size check returns error
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
					"agent.messages.agent-a": ch,
				},
				subscribeErr: tc.subscribeErr,
				ackErr:       tc.ackErr,
			}
			enf := &mockEnforcer{allowed: tc.allowed, err: tc.enforcerErr}
			g := newTestGateway(t, trans, enf)

			in := receiveMessageIn{Agent: tc.agent}
			_, out, err := g.handleReceiveMessage(context.Background(), nil, in)

			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v; wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if out.From != tc.wantFrom {
					t.Errorf("from = %q; want %q", out.From, tc.wantFrom)
				}
				if out.Body != tc.wantBody {
					t.Errorf("body = %q; want %q", out.Body, tc.wantBody)
				}
			}
			if trans.acked != tc.wantAcked {
				t.Errorf("acked = %v; want %v", trans.acked, tc.wantAcked)
			}
		})
	}
}

// --- logging assertions for tool handlers ------------------------------------

// makeDelivery builds a minimal transport.Delivery for use in handler tests.
// Uses document.Document directly to avoid builder validation requiring all mandatory fields.
func makeDelivery(from, msgType, body string) transport.Delivery {
	doc := &document.Document{
		Envelope: document.Envelope{
			ID:   "env-test",
			Type: protocol.MessageType(msgType),
			From: from,
		},
		Body: body,
	}
	return transport.Delivery{MsgID: "msg-1", Doc: doc}
}

func TestHandleCompleteTask_Logs(t *testing.T) {
	trans := &mockTransport{}
	enf := &mockEnforcer{allowed: true}
	g, spy := newTestGatewayWithSpy(t, trans, enf)

	g.pending.Store("task-1", "agent.tasks.agent-a", "redis-1")
	in := completeTaskIn{TaskID: "task-1", Agent: "agent-a", Result: "done"}
	_, _, err := g.handleCompleteTask(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, ok := spy.last()
	if !ok {
		t.Fatal("expected log entry, got none")
	}
	if entry.Direction != "send" {
		t.Errorf("Direction = %q, want send", entry.Direction)
	}
	if entry.ToolName != "complete_task" {
		t.Errorf("ToolName = %q, want complete_task", entry.ToolName)
	}
	if entry.FromAgent != "agent-a" {
		t.Errorf("FromAgent = %q, want agent-a", entry.FromAgent)
	}
	if entry.ToAgent != "orchestrator" {
		t.Errorf("ToAgent = %q, want orchestrator", entry.ToAgent)
	}
	if entry.MsgType != string(protocol.TaskResult) {
		t.Errorf("MsgType = %q, want %q", entry.MsgType, string(protocol.TaskResult))
	}
}

func TestHandleFailTask_Logs(t *testing.T) {
	trans := &mockTransport{}
	enf := &mockEnforcer{allowed: true}
	g, spy := newTestGatewayWithSpy(t, trans, enf)

	g.pending.Store("task-2", "agent.tasks.agent-a", "redis-2")
	in := failTaskIn{TaskID: "task-2", Agent: "agent-a", Error: "boom"}
	_, _, err := g.handleFailTask(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, ok := spy.last()
	if !ok {
		t.Fatal("expected log entry, got none")
	}
	if entry.Direction != "send" {
		t.Errorf("Direction = %q, want send", entry.Direction)
	}
	if entry.ToolName != "fail_task" {
		t.Errorf("ToolName = %q, want fail_task", entry.ToolName)
	}
	if entry.FromAgent != "agent-a" {
		t.Errorf("FromAgent = %q, want agent-a", entry.FromAgent)
	}
	if entry.ToAgent != "orchestrator" {
		t.Errorf("ToAgent = %q, want orchestrator", entry.ToAgent)
	}
	if entry.MsgType != string(protocol.TaskFail) {
		t.Errorf("MsgType = %q, want %q", entry.MsgType, string(protocol.TaskFail))
	}
}

func TestHandleGetTask_Logs(t *testing.T) {
	ch := make(chan transport.Delivery, 1)
	ch <- makeDelivery("orchestrator", "task.request", "do work")
	trans := &mockTransport{ch: map[string]chan transport.Delivery{"agent.tasks.agent-a": ch}}
	enf := &mockEnforcer{allowed: true}
	g, spy := newTestGatewayWithSpy(t, trans, enf)

	_, _, err := g.handleGetTask(context.Background(), nil, getTaskIn{Agent: "agent-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, ok := spy.last()
	if !ok {
		t.Fatal("expected log entry, got none")
	}
	if entry.Direction != "receive" {
		t.Errorf("Direction = %q, want receive", entry.Direction)
	}
	if entry.ToolName != "get_task" {
		t.Errorf("ToolName = %q, want get_task", entry.ToolName)
	}
	if entry.ToAgent != "agent-a" {
		t.Errorf("ToAgent = %q, want agent-a", entry.ToAgent)
	}
}

func TestHandleSendMessage_Logs(t *testing.T) {
	trans := &mockTransport{}
	enf := &mockEnforcer{allowed: true}
	g, spy := newTestGatewayWithSpy(t, trans, enf)

	in := sendMessageIn{Agent: "agent-a", To: "agent-b", Body: "hello"}
	_, _, err := g.handleSendMessage(context.Background(), nil, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, ok := spy.last()
	if !ok {
		t.Fatal("expected log entry, got none")
	}
	if entry.Direction != "send" {
		t.Errorf("Direction = %q, want send", entry.Direction)
	}
	if entry.ToolName != "send_message" {
		t.Errorf("ToolName = %q, want send_message", entry.ToolName)
	}
	if entry.FromAgent != "agent-a" || entry.ToAgent != "agent-b" {
		t.Errorf("FromAgent/ToAgent = %q/%q, want agent-a/agent-b", entry.FromAgent, entry.ToAgent)
	}
}

func TestHandleReceiveMessage_Logs(t *testing.T) {
	ch := make(chan transport.Delivery, 1)
	ch <- makeDelivery("agent-b", "agent.message", "reply")
	trans := &mockTransport{ch: map[string]chan transport.Delivery{"agent.messages.agent-a": ch}}
	enf := &mockEnforcer{allowed: true}
	g, spy := newTestGatewayWithSpy(t, trans, enf)

	_, _, err := g.handleReceiveMessage(context.Background(), nil, receiveMessageIn{Agent: "agent-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, ok := spy.last()
	if !ok {
		t.Fatal("expected log entry, got none")
	}
	if entry.Direction != "receive" {
		t.Errorf("Direction = %q, want receive", entry.Direction)
	}
	if entry.ToolName != "receive_message" {
		t.Errorf("ToolName = %q, want receive_message", entry.ToolName)
	}
}

// --- mocks for limits + policies tests -----------------------------------

// mockLimitEnforcer records calls and returns configurable results.
type mockLimitEnforcer struct {
	checkSendErr       error
	checkPendingErr    error
	effectiveLimits    *LimitInfo
	effectiveLimitsErr error
}

func (m *mockLimitEnforcer) CheckSend(_ context.Context, _ string, _ int) error {
	return m.checkSendErr
}
func (m *mockLimitEnforcer) RecordSend(_ context.Context, _ string) {}
func (m *mockLimitEnforcer) CheckPendingTasks(_ context.Context, _ string, _ int) error {
	return m.checkPendingErr
}
func (m *mockLimitEnforcer) GetStreamMaxLen(_ context.Context, _ string) int64 { return 0 }
func (m *mockLimitEnforcer) GetEffectiveLimits(_ context.Context, _ string) (*LimitInfo, error) {
	return m.effectiveLimits, m.effectiveLimitsErr
}

// --- handleGetMyLimits tests -------------------------------------------

func TestHandleGetMyLimits(t *testing.T) {
	tests := []struct {
		name            string
		agent           string
		limiterLimits   *LimitInfo
		limiterErr      error
		wantErr         bool
		wantBodyBytes   int
		wantSendPerMin  int
		wantPendingTask int
		wantStreamLen   int64
	}{
		{
			name:  "invalid agent id",
			agent: "!!bad",
			wantErr: true,
		},
		{
			name:       "limiter error",
			agent:      "agent-a",
			limiterErr: errors.New("db down"),
			wantErr:    true,
		},
		{
			name:  "returns effective limits",
			agent: "agent-a",
			limiterLimits: &LimitInfo{
				MaxBodyBytes:    65536,
				MaxSendPerMin:   60,
				MaxPendingTasks: 10,
				MaxStreamLen:    1000,
			},
			wantBodyBytes:   65536,
			wantSendPerMin:  60,
			wantPendingTask: 10,
			wantStreamLen:   1000,
		},
		{
			name:  "zero limits (unlimited)",
			agent: "agent-a",
			limiterLimits: &LimitInfo{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lim := &mockLimitEnforcer{
				effectiveLimits:    tc.limiterLimits,
				effectiveLimitsErr: tc.limiterErr,
			}
			cfg := DefaultConfig()
			cfg.TaskReadTimeout = 50 * time.Millisecond
			g := New(
				&mockTransport{ch: map[string]chan transport.Delivery{}},
				&mockEnforcer{allowed: true},
				nil,
				NewNoopMessageLogger(),
				lim,
				cfg,
			)

			_, out, err := g.handleGetMyLimits(context.Background(), nil, getMyLimitsIn{Agent: tc.agent})
			if (err != nil) != tc.wantErr {
				t.Fatalf("handleGetMyLimits() error = %v; wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if out.MaxBodyBytes != tc.wantBodyBytes {
				t.Errorf("MaxBodyBytes = %d; want %d", out.MaxBodyBytes, tc.wantBodyBytes)
			}
			if out.MaxSendPerMin != tc.wantSendPerMin {
				t.Errorf("MaxSendPerMin = %d; want %d", out.MaxSendPerMin, tc.wantSendPerMin)
			}
			if out.MaxPendingTasks != tc.wantPendingTask {
				t.Errorf("MaxPendingTasks = %d; want %d", out.MaxPendingTasks, tc.wantPendingTask)
			}
			if out.MaxStreamLen != tc.wantStreamLen {
				t.Errorf("MaxStreamLen = %d; want %d", out.MaxStreamLen, tc.wantStreamLen)
			}
		})
	}
}

// --- handleGetMyPolicies tests ----------------------------------------

// queryingEnforcer wraps mockEnforcer and also satisfies PolicyQuerier.
type queryingEnforcer struct {
	allowed  bool
	policies []admin.CommunicationPolicy
	listErr  error
}

func (q *queryingEnforcer) Allowed(_ context.Context, _, _ string) (bool, error) {
	return q.allowed, nil
}
func (q *queryingEnforcer) ListPoliciesFor(_ context.Context, _ string) ([]admin.CommunicationPolicy, error) {
	return q.policies, q.listErr
}

func TestHandleGetMyPolicies(t *testing.T) {
	tests := []struct {
		name        string
		agent       string
		enforcer    PolicyEnforcer
		wantErr     bool
		wantLen     int
		wantSource  string
		wantTarget  string
	}{
		{
			name:     "invalid agent id",
			agent:    "!!bad",
			enforcer: &mockEnforcer{allowed: true},
			wantErr:  true,
		},
		{
			name:     "enforcer does not implement PolicyQuerier",
			agent:    "agent-a",
			enforcer: &mockEnforcer{allowed: true},
			wantLen:  0, // returns empty list, no error
		},
		{
			name:  "querier returns error",
			agent: "agent-a",
			enforcer: &queryingEnforcer{
				allowed: true,
				listErr: errors.New("db error"),
			},
			wantErr: true,
		},
		{
			name:  "returns matching policies",
			agent: "agent-a",
			enforcer: &queryingEnforcer{
				allowed: true,
				policies: []admin.CommunicationPolicy{
					{
						ID:        "pol-1",
						SourceID:  "agent-a",
						TargetID:  "*",
						Direction: admin.DirectionBidirectional,
						Action:    admin.PolicyActionAllow,
						Priority:  10,
					},
				},
			},
			wantLen:    1,
			wantSource: "agent-a",
			wantTarget: "*",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.TaskReadTimeout = 50 * time.Millisecond
			g := New(
				&mockTransport{ch: map[string]chan transport.Delivery{}},
				tc.enforcer,
				nil,
				NewNoopMessageLogger(),
				nil,
				cfg,
			)

			_, out, err := g.handleGetMyPolicies(context.Background(), nil, getMyPoliciesIn{Agent: tc.agent})
			if (err != nil) != tc.wantErr {
				t.Fatalf("handleGetMyPolicies() error = %v; wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(out.Policies) != tc.wantLen {
				t.Fatalf("len(Policies) = %d; want %d", len(out.Policies), tc.wantLen)
			}
			if tc.wantLen > 0 {
				if out.Policies[0].SourceID != tc.wantSource {
					t.Errorf("SourceID = %q; want %q", out.Policies[0].SourceID, tc.wantSource)
				}
				if out.Policies[0].TargetID != tc.wantTarget {
					t.Errorf("TargetID = %q; want %q", out.Policies[0].TargetID, tc.wantTarget)
				}
			}
		})
	}
}
