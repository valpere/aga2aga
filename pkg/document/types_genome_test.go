package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

// TestPromptPolicy_StyleAttackerControlled is a security surface documentation test.
// PromptPolicy.Style is map[string]any — it accepts arbitrary YAML keys/values from
// the wire. Callers MUST NOT use Style values for auth, signing, or lifecycle decisions
// without explicit sanitization. This test locks the open-vocab contract.
// DO NOT DELETE — this test documents the CWE-20 surface from issue #35.
func TestPromptPolicy_StyleAttackerControlled(t *testing.T) {
	t.Parallel()

	raw := `type: agent.genome
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
  style:
    tone: formal
    verbosity: high
    injected_key: attacker-value
    nested:
      deep: payload
routing_policy:
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

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	genome, err := document.As[document.AgentGenome](&doc)
	if err != nil {
		t.Fatalf("As[AgentGenome]() error = %v", err)
	}

	// Style round-trips arbitrary keys — open vocab by design (spec §4.3).
	// The entire map is attacker-controlled; callers must sanitize before use.
	if genome.PromptPolicy.Style == nil {
		t.Fatal("PromptPolicy.Style is nil, want map with entries")
	}

	if genome.PromptPolicy.Style["tone"] != "formal" {
		t.Errorf("Style[tone] = %v, want formal", genome.PromptPolicy.Style["tone"])
	}

	// Attacker-injected key is preserved — this is by design (open vocab).
	// Callers bear responsibility for sanitization.
	if _, ok := genome.PromptPolicy.Style["injected_key"]; !ok {
		t.Error("Style[injected_key] not preserved — open-vocab contract broken")
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
