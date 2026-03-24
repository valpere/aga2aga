package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestPromotion_UnmarshalYAML(t *testing.T) {
	raw := `type: agent.promotion
version: v1
id: msg-promote-1
from: population-manager
target_agent: agent-candidate-42
from_status: sandbox
to_status: candidate
`
	var doc document.Document
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}
	p, err := document.As[document.Promotion](&doc)
	if err != nil {
		t.Fatalf("As[Promotion]() error = %v", err)
	}
	if p.TargetAgent != "agent-candidate-42" {
		t.Errorf("TargetAgent = %q, want agent-candidate-42", p.TargetAgent)
	}
	if p.ToStatus != "candidate" {
		t.Errorf("ToStatus = %q, want candidate", p.ToStatus)
	}
}

func TestQuarantine_UnmarshalYAML(t *testing.T) {
	raw := `type: agent.quarantine
version: v1
id: msg-quarantine-1
from: safety-auditor
target_agent: agent-rogue-7
reason: safety violation in production task
`
	var doc document.Document
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}
	q, err := document.As[document.Quarantine](&doc)
	if err != nil {
		t.Fatalf("As[Quarantine]() error = %v", err)
	}
	if q.TargetAgent != "agent-rogue-7" {
		t.Errorf("TargetAgent = %q, want agent-rogue-7", q.TargetAgent)
	}
	if q.Reason != "safety violation in production task" {
		t.Errorf("Reason = %q", q.Reason)
	}
}

func TestRecombineProposal_UnmarshalYAML(t *testing.T) {
	raw := `type: agent.recombine.proposal
version: v1
id: msg-recombine-1
from: meta-evolver
candidate_id: agent-combined-1
parent_ids:
  - agent-parent-a
  - agent-parent-b
goal: combine security expertise from parent-a with speed of parent-b
status: proposed
`
	var doc document.Document
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}
	r, err := document.As[document.RecombineProposal](&doc)
	if err != nil {
		t.Fatalf("As[RecombineProposal]() error = %v", err)
	}
	if len(r.ParentIDs) != 2 {
		t.Errorf("ParentIDs len = %d, want 2", len(r.ParentIDs))
	}
	if r.Goal == "" {
		t.Errorf("Goal is empty")
	}
}
