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
