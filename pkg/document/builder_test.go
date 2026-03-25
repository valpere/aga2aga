package document_test

import (
	"strings"
	"testing"
	"time"

	"github.com/valpere/aga2aga/pkg/document"
	"github.com/valpere/aga2aga/pkg/protocol"
)

// minTaskRequest returns a Builder with all required task.request fields set.
// Use as a baseline; chain additional Field calls for specific test scenarios.
func minTaskRequest() *document.Builder {
	return document.NewBuilder(protocol.TaskRequest).
		ID("msg-1").
		From("orchestrator").
		To("reviewer").
		ExecID("exec-1").
		Field("step", "review")
}

// TestBuilder_ValidBuild verifies that a fully-specified task.request builds without error.
func TestBuilder_ValidBuild(t *testing.T) {
	doc, err := minTaskRequest().Body("Do the review.").Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc.ID != "msg-1" {
		t.Errorf("ID = %q, want %q", doc.ID, "msg-1")
	}
	if doc.From != "orchestrator" {
		t.Errorf("From = %q, want %q", doc.From, "orchestrator")
	}
	if doc.Body != "Do the review." {
		t.Errorf("Body = %q, want %q", doc.Body, "Do the review.")
	}
}

// TestBuilder_MissingRequired verifies that Build() returns an error when required fields are absent.
func TestBuilder_MissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		builder *document.Builder
	}{
		{
			name: "missing id",
			builder: document.NewBuilder(protocol.TaskRequest).
				From("orchestrator").To("reviewer").ExecID("e1").Field("step", "s1"),
		},
		{
			name: "missing from",
			builder: document.NewBuilder(protocol.TaskRequest).
				ID("msg-1").To("reviewer").ExecID("e1").Field("step", "s1"),
		},
		{
			name: "missing exec_id",
			builder: document.NewBuilder(protocol.TaskRequest).
				ID("msg-1").From("orch").To("reviewer").Field("step", "s1"),
		},
		{
			name: "missing step",
			builder: document.NewBuilder(protocol.TaskRequest).
				ID("msg-1").From("orch").To("reviewer").ExecID("e1"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.builder.Build()
			if err == nil {
				t.Errorf("Build() expected error for %q, got nil", tc.name)
			}
		})
	}
}

// TestBuilder_To_MultipleTargets verifies To() accepts multiple recipients.
func TestBuilder_To_MultipleTargets(t *testing.T) {
	doc, err := document.NewBuilder(protocol.TaskRequest).
		ID("msg-2").
		From("orchestrator").
		To("alice", "bob").
		ExecID("e1").
		Field("step", "s1").
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(doc.To) != 2 {
		t.Errorf("To len = %d, want 2", len(doc.To))
	}
	if doc.To[0] != "alice" || doc.To[1] != "bob" {
		t.Errorf("To = %v, want [alice bob]", []string(doc.To))
	}
}

// TestBuilder_Field_SetsExtra verifies Field() writes into Document.Extra.
func TestBuilder_Field_SetsExtra(t *testing.T) {
	doc, err := minTaskRequest().
		Field("custom_key", "custom_value").
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc.Extra["custom_key"] != "custom_value" {
		t.Errorf("Extra[custom_key] = %v, want %q", doc.Extra["custom_key"], "custom_value")
	}
}

// TestBuilder_Field_StickyError verifies only the first reserved-key violation is recorded
// and subsequent Field() calls (even with different reserved keys) are silently dropped.
func TestBuilder_Field_StickyError(t *testing.T) {
	_, err := minTaskRequest().
		Field("type", "injected").    // first violation — recorded
		Field("version", "injected"). // second violation — silently dropped
		Field("id", "injected").      // third violation — silently dropped
		Build()
	if err == nil {
		t.Fatal("Build() expected error for reserved key, got nil")
	}
	if !strings.Contains(err.Error(), "\"type\"") {
		t.Errorf("error = %v; expected first violation key %q to appear", err, "type")
	}
}

// TestBuilder_Build_MapIndependence verifies Build() copies extra so later mutations
// to the builder do not affect the returned Document.
func TestBuilder_Build_MapIndependence(t *testing.T) {
	b := minTaskRequest().Field("key1", "value1")
	doc, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	// Mutate builder after Build() — should not affect doc.Extra.
	b.Field("key2", "value2")
	if _, ok := doc.Extra["key2"]; ok {
		t.Error("doc.Extra was mutated after Build() — builder and document share the same map")
	}
}

// TestBuilder_AutoVersion verifies Build() always sets version to v1.
func TestBuilder_AutoVersion(t *testing.T) {
	doc, err := minTaskRequest().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc.Version != "v1" {
		t.Errorf("Version = %q, want v1", doc.Version)
	}
}

// TestBuilder_AutoCreatedAt verifies Build() sets created_at in RFC3339 format.
func TestBuilder_AutoCreatedAt(t *testing.T) {
	doc, err := minTaskRequest().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc.CreatedAt == "" {
		t.Fatal("CreatedAt should be auto-set")
	}
	if _, parseErr := time.Parse(time.RFC3339, doc.CreatedAt); parseErr != nil {
		t.Errorf("CreatedAt %q is not valid RFC3339: %v", doc.CreatedAt, parseErr)
	}
}

// TestBuilder_Chaining verifies all setter methods return *Builder for fluent use.
func TestBuilder_Chaining(t *testing.T) {
	// This compiles only if every method returns *Builder.
	doc, err := document.NewBuilder(protocol.TaskRequest).
		ID("x").
		From("a").
		To("b").
		ExecID("e1").
		Status("").
		InReplyTo("").
		ThreadID("").
		Field("step", "s").
		Body("hello").
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc == nil {
		t.Fatal("Build() returned nil doc")
	}
}

// TestBuilder_Field_RejectsEnvelopeKeys verifies Field() rejects reserved envelope key names.
// The reservedKeys list must match the envelopeKeys map in types.go — keep in sync.
func TestBuilder_Field_RejectsEnvelopeKeys(t *testing.T) {
	reservedKeys := []string{"type", "version", "id", "from", "to", "exec_id",
		"status", "in_reply_to", "thread_id", "created_at", "signature", "signing_key_id"}

	for _, key := range reservedKeys {
		t.Run(key, func(t *testing.T) {
			_, err := minTaskRequest().Field(key, "value").Build()
			if err == nil {
				t.Errorf("Build() expected error for reserved key %q, got nil", key)
			}
			if !strings.Contains(err.Error(), "reserved envelope key") {
				t.Errorf("error = %v; want 'reserved envelope key'", err)
			}
		})
	}
}

// TestBuilder_Status_EnvelopeField verifies Status() sets Envelope.Status correctly.
func TestBuilder_Status_EnvelopeField(t *testing.T) {
	doc, err := minTaskRequest().Status("active").Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc.Status != "active" {
		t.Errorf("Status = %q, want active", doc.Status)
	}
}

// TestBuilder_InReplyTo_EnvelopeField verifies InReplyTo() sets Envelope.InReplyTo.
func TestBuilder_InReplyTo_EnvelopeField(t *testing.T) {
	doc, err := minTaskRequest().InReplyTo("msg-0").Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc.InReplyTo != "msg-0" {
		t.Errorf("InReplyTo = %q, want msg-0", doc.InReplyTo)
	}
}

// TestBuilder_ThreadID_EnvelopeField verifies ThreadID() sets Envelope.ThreadID.
func TestBuilder_ThreadID_EnvelopeField(t *testing.T) {
	doc, err := minTaskRequest().ThreadID("thread-1").Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if doc.ThreadID != "thread-1" {
		t.Errorf("ThreadID = %q, want thread-1", doc.ThreadID)
	}
}

// TestBuilder_Build_ReportsAllErrors verifies Build() reports all validation errors, not just the first.
func TestBuilder_Build_ReportsAllErrors(t *testing.T) {
	// task.request requires id, from, to, exec_id, step — 5 missing fields → 5 errors.
	_, err := document.NewBuilder(protocol.TaskRequest).Build()
	if err == nil {
		t.Fatal("Build() expected error, got nil")
	}
	msg := err.Error()
	// Multiple errors appear in the message separated by "; ".
	if !strings.Contains(msg, "; ") {
		t.Errorf("error = %v; want multiple errors separated by '; '", msg)
	}
	// Each missing field should appear in the combined message.
	for _, field := range []string{"id", "from", "to", "exec_id", "step"} {
		if !strings.Contains(msg, field) {
			t.Errorf("error message missing field %q: %v", field, msg)
		}
	}
}

// TestNewGenomeBuilder_IncompleteReturnsError verifies an incomplete genome build
// returns a validation error (not a panic) for missing required fields.
func TestNewGenomeBuilder_IncompleteReturnsError(t *testing.T) {
	_, err := document.NewGenomeBuilder("agent-1", "reviewer").Build()
	if err == nil {
		t.Fatal("expected Build() to fail — genome requires many fields beyond agent_id+kind")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error = %v; want validation failure", err)
	}
}

// TestNewGenomeBuilder_Type verifies that NewGenomeBuilder produces an agent.genome document type.
func TestNewGenomeBuilder_Type(t *testing.T) {
	// Build a minimal valid genome by providing all required fields.
	doc, err := document.NewGenomeBuilder("agent-1", "reviewer").
		Status("proposed").
		Field("identity", map[string]any{"public_key": "pk-abc"}).
		Field("capabilities", map[string]any{"skills": []any{"review"}}).
		Field("tools", map[string]any{"allowed": []any{"read", "grep"}}).
		Field("model_policy", map[string]any{"provider": "anthropic"}).
		Field("prompt_policy", map[string]any{"profile": "default"}).
		Field("routing_policy", map[string]any{"accepts": []any{"task.request"}}).
		Field("thresholds", map[string]any{}).
		Field("constraints", map[string]any{"hard": []any{"no_exec"}}).
		Field("mutation_policy", map[string]any{"allowed": []any{"prompt_policy"}}).
		Build()
	if err != nil {
		t.Fatalf("NewGenomeBuilder full build error = %v", err)
	}
	if doc.Type != protocol.AgentGenome {
		t.Errorf("Type = %q, want %q", doc.Type, protocol.AgentGenome)
	}
	if doc.Extra["agent_id"] != "agent-1" {
		t.Errorf("agent_id = %v, want agent-1", doc.Extra["agent_id"])
	}
}

// TestNewSpawnProposalBuilder_Valid verifies a minimal valid spawn proposal builds.
func TestNewSpawnProposalBuilder_Valid(t *testing.T) {
	doc, err := document.NewSpawnProposalBuilder("parent-1", "candidate-2").
		ID("msg-sp-1").
		From("meta-evolver").
		Field("spawn_reason", "performance improvement").
		Build()

	if err != nil {
		t.Fatalf("NewSpawnProposalBuilder().Build() error = %v", err)
	}
	if doc.Extra["candidate_id"] != "candidate-2" {
		t.Errorf("candidate_id = %v, want candidate-2", doc.Extra["candidate_id"])
	}
}

// TestNewTaskRequestBuilder_Valid verifies the convenience constructor builds a valid task.request.
func TestNewTaskRequestBuilder_Valid(t *testing.T) {
	doc, err := document.NewTaskRequestBuilder("exec-1", "orchestrator").
		ID("msg-t-1").
		To("reviewer").
		Field("step", "check").
		Build()

	if err != nil {
		t.Fatalf("NewTaskRequestBuilder().Build() error = %v", err)
	}
	if doc.Type != protocol.TaskRequest {
		t.Errorf("Type = %q, want task.request", doc.Type)
	}
	if doc.ExecID != "exec-1" {
		t.Errorf("ExecID = %q, want exec-1", doc.ExecID)
	}
}
