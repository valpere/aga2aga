package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

// TestLifecycle_WireFieldsAreAttackerControlled is a security surface documentation test.
// FromStatus and ToStatus on lifecycle types are self-reported wire strings — the parse
// layer accepts any string value verbatim. Executors MUST derive authoritative state from
// a trusted state-store and call document.ValidTransition(), never trusting wire values.
// Reason is an opaque logging label — MUST NOT influence transition logic.
// This test locks the open-wire-string contract using the production document.Parse() path.
// DO NOT DELETE — documents CWE-20 surface from issue #39.
func TestLifecycle_WireFieldsAreAttackerControlled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		checkFn func(t *testing.T, doc *document.Document)
	}{
		{
			name: "Promotion: attacker-injected from_status/to_status preserved verbatim",
			raw: `type: agent.promotion
version: v1
id: msg-1
from: attacker
target_agent: agent-1
from_status: attacker-injected-status
to_status: attacker-injected-to
`,
			checkFn: func(t *testing.T, doc *document.Document) {
				t.Helper()
				p, err := document.As[document.Promotion](doc)
				if err != nil {
					t.Fatalf("As[Promotion]() error = %v", err)
				}
				if p.FromStatus != "attacker-injected-status" {
					t.Errorf("FromStatus = %q, want attacker-injected-status — open-wire contract broken", p.FromStatus)
				}
				if p.ToStatus != "attacker-injected-to" {
					t.Errorf("ToStatus = %q, want attacker-injected-to — open-wire contract broken", p.ToStatus)
				}
			},
		},
		{
			name: "Rollback: attacker-injected from_status/to_status preserved verbatim",
			raw: `type: agent.rollback
version: v1
id: msg-2
from: attacker
target_agent: agent-1
from_status: injected-from
to_status: injected-to
reason: "DROP TABLE agents"
`,
			checkFn: func(t *testing.T, doc *document.Document) {
				t.Helper()
				r, err := document.As[document.Rollback](doc)
				if err != nil {
					t.Fatalf("As[Rollback]() error = %v", err)
				}
				if r.FromStatus != "injected-from" {
					t.Errorf("FromStatus = %q, want injected-from — open-wire contract broken", r.FromStatus)
				}
				if r.ToStatus != "injected-to" {
					t.Errorf("ToStatus = %q, want injected-to — open-wire contract broken", r.ToStatus)
				}
				if r.Reason != "DROP TABLE agents" {
					t.Errorf("Reason = %q, want DROP TABLE agents — open-wire contract: arbitrary reason strings must be preserved verbatim", r.Reason)
				}
			},
		},
		{
			name: "Quarantine: attacker-injected from_status preserved; reason is opaque label",
			raw: `type: agent.quarantine
version: v1
id: msg-3
from: attacker
target_agent: agent-1
reason: <script>alert(1)</script>
from_status: injected-state
`,
			checkFn: func(t *testing.T, doc *document.Document) {
				t.Helper()
				q, err := document.As[document.Quarantine](doc)
				if err != nil {
					t.Fatalf("As[Quarantine]() error = %v", err)
				}
				if q.FromStatus != "injected-state" {
					t.Errorf("FromStatus = %q, want injected-state — open-wire contract broken", q.FromStatus)
				}
				if q.Reason != "<script>alert(1)</script>" {
					t.Errorf("Reason = %q, want <script>alert(1)</script> — open-wire contract: arbitrary reason strings must be preserved verbatim", q.Reason)
				}
			},
		},
		{
			name: "Retirement: attacker-injected from_status preserved; reason is opaque label",
			raw: `type: agent.retirement
version: v1
id: msg-4
from: attacker
target_agent: agent-1
reason: $(rm -rf /)
from_status: injected-state
`,
			checkFn: func(t *testing.T, doc *document.Document) {
				t.Helper()
				r, err := document.As[document.Retirement](doc)
				if err != nil {
					t.Fatalf("As[Retirement]() error = %v", err)
				}
				if r.FromStatus != "injected-state" {
					t.Errorf("FromStatus = %q, want injected-state — open-wire contract broken", r.FromStatus)
				}
				if r.Reason != "$(rm -rf /)" {
					t.Errorf("Reason = %q, want $(rm -rf /) — open-wire contract: arbitrary reason strings must be preserved verbatim", r.Reason)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Use document.Parse() — the production wire-parse path — so the test
			// exercises the same code path as real incoming documents and includes
			// the MaxDocumentBytes guard.
			doc, err := document.Parse([]byte("---\n" + tc.raw + "---\n"))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			tc.checkFn(t, doc)
		})
	}
}

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

func TestQuarantine_FromStatus(t *testing.T) {
	t.Parallel()

	raw := `type: agent.quarantine
version: v1
id: msg-quarantine-2
from: safety-auditor
target_agent: agent-rogue-7
reason: safety violation
from_status: active
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	quarantine, err := document.As[document.Quarantine](&doc)
	if err != nil {
		t.Fatalf("As[Quarantine]() error = %v", err)
	}

	if quarantine.FromStatus != "active" {
		t.Errorf("FromStatus = %q, want active", quarantine.FromStatus)
	}
}

func TestQuarantine_FromStatus_Optional(t *testing.T) {
	t.Parallel()

	raw := `type: agent.quarantine
version: v1
id: msg-quarantine-3
from: safety-auditor
target_agent: agent-rogue-7
reason: safety violation
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	quarantine, err := document.As[document.Quarantine](&doc)
	if err != nil {
		t.Fatalf("As[Quarantine]() error = %v", err)
	}

	if quarantine.FromStatus != "" {
		t.Errorf("FromStatus = %q, want empty (omitempty)", quarantine.FromStatus)
	}
}

func TestRetirement_FromStatus(t *testing.T) {
	t.Parallel()

	raw := `type: agent.retirement
version: v1
id: msg-retire-2
from: population-manager
target_agent: agent-old-99
reason: superseded
from_status: active
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	retirement, err := document.As[document.Retirement](&doc)
	if err != nil {
		t.Fatalf("As[Retirement]() error = %v", err)
	}

	if retirement.FromStatus != "active" {
		t.Errorf("FromStatus = %q, want active", retirement.FromStatus)
	}
}

func TestRetirement_FromStatus_Optional(t *testing.T) {
	t.Parallel()

	raw := `type: agent.retirement
version: v1
id: msg-retire-3
from: population-manager
target_agent: agent-old-99
reason: superseded
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	retirement, err := document.As[document.Retirement](&doc)
	if err != nil {
		t.Fatalf("As[Retirement]() error = %v", err)
	}

	if retirement.FromStatus != "" {
		t.Errorf("FromStatus = %q, want empty (omitempty)", retirement.FromStatus)
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
