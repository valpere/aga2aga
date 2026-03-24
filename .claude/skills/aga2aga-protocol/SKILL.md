---
name: aga2aga-protocol
description: "Expert reference for the aga2aga Skills Document protocol. Use when working with message types, YAML headers, document parsing/validation/building, lifecycle transitions, or agent genome fields."
metadata:
  version: "1.0.0"
  domain: protocol
  triggers: skills document, yaml header, message type, parse document, validate document, build document, lifecycle, agent genome, spawn proposal, negotiation, task request
  scope: implementation, design, review
---

# aga2aga Protocol — Developer Reference

**Announce at start:** "I'm using the aga2aga-protocol skill."

---

## 1. Document Structure

Every protocol message is a Markdown document with a YAML control header:

```markdown
---
type: task.request
version: v1
id: <uuid>
from: <sender-agent-id>
to: <recipient-agent-id>
created_at: 2026-03-23T10:00:00Z
---

## Task Title

Human-readable content here. Passed to the agent as-is.
```

**Parsing rules:**
- Front matter is delimited by `---` on its own line
- Everything after the second `---` is the Markdown body
- YAML header is machine-parsed; body is passed to the agent verbatim
- `to` field may be a single string or a YAML list (`StringOrList` type)

---

## 2. Required Envelope Fields

Every document MUST have these fields:

| Field | Type | Notes |
|-------|------|-------|
| `type` | `protocol.MessageType` | One of the canonical types below |
| `version` | string | Always `"v1"` — use `protocol.ProtocolVersion` constant |
| `id` | string | UUIDv4; unique per message |
| `from` | string | Sender agent pseudonym |
| `to` | StringOrList | Recipient(s); may be `"*"` for broadcast |
| `created_at` | RFC3339 | Timestamp of creation |

Optional envelope fields: `in_reply_to`, `thread_id`, `exec_id`, `ttl`, `status`, `signature`.

**DO NOT hardcode `"v1"` as a string literal** — always use `protocol.ProtocolVersion`.

---

## 3. Canonical Message Types

### Agent Evolution (11 types)

| Constant | Wire value |
|----------|-----------|
| `MessageTypeAgentGenome` | `agent.genome` |
| `MessageTypeSpawnProposal` | `agent.spawn.proposal` |
| `MessageTypeSpawnApproval` | `agent.spawn.approval` |
| `MessageTypeSpawnRejection` | `agent.spawn.rejection` |
| `MessageTypeEvaluationRequest` | `agent.evaluation.request` |
| `MessageTypeEvaluationResult` | `agent.evaluation.result` |
| `MessageTypePromotion` | `agent.promotion` |
| `MessageTypeRollback` | `agent.rollback` |
| `MessageTypeRetirement` | `agent.retirement` |
| `MessageTypeQuarantine` | `agent.quarantine` |
| `MessageTypeRecombineProposal` | `agent.recombine.proposal` |

### Task Types (5 types)

| Constant | Wire value |
|----------|-----------|
| `MessageTypeTaskRequest` | `task.request` |
| `MessageTypeTaskResult` | `task.result` |
| `MessageTypeTaskFail` | `task.fail` |
| `MessageTypeTaskProgress` | `task.progress` |
| `MessageTypeAgentMessage` | `agent.message` |

### Negotiation Types (8 types)

| Constant | Wire value |
|----------|-----------|
| `MessageTypeNegotiationPropose` | `negotiation.propose` |
| `MessageTypeNegotiationAccept` | `negotiation.accept` |
| `MessageTypeNegotiationReject` | `negotiation.reject` |
| `MessageTypeNegotiationCounter` | `negotiation.counter` |
| `MessageTypeNegotiationClarify` | `negotiation.clarify` |
| `MessageTypeNegotiationDelegate` | `negotiation.delegate` |
| `MessageTypeNegotiationCommit` | `negotiation.commit` |
| `MessageTypeNegotiationAbort` | `negotiation.abort` |

---

## 4. Agent Lifecycle State Machine

### States (10 total)

```
proposed → approved_for_sandbox → sandbox → candidate → active
                ↓                    ↓          ↓          ↓
             rejected          failed_sandbox  ↓       inactive → retired
                                           quarantined → retired
                                           rolled_back  (terminal)
```

### Valid Transitions

| From | To |
|------|----|
| `proposed` | `approved_for_sandbox`, `rejected` |
| `approved_for_sandbox` | `sandbox` |
| `sandbox` | `candidate`, `failed_sandbox`, `quarantined` |
| `candidate` | `active`, `sandbox`, `quarantined` |
| `active` | `candidate`, `inactive`, `retired`, `quarantined` |
| `inactive` | `active`, `retired` |
| `quarantined` | `retired` |
| `rejected` | *(terminal)* |
| `failed_sandbox` | *(terminal)* |
| `rolled_back` | *(terminal)* |
| `retired` | *(terminal)* |

**Always use `lifecycle.ValidTransition(from, to)` — never implement this logic inline.**

### DO_NOT_TOUCH — Lifecycle

```
// DO_NOT_TOUCH: lifecycle transition table — spec §16 — modifying breaks wire compatibility
var validTransitions = map[LifecycleState][]LifecycleState{ ... }
```

The `ValidTransition(from, to LifecycleState) bool` function signature is frozen.

---

## 5. Agent Genome Fields

### Immutable Fields — DO NOT mutate in descendants

```
// DO_NOT_TOUCH: spec §5.4
id           — original agent identifier
lineage      — parent chain
genome_version — schema version of the genome format
created_at   — original creation timestamp
kind         — fundamental agent category
```

Descendants receive a **new** `id`, referencing the parent via `lineage`.

### Key Mutable Fields

`status`, `name`, `description`, `capabilities`, `constraints.soft`, `fitness_history`

### Forbidden Mutation Targets (spec §5.6)

`identity` (cryptographic identity), `constraints.hard` (hard safety limits)

---

## 6. Builder API

```go
doc, err := document.NewBuilder(protocol.MessageTypeTaskRequest).
    From("orchestrator-1").
    To("reviewer-7b").
    Field("exec_id", "exec-abc123").
    Body("## Review this PR\n\nPlease review PR #42.").
    Build()
```

- `NewBuilder(msgType)` — sets `type` and `version: v1`; generates `id` and `created_at` if not set
- `From(id string)` — sets `from`
- `To(ids ...string)` — sets `to` (single or multiple)
- `Field(key string, value any)` — sets any extra envelope field or typed message field
- `Body(markdown string)` — sets the Markdown body
- `Build() (*Document, error)` — runs full 3-layer validation; returns error if invalid

**`Build()` never returns an invalid document.** If validation fails, fix inputs — do not skip validation.

---

## 7. Validator Layers

Validation runs three independent layers in sequence:

| Layer | What it checks | Error field |
|-------|---------------|-------------|
| **Structural** | Required fields per `protocol.Registry` | `Layer: "structural"` |
| **JSON Schema** | Full document against schema 2020-12 | `Layer: "schema"` |
| **Semantic** | Lifecycle transitions, no self-promotion, immutable field mutations | `Layer: "semantic"` |

```go
v, err := document.NewValidator(schemaBytes)
errs := v.Validate(doc)           // all layers
errs = v.ValidateStructural(doc)  // layer 1 only
errs = v.ValidateSchema(doc)      // layer 2 only
errs = v.ValidateSemantic(doc)    // layer 3 only
```

`ValidationError` fields: `Layer string`, `Field string`, `Message string`.

---

## 8. Reference Routing Table

| Operation | File |
|-----------|------|
| Parse a document | `pkg/document/parser.go` — `Parse()`, `SplitFrontMatter()`, `ParseAs[T]()` |
| Serialize a document | `pkg/document/parser.go` — `Serialize()` |
| Validate a document | `pkg/document/validator.go` — `Validator.Validate()` |
| Check lifecycle transition | `pkg/document/lifecycle.go` — `ValidTransition()` |
| Build a document | `pkg/document/builder.go` — `NewBuilder()` chain |
| All message type constants | `pkg/protocol/types.go` |
| Required fields per type | `pkg/protocol/registry.go` — `Registry` map |
| JSON Schema 2020-12 | `pkg/document/schema.yaml` |
| Formal spec (authoritative) | `context/preparation/17-single_formal_spec_document.md` |
| JSON Schema source | `context/preparation/18-...` (design doc 18) |

---

## 9. Key Invariants

- `Parse(Serialize(doc))` produces a document equal to the original (round-trip invariant)
- Every document returned by `Build()` passes `Validate()` with zero errors
- `ValidTransition(from, to)` is the single source of truth for lifecycle legality — never duplicate
- `ProtocolVersion = "v1"` — never use the string literal directly
- `pkg/document` MUST NOT import `internal/`, `cmd/`, or any infrastructure package
