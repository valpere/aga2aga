package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestStringOrList_UnmarshalYAML(t *testing.T) {
	t.Parallel()

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
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var dest struct {
				To document.StringOrList `yaml:"to"`
			}

			if err := yaml.Unmarshal([]byte(testCase.input), &dest); err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v", err)
			}

			if len(dest.To) != len(testCase.want) {
				t.Fatalf("StringOrList len = %d, want %d; got %v", len(dest.To), len(testCase.want), dest.To)
			}

			for i := range testCase.want {
				if dest.To[i] != testCase.want[i] {
					t.Errorf("StringOrList[%d] = %q, want %q", i, dest.To[i], testCase.want[i])
				}
			}
		})
	}
}

func TestStringOrList_UnmarshalYAML_Error(t *testing.T) {
	t.Parallel()

	// A YAML mapping node (map value) must be rejected — StringOrList only accepts scalar or sequence.
	input := "to:\n  key: value"

	var dest struct {
		To document.StringOrList `yaml:"to"`
	}

	if err := yaml.Unmarshal([]byte(input), &dest); err == nil {
		t.Errorf("yaml.Unmarshal() expected error for mapping node, got nil")
	}
}

func TestStringOrList_NullBecomesEmpty(t *testing.T) {
	t.Parallel()

	// YAML null (~) must not silently route to recipient "".
	// It must produce an empty slice.
	input := "to: ~"

	var dest struct {
		To document.StringOrList `yaml:"to"`
	}

	if err := yaml.Unmarshal([]byte(input), &dest); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if len(dest.To) != 0 {
		t.Errorf("YAML null should produce empty StringOrList, got %v", dest.To)
	}
}

func TestAs_StripsEnvelopeKeys(t *testing.T) {
	t.Parallel()

	type Inner struct {
		From    string `yaml:"from"`
		AgentID string `yaml:"agent_id"`
	}

	// Simulate Extra that somehow contains an envelope key alongside a payload key.
	// As[T] must strip envelope keys so they cannot bleed into typed structs.
	doc := &document.Document{
		Envelope: document.Envelope{
			Type:         "",
			Version:      "",
			ID:           "",
			From:         "legit-sender",
			To:           nil,
			CreatedAt:    "",
			InReplyTo:    "",
			ThreadID:     "",
			ExecID:       "",
			TTL:          "",
			Status:       "",
			Signature:    "",
			SigningKeyID: "",
		},
		Extra: map[string]any{
			"from":     "injected",
			"agent_id": "agent-42",
		},
		Body: "",
		Raw:  nil,
	}

	got, err := document.As[Inner](doc)
	if err != nil {
		t.Fatalf("As[Inner]() error = %v", err)
	}

	// Envelope key "from" in Extra must NOT bleed into the typed struct.
	if got.From != "" {
		t.Errorf("As[T] leaked envelope key into typed struct: From = %q, want empty", got.From)
	}

	if got.AgentID != "agent-42" {
		t.Errorf("AgentID = %q, want agent-42", got.AgentID)
	}
}

func TestAs_RoundTrip(t *testing.T) {
	t.Parallel()

	type Inner struct {
		AgentID string `yaml:"agent_id"`
		Kind    string `yaml:"kind"`
	}

	doc := &document.Document{
		Envelope: document.Envelope{
			Type:         "",
			Version:      "",
			ID:           "",
			From:         "",
			To:           nil,
			CreatedAt:    "",
			InReplyTo:    "",
			ThreadID:     "",
			ExecID:       "",
			TTL:          "",
			Status:       "",
			Signature:    "",
			SigningKeyID: "",
		},
		Extra: nil,
		Body:  "",
		Raw:   nil,
	}
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

func TestStringOrList_MarshalYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		list document.StringOrList
		want string
	}{
		{name: "single element marshals as scalar", list: document.StringOrList{"alice"}, want: "to: alice\n"},
		{name: "multi element marshals as sequence", list: document.StringOrList{"alice", "bob"}, want: "to:\n    - alice\n    - bob\n"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			type envelope struct {
				To document.StringOrList `yaml:"to"`
			}

			got, err := yaml.Marshal(envelope{To: testCase.list})
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}

			if string(got) != testCase.want {
				t.Errorf("Marshal() = %q, want %q", string(got), testCase.want)
			}
		})
	}
}

func TestAs_NilDoc(t *testing.T) {
	t.Parallel()

	type Inner struct {
		AgentID string `yaml:"agent_id"`
	}

	_, err := document.As[Inner](nil)
	if err == nil {
		t.Errorf("As[Inner](nil) expected error, got nil")
	}
}

func TestAs_EmptyExtra(t *testing.T) {
	t.Parallel()

	type Inner struct {
		AgentID string `yaml:"agent_id"`
	}

	doc := &document.Document{
		Envelope: document.Envelope{},
		Extra:    nil,
		Body:     "",
		Raw:      nil,
	}

	got, err := document.As[Inner](doc)
	if err != nil {
		t.Fatalf("As[Inner](empty extra) error = %v", err)
	}

	if got.AgentID != "" {
		t.Errorf("AgentID = %q, want empty", got.AgentID)
	}
}

func TestEnvelope_SigningKeyID(t *testing.T) {
	t.Parallel()

	raw := `type: task.request
version: v1
id: msg-signed-1
from: agent-a
to: agent-b
signing_key_id: key-ed25519-abc123
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if doc.SigningKeyID != "key-ed25519-abc123" {
		t.Errorf("SigningKeyID = %q, want key-ed25519-abc123", doc.SigningKeyID)
	}

	// signing_key_id must NOT appear in Extra (it belongs to the Envelope).
	if _, ok := doc.Extra["signing_key_id"]; ok {
		t.Errorf("signing_key_id leaked into Extra; should be captured by Envelope")
	}
}

func TestDocument_EnvelopeFields(t *testing.T) {
	t.Parallel()

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
