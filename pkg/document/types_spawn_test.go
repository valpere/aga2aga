package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestSpawnProposal_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.spawn.proposal
version: v1
id: msg-spawn-1
from: meta-evolver
candidate_id: agent-candidate-42
parent_ids:
  - agent-parent-1
spawn_reason: performance improvement in code review tasks
status: proposed
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	proposal, err := document.As[document.SpawnProposal](&doc)
	if err != nil {
		t.Fatalf("As[SpawnProposal]() error = %v", err)
	}

	if proposal.CandidateID != "agent-candidate-42" {
		t.Errorf("CandidateID = %q, want %q", proposal.CandidateID, "agent-candidate-42")
	}

	if len(proposal.ParentIDs) != 1 || proposal.ParentIDs[0] != "agent-parent-1" {
		t.Errorf("ParentIDs = %v, want [agent-parent-1]", proposal.ParentIDs)
	}

	if proposal.SpawnReason != "performance improvement in code review tasks" {
		t.Errorf("SpawnReason = %q", proposal.SpawnReason)
	}
}

func TestSpawnApproval_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.spawn.approval
version: v1
id: msg-approval-1
from: safety-auditor
in_reply_to: msg-spawn-1
candidate_id: agent-candidate-42
decision: approved_for_sandbox
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	approval, err := document.As[document.SpawnApproval](&doc)
	if err != nil {
		t.Fatalf("As[SpawnApproval]() error = %v", err)
	}

	if approval.Decision != "approved_for_sandbox" {
		t.Errorf("Decision = %q, want %q", approval.Decision, "approved_for_sandbox")
	}

	if approval.CandidateID != "agent-candidate-42" {
		t.Errorf("CandidateID = %q, want %q", approval.CandidateID, "agent-candidate-42")
	}

	// in_reply_to is an Envelope field — consumers MUST read it from doc.Envelope.InReplyTo.
	// SpawnApproval must NOT have an InReplyTo field (it would always be empty after As[T]).
	if doc.InReplyTo != "msg-spawn-1" {
		t.Errorf("Envelope.InReplyTo = %q, want msg-spawn-1", doc.InReplyTo)
	}
}

// TestSpawnProposal_GenomePatch_MutableFields verifies that valid mutable fields in
// genome_patch round-trip correctly through As[SpawnProposal].
// Flat (non-table-driven): single adversarial fixture whose clarity outweighs the
// overhead of wrapping in []struct{}. Add a table row if new field coverage is needed.
func TestSpawnProposal_GenomePatch_MutableFields(t *testing.T) {
	t.Parallel()

	raw := `type: agent.spawn.proposal
version: v1
id: msg-spawn-2
from: meta-evolver
candidate_id: agent-candidate-99
parent_ids:
  - agent-parent-1
spawn_reason: improve routing coverage
genome_patch:
  capabilities:
    skills:
      - code-review
      - testing
  soft_constraints:
    - prefer-fast-response
  tags:
    - experimental
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	proposal, err := document.As[document.SpawnProposal](&doc)
	if err != nil {
		t.Fatalf("As[SpawnProposal]() error = %v", err)
	}

	if proposal.GenomePatch == nil {
		t.Fatal("GenomePatch is nil, want non-nil")
	}

	if proposal.GenomePatch.Capabilities == nil {
		t.Fatal("GenomePatch.Capabilities is nil")
	}

	if len(proposal.GenomePatch.Capabilities.Skills) != 2 {
		t.Errorf("GenomePatch.Capabilities.Skills = %v, want 2 items", proposal.GenomePatch.Capabilities.Skills)
	}

	if len(proposal.GenomePatch.SoftConstraints) != 1 || proposal.GenomePatch.SoftConstraints[0] != "prefer-fast-response" {
		t.Errorf("GenomePatch.SoftConstraints = %v, want [prefer-fast-response]", proposal.GenomePatch.SoftConstraints)
	}

	if len(proposal.GenomePatch.Tags) != 1 || proposal.GenomePatch.Tags[0] != "experimental" {
		t.Errorf("GenomePatch.Tags = %v, want [experimental]", proposal.GenomePatch.Tags)
	}
}

// TestSpawnProposal_GenomePatch_DoNotTouchFieldsDropped is a security regression test.
// It verifies that DO_NOT_TOUCH fields injected into genome_patch YAML are structurally
// dropped by the typed GenomePatch struct and are never accessible to callers.
// Flat (non-table-driven): single adversarial payload whose inline YAML is the spec.
// DO NOT DELETE — this test locks the CWE-915 fix from issue #30.
func TestSpawnProposal_GenomePatch_DoNotTouchFieldsDropped(t *testing.T) {
	t.Parallel()

	// Attacker injects DO_NOT_TOUCH fields into genome_patch.
	// The typed struct must not have fields for them — they simply do not unmarshal.
	raw := `type: agent.spawn.proposal
version: v1
id: msg-spawn-3
from: meta-evolver
candidate_id: agent-candidate-99
parent_ids:
  - agent-parent-1
spawn_reason: malicious patch attempt
genome_patch:
  agent_id: attacker-controlled-id
  kind: attacker-kind
  identity:
    public_key: attacker-pubkey
  constraints:
    hard:
      - do-anything
  mutation_policy:
    forbidden: []
  tags:
    - legitimate-tag
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	proposal, err := document.As[document.SpawnProposal](&doc)
	if err != nil {
		t.Fatalf("As[SpawnProposal]() error = %v", err)
	}

	if proposal.GenomePatch == nil {
		t.Fatal("GenomePatch is nil, want non-nil (has tags)")
	}

	// DO_NOT_TOUCH fields must not be accessible through GenomePatch.
	// The struct simply has no such fields — they are silently dropped on unmarshal.
	// Verify the only accessible field is the legitimate one.
	if len(proposal.GenomePatch.Tags) != 1 || proposal.GenomePatch.Tags[0] != "legitimate-tag" {
		t.Errorf("GenomePatch.Tags = %v, want [legitimate-tag]", proposal.GenomePatch.Tags)
	}
}

func TestSpawnRejection_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.spawn.rejection
version: v1
id: msg-reject-1
from: safety-auditor
in_reply_to: msg-spawn-1
candidate_id: agent-candidate-42
decision: rejected
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	rejection, err := document.As[document.SpawnRejection](&doc)
	if err != nil {
		t.Fatalf("As[SpawnRejection]() error = %v", err)
	}

	if rejection.Decision != "rejected" {
		t.Errorf("Decision = %q, want %q", rejection.Decision, "rejected")
	}
}
