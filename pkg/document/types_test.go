package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestStringOrList_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  document.StringOrList
	}{
		{
			name:  "single string",
			input: `to: "alice"`,
			want:  document.StringOrList{"alice"},
		},
		{
			name:  "list of strings",
			input: "to:\n  - alice\n  - bob",
			want:  document.StringOrList{"alice", "bob"},
		},
		{
			name:  "empty string",
			input: `to: ""`,
			want:  document.StringOrList{""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dest struct {
				To document.StringOrList `yaml:"to"`
			}
			if err := yaml.Unmarshal([]byte(tc.input), &dest); err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v", err)
			}
			if len(dest.To) != len(tc.want) {
				t.Fatalf("StringOrList len = %d, want %d; got %v", len(dest.To), len(tc.want), dest.To)
			}
			for i := range tc.want {
				if dest.To[i] != tc.want[i] {
					t.Errorf("StringOrList[%d] = %q, want %q", i, dest.To[i], tc.want[i])
				}
			}
		})
	}
}

func TestAs_RoundTrip(t *testing.T) {
	type Inner struct {
		AgentID string `yaml:"agent_id"`
		Kind    string `yaml:"kind"`
	}

	doc := &document.Document{}
	doc.Extra = map[string]any{
		"agent_id": "agent-42",
		"kind":     "reviewer",
	}

	got, err := document.As[Inner](doc)
	if err != nil {
		t.Fatalf("As() error = %v", err)
	}
	if got.AgentID != "agent-42" {
		t.Errorf("AgentID = %q, want %q", got.AgentID, "agent-42")
	}
	if got.Kind != "reviewer" {
		t.Errorf("Kind = %q, want %q", got.Kind, "reviewer")
	}
}

func TestDocument_EnvelopeFields(t *testing.T) {
	raw := `type: task.request
version: v1
id: msg-001
from: agent-a
to: agent-b
exec_id: exec-1
step: parse
`
	var doc document.Document
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if string(doc.Type) != "task.request" {
		t.Errorf("Type = %q, want %q", doc.Type, "task.request")
	}
	if doc.ID != "msg-001" {
		t.Errorf("ID = %q, want %q", doc.ID, "msg-001")
	}
	if len(doc.To) != 1 || doc.To[0] != "agent-b" {
		t.Errorf("To = %v, want [agent-b]", doc.To)
	}
	if doc.ExecID != "exec-1" {
		t.Errorf("ExecID = %q, want %q", doc.ExecID, "exec-1")
	}
}
