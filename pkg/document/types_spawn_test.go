package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestSpawnProposal_UnmarshalYAML(t *testing.T) {
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
	p, err := document.As[document.SpawnProposal](&doc)
	if err != nil {
		t.Fatalf("As[SpawnProposal]() error = %v", err)
	}
	if p.CandidateID != "agent-candidate-42" {
		t.Errorf("CandidateID = %q, want %q", p.CandidateID, "agent-candidate-42")
	}
	if len(p.ParentIDs) != 1 || p.ParentIDs[0] != "agent-parent-1" {
		t.Errorf("ParentIDs = %v, want [agent-parent-1]", p.ParentIDs)
	}
	if p.SpawnReason != "performance improvement in code review tasks" {
		t.Errorf("SpawnReason = %q", p.SpawnReason)
	}
}

func TestSpawnApproval_UnmarshalYAML(t *testing.T) {
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
	a, err := document.As[document.SpawnApproval](&doc)
	if err != nil {
		t.Fatalf("As[SpawnApproval]() error = %v", err)
	}
	if a.Decision != "approved_for_sandbox" {
		t.Errorf("Decision = %q, want %q", a.Decision, "approved_for_sandbox")
	}
	if a.CandidateID != "agent-candidate-42" {
		t.Errorf("CandidateID = %q, want %q", a.CandidateID, "agent-candidate-42")
	}
}

func TestSpawnRejection_UnmarshalYAML(t *testing.T) {
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
	r, err := document.As[document.SpawnRejection](&doc)
	if err != nil {
		t.Fatalf("As[SpawnRejection]() error = %v", err)
	}
	if r.Decision != "rejected" {
		t.Errorf("Decision = %q, want %q", r.Decision, "rejected")
	}
}
