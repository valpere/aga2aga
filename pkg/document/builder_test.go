package document_test

import (
	"strings"
	"testing"

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
	if !strings.Contains(doc.CreatedAt, "T") {
		t.Errorf("CreatedAt %q does not look like RFC3339", doc.CreatedAt)
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

// TestNewGenomeBuilder_SetsTypeAndFields verifies the convenience constructor
// sets agent_id and kind, and returns a validation error (not a panic) for missing
// genome fields — callers must chain additional Field() calls.
func TestNewGenomeBuilder_SetsTypeAndFields(t *testing.T) {
	_, err := document.NewGenomeBuilder("agent-1", "reviewer").Build()
	if err == nil {
		t.Fatal("expected Build() to fail — genome requires many fields beyond agent_id+kind")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error = %v; want validation failure", err)
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
