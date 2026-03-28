# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**aga2aga** — a Go MCP Gateway that bridges external AI agents (Claude Code, Codex CLI, Gemini CLI) to a Redis Streams-based orchestration system. Module: `github.com/valpere/aga2aga`.

The core insight: you can't embed an SDK into closed agents, so you expose an MCP interface they already understand and translate between MCP ↔ Redis Streams internally.

## Common Commands

```bash
go mod tidy          # sync dependencies
go build ./...       # build all packages
go test ./...        # run all tests
go test -run TestFoo # run a single test
go vet ./...         # static analysis
gofmt -w .           # format code
```

## Tech Stack

- **Transport:** Redis Streams (Phase 1–2), Gossip P2P (Phase 5)
- **Protocol:** Markdown + YAML envelope documents
- **Identity/Crypto:** Ed25519 signatures
- **Schema validation:** JSON Schema 2020-12
- **CI:** GitHub Actions — golangci-lint **v2.11.4** (local machine has v1; do not use local lint to validate config); step order: go mod tidy → Build → Test → Upload coverage (7-day artifact) → Vet → Lint; no secrets under `pull_request` trigger
- **Container:** Docker (Phase 2+)

## Architecture

### Data Flow

```
Agents (Claude Code / Codex / Gemini)
        ↓ MCP (stdio or HTTP)
   MCP Gateway (Go)  ←→  pending map [taskID → msgID]
        ↓
   Redis Streams          ← Phase 1-2
        ↓
   Orchestrator
```

Transport is pluggable: Redis → Gossip P2P → fully offline. Each layer is optional.

### MCP Tools Exposed

| Tool              | Redis operation                                       |
| ----------------- | ----------------------------------------------------- |
| `get_task`        | `XREADGROUP` from `agent.tasks.<agent>`               |
| `complete_task`   | `XADD` to `agent.events.completed` + `XACK`           |
| `fail_task`       | `XADD` to `agent.events.failed`                       |
| `heartbeat`       | health check only                                     |
| `send_message`    | `XADD` to `agent.messages.<recipient>`                |
| `receive_message` | `XREADGROUP` from `agent.messages.<agent>` + `XACK`   |

### Envelope Document Protocol

All inter-agent messages are **Markdown documents with a YAML control header**:

```markdown
---
id: <unique-id>
type: task.request | task.result | task.fail | task.progress | agent.message
version: v1
from: <sender-id>
to: <recipient-id>
exec_id: <workflow-id>
step: <step-name>
---

## Task / Result / Body

Human-readable content here.
```

The YAML header is machine-parsed; the Markdown body is passed to the agent as-is. The gateway converts Redis payloads into these documents and routes agent responses back to Redis.

### Agent Genome & Lifecycle

Agents are described as `agent.genome` documents (YAML+Markdown) with lifecycle states:

```
proposed → approved_for_sandbox → sandbox → candidate → active
                                                  ↓
                               quarantined / rolled_back / retired
```

Key governance roles: `meta-evolver`, `safety-auditor`, `benchmark-curator`, `evaluator`, `population-manager`.

Fitness is a weighted score (quality 35%, safety 15%, reliability 20%, latency 10%, cost 10%, collaboration 10%). Safety is a hard gate — zero violations required for promotion.

### Known Constraints

- Solo developer — bandwidth is the bottleneck; keep scope tight
- Closed agents (Claude Code, Codex) are session-based — the gateway must proxy state for them between calls (e.g. `taskID → msgID` mapping)
- ZK crypto layers are research-grade and not near-term

### Package Structure

```
cmd/enveloper/      CLI tool (aga2aga-enveloper validate/create/inspect)  ← DONE (issue #21)
cmd/admin/        Web admin UI binary (aga2aga-admin)         ← DONE (issue #86)
cmd/gateway/      MCP Gateway binary                          ← DONE (issue #92)
pkg/document/     envelope document parser, validator, builder  ← DONE (Phase 1)
pkg/protocol/     Message types and registry                  ← DONE (issue #15)
pkg/transport/    Transport abstraction (Redis, Gossip)       (stub)
pkg/identity/     Ed25519 identity and trust                  (stub)
pkg/negotiation/  Negotiation protocol engine                 (stub)
internal/admin/   Web admin HTTP handlers, middleware, SQLite store ← DONE (issue #86)
internal/gateway/ MCP Gateway implementation                       ← DONE (#90, #91)
pkg/admin/        Admin domain types, Store interface, policy eval ← DONE (issue #86)
docs/             All project documentation
```

#### Implemented: pkg/protocol

- 24 `MessageType` constants across 3 groups (agent evolution, task, negotiation)
- `Registry` map — `TypeMeta` per type (required fields + schema ref)
- `BaseEnvelopeFields`, `ProtocolVersion = "v1"` (DO_NOT_TOUCH)

#### Implemented: pkg/document

- `StringOrList` — scalar/sequence YAML type for `to:` field
- `Envelope` — 13 wire fields; `From` is unverified until Phase 3
- `Document` — `Envelope` + `Extra map[string]any` + `Body` + `Raw`
- `As[T]` — YAML round-trip to typed struct; strips all Envelope keys from `Extra` first (injection defence)
- Typed structs for all 24 message types across 5 files (`types_task`, `types_genome`, `types_lifecycle`, `types_spawn`, `types_evaluation`)
- Parser: `Parse`, `Serialize`, `SplitFrontMatter`; `MaxDocumentBytes = 64 KiB` hard limit
- Lifecycle: `ValidTransition`, `AllowedTransitions`, 11 `LifecycleState` constants (DO_NOT_TOUCH)
- Validator: `ValidateStructural` / `ValidateSchema` / `ValidateSemantic` / `Validate` (3-layer composite); `DefaultValidator()`
  - Semantic layer enforces `ValidTransition` for promotion, rollback, quarantine, and retirement
  - Self-action governance check (`from == target_agent`) enforced for all four types; fires independently of `from_status` presence
  - `--strict` mode: semantic errors are warnings by default, fatal with `--strict`
  - `DefaultValidator()` is a `sync.Once` singleton; all per-type `$def` schemas are pre-compiled eagerly in `NewValidator` — safe for concurrent use (no data race)
- Builder: `NewBuilder` + fluent setters (`ID`, `From`, `To`, `ExecID`, `TTL`, `Status`, `InReplyTo`, `ThreadID`, `Body`, `Field`); `Build()` runs full validation; sticky-error guard rejects reserved envelope keys in `Field()`
  - Convenience: `NewGenomeBuilder`, `NewSpawnProposalBuilder`, `NewTaskRequestBuilder`

#### Implemented: pkg/transport/redis (#89)

- `RedisTransport` — `Publish`, `Subscribe`, `Ack`, `Close`; context on all I/O; wraps go-redis v9
- `PendingMap` — taskID → msgID mapping with configurable TTL-based cleanup goroutine; concurrent-safe

#### Implemented: internal/gateway (#90, #91) and cmd/gateway (#92)

- `PolicyEnforcer` interface — `Allowed(ctx, source, target string) (bool, error)`
- `EmbeddedEnforcer` — in-process via `admin.PolicyStore.ListPolicies` + `admin.Evaluate`; default deny
- `HTTPEnforcer` — remote `GET /api/v1/evaluate?source=X&target=Y` with Bearer token
  - `NewHTTPEnforcer(baseURL, token string) (*HTTPEnforcer, error)` — validates scheme (http/https) + non-empty host (CWE-918)
  - `io.LimitReader(4KiB)` on response body before JSON decode (CWE-400)
  - Bearer token never in error messages (CWE-532)
  - `url.QueryEscape` on source/target params
- `Config` struct + `DefaultConfig()` — `AgentID`, `TaskReadTimeout` (5s), `PendingTTL` (5m)
- `Gateway` struct — wires MCP server, Transport, PendingMap, PolicyEnforcer
- `New(t, e, cfg)` — creates Gateway with 6 MCP tools registered
- `Run(ctx, mcpTransport)` — starts PendingMap cleanup, serves MCP over given transport
- 6 tool handlers:
  - `get_task`: validates agent ID → policy check → subscribe → wait with timeout → store in PendingMap → return
  - `complete_task`: validates → policy → body size cap → LoadAndDelete → build task.result → Publish → Ack
  - `fail_task`: same pattern, publishes to `agent.events.failed`
  - `heartbeat`: no-op, returns `{status: "ok"}`
  - `send_message`: validates sender + recipient IDs → policy(sender→recipient) → body size cap → build agent.message → Publish to `agent.messages.<recipient>`
  - `receive_message`: validates → policy(agent→orchestrator) → subscribe `agent.messages.<agent>` → wait with timeout → Ack immediately → return `{from, body}`
- Security: agent ID regex `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}[a-zA-Z0-9]$` (CWE-20/CWE-74); `taskID = delivery.MsgID` (transport-layer ID, not attacker-controlled `Doc.ID`); body capped at `MaxDocumentBytes` (CWE-400); `SECURITY(Phase 3):` comments at all `Allowed()` call sites (self-reported agent ID, CWE-287)
- `cmd/gateway/main.go` (#92): 10 CLI flags; ADMIN_API_KEY env var preferred over flag (CWE-214); LIFO defer — `rdb` deferred first (runs last), `trans` deferred second (runs first); stdio + HTTP transports; `WriteTimeout:0` on http.Server for SSE long-lived streams; graceful shutdown with 10s context; `mustEnforcer` with `filepath.EvalSymlinks` on `--admin-db` (CWE-22/61)
- `PendingMap.StartCleanup` idempotent via `sync.Once` — safe to call from both `Gateway.Run()` (stdio) and `Gateway.StartCleanup()` (HTTP path) without spawning duplicate goroutines

#### Implemented: pkg/transport, pkg/identity, pkg/negotiation (stubs)

- `pkg/transport` — `Transport` interface: `Publish`, `Subscribe`, `Ack` (with pending-map source contract), `Close`; context on all I/O methods; no external imports
- `pkg/identity` — `Identity` struct (`Pseudonym`, `PublicKey ed25519.PublicKey`); `Signer` interface: `Sign`, `Verify(data, sig []byte) (bool, error)` (error-aware for config faults, CWE-252)
- `pkg/negotiation` — `NegotiationState` type; 8 constants derived from `pkg/protocol` (no drift); `NegotiationTransition` stub (always false, NOT for gate use before Phase 4)

#### Implemented: cmd/enveloper

- `aga2aga-enveloper validate <file>` — 3-layer validation; `--strict` flag
- `aga2aga-enveloper create <type>` — build any registered message type via `--id/--from/--to/--exec-id/--field/--out`
- `aga2aga-enveloper inspect <file>` — print envelope fields; `--format text|json`; JSON output nests `Extra` under `"extra"` key
- `readAndParseFile` helper (`helpers.go`) — shared open/size-check/parse; `ErrDocumentTooLarge` sentinel for `errors.Is` testing; `filepath.EvalSymlinks` guard (CWE-22/61); path is CLI-only (SECURITY godoc)

#### Security invariants (pkg/document)

- `Envelope.From` is self-reported; authorization MUST NOT rely on it until Phase 3 (Ed25519)
- `Document.Extra` is attacker-controlled; never use directly for auth, signing, or lifecycle decisions
- `As[T]` strips the 13 Envelope yaml keys via `envelopeKeys` map before marshal — attacker cannot shadow Envelope fields in typed structs
- `Promotion.FromStatus` / `Rollback.FromStatus` / `Quarantine.FromStatus` / `Retirement.FromStatus` are self-reported wire strings (CWE-20); executors MUST derive authoritative state from state-store and call `document.ValidTransition()` — never trust wire values. Quarantine/Retirement `from_status` is optional (`omitempty`); Promotion/Rollback `from_status` is required on wire
- `Promotion.Reason` / `Rollback.Reason` / `Quarantine.Reason` / `Retirement.Reason` are opaque logging labels — MUST NOT influence transition logic
- `RecombineProposal.CandidateID` and `ParentIDs` are self-reported; executors MUST verify against state-store before genome creation
- Semantic validator calls `ValidTransition` for quarantine/retirement when `from_status` is present; schema guards the enum
- `SpawnProposal.GenomePatch` is typed (`*GenomePatch`) — DO_NOT_TOUCH fields are structurally absent; patch-apply MUST only append to `SoftConstraints`, never replace
- `PromptPolicy.Style` is `map[string]any` — attacker-controlled (open vocab per spec §4.3); callers MUST sanitise before auth/signing/lifecycle use (annotated in `types_genome.go`)
- `EscalationRule.Condition` is an opaque label — MUST NOT be executed or interpreted as code/query language (CWE-20); `EscalationRule.Target` is self-reported — MUST validate in agent registry before dispatch (CWE-601)
- `RoutingPolicy.Accepts`, `DelegatesTo`, `EscalationRules` are all wire-supplied and attacker-controlled; dispatchers MUST sanitize Accepts against protocol registry and validate DelegatesTo IDs in state-store before any routing/delegation decision (annotated in `types_genome.go`, issue #38)

## Skills and Plugins

The following plugins are installed for this project:

- **`obra/superpowers`** — core workflow skills: brainstorming, writing-plans, TDD, debugging, code-reviewer, subagent execution
- **`anthropics/mcp-server-dev`** — MCP server development skill (use in Phase 2)

When using these skills, invoke them via the `Skill` tool — do not read skill files directly.

### Skill Authoring (when creating project-specific skills)

Descriptions must follow: `[Brief capability]. Use when [trigger conditions].` — max 1024 chars. Never put process steps in the description; those go in the skill body.

Frontmatter template:
```yaml
---
name: skill-name-with-hyphens
description: "Brief capability. Use when trigger conditions."
metadata:
  version: "1.0.0"
  domain: protocol | transport | identity | negotiation | gateway
  triggers: keyword1, keyword2
  scope: implementation | review | design | testing
---
```

## Behaviour

### Skill Activation

If there is even a **1% chance** a skill applies to the current task, invoke it — this is not optional and cannot be rationalized away. Red flags to reject:

- "This is just a simple question"
- "I remember what that skill says"
- "This seems like overkill"
- "I need context first"

### Verification Discipline

No completion claims without fresh evidence. The sequence is always: identify the verification command → run it → examine output → then state the claim. Forbidden language: "should work", "probably done", "I think it's fixed".

### Debugging Threshold

After **3 failed fix attempts**, stop. Three failures signals an architectural problem. Surface to the user, discuss the root cause, consider restructuring — do not attempt a fourth patch.

### Test-Driven Development

All Go code in this repo follows strict TDD. Write the failing test first, watch it fail, then write the minimum code to pass. Never write production code before a failing test exists. Use table-driven tests (`[]struct{ ... }`) as the default pattern.

## Reference Repositories

The following repos are pre-authorized for reading and serve as design references:

- `/home/val/wrk/projects/aga2aga/context/preparation/` — design docs covering MCP integration patterns, the envelope protocol, Agent Evolution Protocol spec, gossip/consensus layers, ZK identity, and P2P trust graph
- `/home/val/wrk/github repos/0sel` — skill/agent reference implementations (superpowers, fullstack-skills, mcp-server-dev)
