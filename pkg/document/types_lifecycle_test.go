package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestPromotion_UnmarshalYAML(t *testing.T) {
	t.Parallel()

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

	promotion, err := document.As[document.Promotion](&doc)
	if err != nil {
		t.Fatalf("As[Promotion]() error = %v", err)
	}

	if promotion.TargetAgent != "agent-candidate-42" {
		t.Errorf("TargetAgent = %q, want agent-candidate-42", promotion.TargetAgent)
	}

	if promotion.ToStatus != "candidate" {
		t.Errorf("ToStatus = %q, want candidate", promotion.ToStatus)
	}
}

func TestQuarantine_UnmarshalYAML(t *testing.T) {
	t.Parallel()

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

	quarantine, err := document.As[document.Quarantine](&doc)
	if err != nil {
		t.Fatalf("As[Quarantine]() error = %v", err)
	}

	if quarantine.TargetAgent != "agent-rogue-7" {
		t.Errorf("TargetAgent = %q, want agent-rogue-7", quarantine.TargetAgent)
	}

	if quarantine.Reason != "safety violation in production task" {
		t.Errorf("Reason = %q", quarantine.Reason)
	}
}

func TestRecombineProposal_UnmarshalYAML(t *testing.T) {
	t.Parallel()

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

	recombine, err := document.As[document.RecombineProposal](&doc)
	if err != nil {
		t.Fatalf("As[RecombineProposal]() error = %v", err)
	}

	if len(recombine.ParentIDs) != 2 {
		t.Errorf("ParentIDs len = %d, want 2", len(recombine.ParentIDs))
	}

	if recombine.Goal == "" {
		t.Errorf("Goal is empty")
	}
}

func TestRollback_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.rollback
version: v1
id: msg-rollback-1
from: population-manager
target_agent: agent-candidate-42
from_status: candidate
to_status: sandbox
reason: quality score below threshold
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	rollback, err := document.As[document.Rollback](&doc)
	if err != nil {
		t.Fatalf("As[Rollback]() error = %v", err)
	}

	if rollback.TargetAgent != "agent-candidate-42" {
		t.Errorf("TargetAgent = %q, want agent-candidate-42", rollback.TargetAgent)
	}

	if rollback.FromStatus != "candidate" {
		t.Errorf("FromStatus = %q, want candidate", rollback.FromStatus)
	}

	if rollback.ToStatus != "sandbox" {
		t.Errorf("ToStatus = %q, want sandbox", rollback.ToStatus)
	}
}

func TestRetirement_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.retirement
version: v1
id: msg-retire-1
from: population-manager
target_agent: agent-old-99
reason: superseded by newer generation
retirement_mode: graceful
replace_with:
  - agent-new-100
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	retirement, err := document.As[document.Retirement](&doc)
	if err != nil {
		t.Fatalf("As[Retirement]() error = %v", err)
	}

	if retirement.TargetAgent != "agent-old-99" {
		t.Errorf("TargetAgent = %q, want agent-old-99", retirement.TargetAgent)
	}

	if retirement.RetirementMode != "graceful" {
		t.Errorf("RetirementMode = %q, want graceful", retirement.RetirementMode)
	}

	if len(retirement.ReplaceWith) != 1 || retirement.ReplaceWith[0] != "agent-new-100" {
		t.Errorf("ReplaceWith = %v, want [agent-new-100]", retirement.ReplaceWith)
	}
}
