package protocol_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/protocol"
)

// allTypes lists every MessageType constant the spec requires.
var allTypes = []protocol.MessageType{ //nolint:gochecknoglobals
	// Agent evolution (11)
	protocol.AgentGenome,
	protocol.AgentSpawnProposal,
	protocol.AgentSpawnApproval,
	protocol.AgentSpawnRejection,
	protocol.AgentEvaluationRequest,
	protocol.AgentEvaluationResult,
	protocol.AgentPromotion,
	protocol.AgentRollback,
	protocol.AgentRetirement,
	protocol.AgentQuarantine,
	protocol.AgentRecombineProposal,
	// Task (5)
	protocol.TaskRequest,
	protocol.TaskResult,
	protocol.TaskFail,
	protocol.TaskProgress,
	protocol.AgentMessage,
	// Negotiation (8)
	protocol.NegotiationPropose,
	protocol.NegotiationAccept,
	protocol.NegotiationReject,
	protocol.NegotiationCounter,
	protocol.NegotiationClarify,
	protocol.NegotiationDelegate,
	protocol.NegotiationCommit,
	protocol.NegotiationAbort,
}

func TestRegistry_AllTypesPresent(t *testing.T) {
	for _, mt := range allTypes {
		if _, ok := protocol.Lookup(mt); !ok {
			t.Errorf("registry missing type %q", mt)
		}
	}
}

func TestRegistry_Count(t *testing.T) {
	want := 24
	got := len(protocol.Registered())
	if got != want {
		t.Errorf("registry len = %d, want %d", got, want)
	}
}

func TestRegistry_NonEmptyRequiredFields(t *testing.T) {
	for _, mt := range protocol.Registered() {
		meta, _ := protocol.Lookup(mt)
		if len(meta.RequiredFields) == 0 {
			t.Errorf("registry[%q].RequiredFields is empty", mt)
		}
	}
}

func TestProtocolVersion(t *testing.T) {
	if protocol.ProtocolVersion != "v1" {
		t.Errorf("ProtocolVersion = %q, want %q", protocol.ProtocolVersion, "v1")
	}
}

func TestBaseEnvelopeFields(t *testing.T) {
	want := []string{"type", "version"}
	got := protocol.BaseEnvelopeFields()
	if len(got) != len(want) {
		t.Fatalf("BaseEnvelopeFields() = %v, want %v", got, want)
	}
	for i, f := range want {
		if got[i] != f {
			t.Errorf("BaseEnvelopeFields()[%d] = %q, want %q", i, got[i], f)
		}
	}
}

func TestBaseEnvelopeFields_ReturnsCopy(t *testing.T) {
	a := protocol.BaseEnvelopeFields()
	b := protocol.BaseEnvelopeFields()
	a[0] = "mutated"
	if b[0] == "mutated" {
		t.Error("BaseEnvelopeFields() returned shared backing array — must return independent copy")
	}
}

func TestRegistry_SpecificTypes(t *testing.T) {
	tests := []struct {
		mt            protocol.MessageType
		wantFields    []string
		wantSchemaRef string
	}{
		{
			mt: protocol.AgentGenome,
			wantFields: []string{
				"agent_id", "kind", "status", "identity", "capabilities",
				"tools", "model_policy", "prompt_policy", "routing_policy",
				"thresholds", "constraints", "mutation_policy",
			},
			wantSchemaRef: "agentGenome",
		},
		{
			mt:            protocol.TaskRequest,
			wantFields:    []string{"id", "from", "to", "exec_id", "step"},
			wantSchemaRef: "",
		},
		{
			mt:            protocol.AgentMessage,
			wantFields:    []string{"id", "from", "to"},
			wantSchemaRef: "",
		},
		{
			mt:            protocol.NegotiationPropose,
			wantFields:    []string{"id", "from", "to", "thread_id"},
			wantSchemaRef: "",
		},
		{
			mt:            protocol.NegotiationAccept,
			wantFields:    []string{"id", "from", "to", "thread_id", "in_reply_to"},
			wantSchemaRef: "",
		},
	}
	for _, tc := range tests {
		meta, ok := protocol.Lookup(tc.mt)
		if !ok {
			t.Errorf("registry missing %q", tc.mt)
			continue
		}
		if meta.SchemaRef != tc.wantSchemaRef {
			t.Errorf("registry[%q].SchemaRef = %q, want %q", tc.mt, meta.SchemaRef, tc.wantSchemaRef)
		}
		fieldSet := make(map[string]bool, len(meta.RequiredFields))
		for _, f := range meta.RequiredFields {
			fieldSet[f] = true
		}
		for _, f := range tc.wantFields {
			if !fieldSet[f] {
				t.Errorf("registry[%q].RequiredFields missing %q", tc.mt, f)
			}
		}
	}
}
