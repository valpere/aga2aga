package protocol_test

import (
	"testing"

	. "github.com/valpere/aga2aga/pkg/protocol"
)

// allTypes lists every MessageType constant the spec requires.
var allTypes = []MessageType{
	// Agent evolution (11)
	AgentGenome,
	AgentSpawnProposal,
	AgentSpawnApproval,
	AgentSpawnRejection,
	AgentEvaluationRequest,
	AgentEvaluationResult,
	AgentPromotion,
	AgentRollback,
	AgentRetirement,
	AgentQuarantine,
	AgentRecombineProposal,
	// Task (5)
	TaskRequest,
	TaskResult,
	TaskFail,
	TaskProgress,
	AgentMessage,
	// Negotiation (8)
	NegotiationPropose,
	NegotiationAccept,
	NegotiationReject,
	NegotiationCounter,
	NegotiationClarify,
	NegotiationDelegate,
	NegotiationCommit,
	NegotiationAbort,
}

func TestRegistry_AllTypesPresent(t *testing.T) {
	for _, mt := range allTypes {
		if _, ok := Registry[mt]; !ok {
			t.Errorf("Registry missing type %q", mt)
		}
	}
}

func TestRegistry_Count(t *testing.T) {
	want := 24
	got := len(Registry)
	if got != want {
		t.Errorf("Registry len = %d, want %d", got, want)
	}
}

func TestRegistry_NonEmptyRequiredFields(t *testing.T) {
	for mt, meta := range Registry {
		if len(meta.RequiredFields) == 0 {
			t.Errorf("Registry[%q].RequiredFields is empty", mt)
		}
	}
}

func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion != "v1" {
		t.Errorf("ProtocolVersion = %q, want %q", ProtocolVersion, "v1")
	}
}

func TestBaseEnvelopeFields(t *testing.T) {
	want := []string{"type", "version"}
	if len(BaseEnvelopeFields) != len(want) {
		t.Fatalf("BaseEnvelopeFields = %v, want %v", BaseEnvelopeFields, want)
	}
	for i, f := range want {
		if BaseEnvelopeFields[i] != f {
			t.Errorf("BaseEnvelopeFields[%d] = %q, want %q", i, BaseEnvelopeFields[i], f)
		}
	}
}

func TestRegistry_SpecificTypes(t *testing.T) {
	tests := []struct {
		mt            MessageType
		wantFields    []string
		wantSchemaRef string
	}{
		{
			mt: AgentGenome,
			wantFields: []string{
				"agent_id", "kind", "status", "identity", "capabilities",
				"tools", "model_policy", "prompt_policy", "routing_policy",
				"thresholds", "constraints", "mutation_policy",
			},
			wantSchemaRef: "agentGenome",
		},
		{
			mt:            TaskRequest,
			wantFields:    []string{"id", "from", "to", "exec_id", "step"},
			wantSchemaRef: "",
		},
		{
			mt:            NegotiationPropose,
			wantFields:    []string{"id", "from", "to", "thread_id"},
			wantSchemaRef: "",
		},
		{
			mt:            NegotiationAccept,
			wantFields:    []string{"id", "from", "to", "thread_id", "in_reply_to"},
			wantSchemaRef: "",
		},
	}
	for _, tc := range tests {
		meta, ok := Registry[tc.mt]
		if !ok {
			t.Errorf("Registry missing %q", tc.mt)
			continue
		}
		if meta.SchemaRef != tc.wantSchemaRef {
			t.Errorf("Registry[%q].SchemaRef = %q, want %q", tc.mt, meta.SchemaRef, tc.wantSchemaRef)
		}
		fieldSet := make(map[string]bool, len(meta.RequiredFields))
		for _, f := range meta.RequiredFields {
			fieldSet[f] = true
		}
		for _, f := range tc.wantFields {
			if !fieldSet[f] {
				t.Errorf("Registry[%q].RequiredFields missing %q", tc.mt, f)
			}
		}
	}
}
