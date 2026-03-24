---
type: agent.genome
version: v1
agent_id: agent-fixture-1
kind: reviewer
status: proposed
identity:
  public_key: ed25519-pubkey-fixture
capabilities:
  skills:
    - code-review
    - security-audit
tools:
  allowed:
    - read_file
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
mutation_policy:
  allowed:
    - prompt_policy
    - thresholds
---

## Agent Blueprint

This agent performs code review and security audit tasks.
