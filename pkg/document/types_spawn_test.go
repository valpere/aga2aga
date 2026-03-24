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
