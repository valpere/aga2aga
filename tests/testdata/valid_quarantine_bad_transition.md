---
type: agent.quarantine
version: v1
id: msg-quarantine-semantic-test
from: safety-auditor
target_agent: agent-1
reason: semantic-only validation test fixture
from_status: proposed
---

## Quarantine Notice

This document passes structural and schema validation but fails semantic validation
because proposed → quarantined is not a permitted lifecycle transition.

Use this fixture to test --strict mode: exits 0 without --strict (semantic errors
are warnings), non-zero with --strict (semantic errors are fatal).
