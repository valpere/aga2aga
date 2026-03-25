package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

// baseGenomeYAML returns a minimal valid agent.genome YAML with the given style block
// substituted in. Used by security surface tests that vary only the style field.
func baseGenomeYAML(styleBlock string) string {
	return `type: agent.genome
version: v1
agent_id: agent-style-test
kind: reviewer
status: proposed
identity:
  public_key: ed25519-pubkey-test
capabilities:
  skills:
    - code-review
tools:
  allowed:
    - read_file
model_policy:
  provider: anthropic
prompt_policy:
  profile: balanced
` + styleBlock + `routing_policy:
  accepts:
    - task.request
thresholds:
  confidence_min: 0.7
constraints:
  hard:
    - no_production_writes
mutation_policy:
  allowed:
    - prompt_policy
`
}

// TestPromptPolicy_StyleAttackerControlled is a security surface documentation test.
// PromptPolicy.Style is map[string]any — it accepts arbitrary YAML keys/values from
// the wire. Callers MUST NOT use Style values for auth, signing, or lifecycle decisions
// without explicit sanitization. This test locks the open-vocab contract.
// DO NOT DELETE — this test documents the CWE-20 surface from issue #35.
func TestPromptPolicy_StyleAttackerControlled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantNil bool
		checkFn func(t *testing.T, style map[string]any)
	}{
		{
			name:    "absent style field yields nil map",
			yaml:    baseGenomeYAML(""),
			wantNil: true,
		},
		{
			name: "flat attacker keys are all preserved",
			yaml: baseGenomeYAML(`  style:
    tone: formal
    injected_key: attacker-value
`),
			checkFn: func(t *testing.T, style map[string]any) {
				t.Helper()
				if style["tone"] != "formal" { // string scalar from gopkg.in/yaml.v3
					t.Errorf("Style[tone] = %v, want formal", style["tone"])
				}
				if _, ok := style["injected_key"]; !ok {
					t.Error("Style[injected_key] not preserved — open-vocab contract broken")
				}
			},
		},
		{
			name: "nested attacker payload is preserved at all depths",
			yaml: baseGenomeYAML(`  style:
    nested:
      deep: payload
`),
			checkFn: func(t *testing.T, style map[string]any) {
				t.Helper()
				nested, ok := style["nested"]
				if !ok {
					t.Fatal("Style[nested] not preserved — open-vocab contract broken")
				}
				inner, ok := nested.(map[string]any)
				if !ok {
					t.Fatalf("Style[nested] type = %T, want map[string]any", nested)
				}
				if inner["deep"] != "payload" {
					t.Errorf("Style[nested][deep] = %v, want payload", inner["deep"])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var doc document.Document
			if err := yaml.Unmarshal([]byte(tc.yaml), &doc); err != nil {
				t.Fatalf("yaml.Unmarshal error = %v", err)
			}

			genome, err := document.As[document.AgentGenome](&doc)
			if err != nil {
				t.Fatalf("As[AgentGenome]() error = %v", err)
			}

			if tc.wantNil {
				if genome.PromptPolicy.Style != nil {
					t.Errorf("Style = %v, want nil", genome.PromptPolicy.Style)
				}
				return
			}

			if genome.PromptPolicy.Style == nil {
				t.Fatal("PromptPolicy.Style is nil, want map with entries")
			}

			if tc.checkFn != nil {
				tc.checkFn(t, genome.PromptPolicy.Style)
			}
		})
	}
}

// baseGenomeWithRoutingYAML returns a minimal valid agent.genome YAML with the given
// routing_policy block substituted in. Used by security surface tests that vary routing.
func baseGenomeWithRoutingYAML(routingBlock string) string {
	return `type: agent.genome
version: v1
agent_id: agent-routing-test
kind: reviewer
status: proposed
identity:
  public_key: ed25519-pubkey-test
capabilities:
  skills:
    - code-review
tools:
  allowed:
    - read_file
model_policy:
  provider: anthropic
prompt_policy:
  profile: balanced
` + routingBlock + `thresholds:
  confidence_min: 0.7
constraints:
  hard:
    - no_production_writes
mutation_policy:
  allowed:
    - prompt_policy
`
}

// TestEscalationRule_RoutingPolicy_AttackerControlled is a security surface documentation test.
// EscalationRule.Condition and Target, and RoutingPolicy.DelegatesTo and Accepts are
// wire-supplied strings — the parse layer accepts any value verbatim. Dispatchers MUST NOT
// execute Condition strings or dispatch to Target/DelegatesTo without validating against a
// known agent registry. Accepts strings MUST be sanitized before routing decisions.
// This test locks the open-wire-string contract using the production document.Parse() path.
// DO NOT DELETE — documents CWE-20 / CWE-601 surface from issue #38.
func TestEscalationRule_RoutingPolicy_AttackerControlled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		checkFn func(t *testing.T, genome *document.AgentGenome)
	}{
		{
			name: "EscalationRule: attacker-injected condition and target preserved verbatim",
			yaml: baseGenomeWithRoutingYAML(`routing_policy:
  accepts:
    - task.request
  escalation_rules:
    - condition: "injected-condition-string"
      target: attacker-controlled-agent
`),
			checkFn: func(t *testing.T, genome *document.AgentGenome) {
				t.Helper()
				if len(genome.RoutingPolicy.EscalationRules) == 0 {
					t.Fatal("EscalationRules is empty — open-wire contract broken")
				}
				rule := genome.RoutingPolicy.EscalationRules[0]
				if rule.Condition != "injected-condition-string" {
					t.Errorf("Condition = %q, want injected-condition-string — open-wire contract: arbitrary condition strings must be preserved verbatim", rule.Condition)
				}
				if rule.Target != "attacker-controlled-agent" {
					t.Errorf("Target = %q, want attacker-controlled-agent — open-wire contract: arbitrary target strings must be preserved verbatim", rule.Target)
				}
			},
		},
		{
			// Condition carries a query-language-shaped string — documents that the parse
			// layer does NOT sanitize or reject expression-like condition strings. Dispatchers
			// MUST NOT interpret Condition as code or query language (CWE-20).
			name: "EscalationRule: query-language condition string preserved verbatim",
			yaml: baseGenomeWithRoutingYAML(`routing_policy:
  accepts:
    - task.request
  escalation_rules:
    - condition: "fitness_score < 0.5 AND safety_violations > 0"
      target: safety-auditor
`),
			checkFn: func(t *testing.T, genome *document.AgentGenome) {
				t.Helper()
				if len(genome.RoutingPolicy.EscalationRules) == 0 {
					t.Fatal("EscalationRules is empty — open-wire contract broken")
				}
				rule := genome.RoutingPolicy.EscalationRules[0]
				if rule.Condition != "fitness_score < 0.5 AND safety_violations > 0" {
					t.Errorf("Condition = %q — open-wire contract: query-language condition strings must be preserved verbatim without interpretation", rule.Condition)
				}
			},
		},
		{
			// No routing_policy key — verifies zero-value contract: RoutingPolicy fields are
			// nil slices, not empty allocated slices. Callers checking len(rules)==0 vs rules!=nil
			// must be aware that absent routing_policy produces nil, not [].
			name: "absent routing_policy produces zero-value RoutingPolicy",
			yaml: baseGenomeWithRoutingYAML(""),
			checkFn: func(t *testing.T, genome *document.AgentGenome) {
				t.Helper()
				if genome.RoutingPolicy.Accepts != nil {
					t.Errorf("Accepts = %v, want nil for absent routing_policy", genome.RoutingPolicy.Accepts)
				}
				if genome.RoutingPolicy.DelegatesTo != nil {
					t.Errorf("DelegatesTo = %v, want nil for absent routing_policy", genome.RoutingPolicy.DelegatesTo)
				}
				if genome.RoutingPolicy.EscalationRules != nil {
					t.Errorf("EscalationRules = %v, want nil for absent routing_policy", genome.RoutingPolicy.EscalationRules)
				}
			},
		},
		{
			name: "RoutingPolicy.DelegatesTo: attacker-injected agent IDs preserved verbatim",
			yaml: baseGenomeWithRoutingYAML(`routing_policy:
  accepts:
    - task.request
  delegates_to:
    - ../../etc/passwd
    - attacker-agent-id
`),
			checkFn: func(t *testing.T, genome *document.AgentGenome) {
				t.Helper()
				if len(genome.RoutingPolicy.DelegatesTo) < 2 {
					t.Fatalf("DelegatesTo len = %d, want 2 — open-wire contract broken", len(genome.RoutingPolicy.DelegatesTo))
				}
				if genome.RoutingPolicy.DelegatesTo[0] != "../../etc/passwd" {
					t.Errorf("DelegatesTo[0] = %q, want ../../etc/passwd — open-wire contract: arbitrary agent IDs must be preserved verbatim", genome.RoutingPolicy.DelegatesTo[0])
				}
				if genome.RoutingPolicy.DelegatesTo[1] != "attacker-agent-id" {
					t.Errorf("DelegatesTo[1] = %q, want attacker-agent-id — open-wire contract broken", genome.RoutingPolicy.DelegatesTo[1])
				}
			},
		},
		{
			name: "RoutingPolicy.Accepts: attacker-injected message types preserved verbatim",
			yaml: baseGenomeWithRoutingYAML(`routing_policy:
  accepts:
    - task.request
    - attacker.injected.type
    - ../../../etc/shadow
`),
			checkFn: func(t *testing.T, genome *document.AgentGenome) {
				t.Helper()
				if len(genome.RoutingPolicy.Accepts) < 3 {
					t.Fatalf("Accepts len = %d, want 3 — open-wire contract broken", len(genome.RoutingPolicy.Accepts))
				}
				if genome.RoutingPolicy.Accepts[1] != "attacker.injected.type" {
					t.Errorf("Accepts[1] = %q, want attacker.injected.type — open-wire contract: arbitrary message types must be preserved verbatim", genome.RoutingPolicy.Accepts[1])
				}
				if genome.RoutingPolicy.Accepts[2] != "../../../etc/shadow" {
					t.Errorf("Accepts[2] = %q, want ../../../etc/shadow — open-wire contract broken", genome.RoutingPolicy.Accepts[2])
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Use document.ParseAs — the production wire-parse path — so the test
			// exercises SplitFrontMatter, MaxDocumentBytes, and the full YAML chain.
			genome, err := document.ParseAs[document.AgentGenome]([]byte("---\n" + tc.yaml + "---\n"))
			if err != nil {
				t.Fatalf("ParseAs[AgentGenome]() error = %v", err)
			}

			tc.checkFn(t, genome)
		})
	}
}

func TestAgentGenome_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.genome
version: v1
agent_id: agent-abc
kind: reviewer
status: proposed
identity:
  pseudonym: reviewer-v1
  public_key: ed25519-pubkey-abc
capabilities:
  skills:
    - code-review
    - security-audit
  domains:
    - go
tools:
  allowed:
    - read_file
    - write_file
model_policy:
  provider: anthropic
  model: claude-3-5-sonnet
prompt_policy:
  profile: balanced
routing_policy:
  accepts:
    - task.request
thresholds:
  confidence_min: 0.7
constraints:
  hard:
    - no_production_writes
  soft:
    - prefer_readonly
mutation_policy:
  allowed:
    - prompt_policy
    - thresholds
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal Document error = %v", err)
	}

	genome, err := document.As[document.AgentGenome](&doc)
	if err != nil {
		t.Fatalf("As[AgentGenome]() error = %v", err)
	}

	if genome.AgentID != "agent-abc" {
		t.Errorf("AgentID = %q, want %q", genome.AgentID, "agent-abc")
	}

	if genome.Kind != "reviewer" {
		t.Errorf("Kind = %q, want %q", genome.Kind, "reviewer")
	}

	if genome.Identity.PublicKey != "ed25519-pubkey-abc" {
		t.Errorf("Identity.PublicKey = %q, want %q", genome.Identity.PublicKey, "ed25519-pubkey-abc")
	}

	if len(genome.Capabilities.Skills) == 0 || genome.Capabilities.Skills[0] != "code-review" {
		t.Errorf("Capabilities.Skills = %v, want [code-review ...]", genome.Capabilities.Skills)
	}

	if genome.ModelPolicy.Provider != "anthropic" {
		t.Errorf("ModelPolicy.Provider = %q, want %q", genome.ModelPolicy.Provider, "anthropic")
	}

	if len(genome.Constraints.Hard) == 0 || genome.Constraints.Hard[0] != "no_production_writes" {
		t.Errorf("Constraints.Hard = %v, want [no_production_writes]", genome.Constraints.Hard)
	}

	if len(genome.MutationPolicy.Allowed) == 0 {
		t.Errorf("MutationPolicy.Allowed is empty")
	}
}
