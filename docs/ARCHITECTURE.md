# Architecture

## Design Motivation

Closed AI agents — Claude Code, Codex CLI, Gemini CLI — have no SDK hooks. You cannot embed a Go library into a process you do not control. What all of them already speak natively is the Model Context Protocol (MCP): a standard JSON-RPC interface that agents use to call tools. The aga2aga gateway exploits this: it exposes a small set of MCP tools that the agents already understand, and translates between MCP calls and a Redis Streams–based orchestration bus internally.

The wire format is the envelope document: a Markdown file with a YAML control header. The YAML header is machine-parsed; the Markdown body is forwarded to the agent verbatim. This keeps the protocol human-readable while remaining machine-actionable.

## Data Flow

```
  Claude Code / Codex CLI / Gemini CLI
           |
           | MCP (stdio or HTTP)
           v
  +------------------+
  |   MCP Gateway    |  <-- pending map: taskID -> msgID
  |  (cmd/gateway)   |
  +------------------+
           |
           | Redis Streams XADD / XREADGROUP
           v
  +------------------+
  |   Orchestrator   |
  +------------------+
```

Agents communicate by exchanging **messages** through the gateway. A **task** is a specialised message that requires an explicit outcome (complete or fail). The gateway exposes six MCP tools:

**Messaging** — fire-and-forget peer-to-peer:

| MCP Tool          | Redis operation                                      |
|-------------------|------------------------------------------------------|
| `send_message`    | `XADD` to `agent.messages.<recipient>`               |
| `receive_message` | `XREADGROUP` from `agent.messages.<agent>` + `XACK`  |

**Task lifecycle** — request-response with guaranteed delivery:

| MCP Tool        | Redis operation                               |
|-----------------|-----------------------------------------------|
| `get_task`      | `XREADGROUP` from `agent.tasks.<agent-id>`    |
| `complete_task` | `XADD` to `agent.events.completed` + `XACK`  |
| `fail_task`     | `XADD` to `agent.events.failed`               |

**Utility:**

| MCP Tool    | Redis operation   |
|-------------|-------------------|
| `heartbeat` | health check only |

The gateway maintains an in-memory `pending map[taskID]msgID` so it can `XACK` the correct Redis message when the agent reports completion. Messaging tools ack immediately on receive — no pending tracking needed.

## Package Dependency Graph

Dependencies flow strictly downward. No package imports a layer above it.

```
  pkg/protocol         -- MessageType constants and registry (no imports)
       ^
  pkg/document         -- parser, validator, builder, lifecycle (imports protocol)
       ^
  pkg/transport        -- Transport interface (imports document)
  pkg/identity         -- Identity, Signer (no document imports)
  pkg/negotiation      -- NegotiationState (imports protocol)
       ^
  internal/gateway     -- MCP Gateway implementation (imports all pkg/)
       ^
  cmd/gateway          -- MCP Gateway binary (imports internal/gateway)
  cmd/enveloper              -- CLI tool (imports pkg/document, pkg/protocol)
```

Clean Architecture rule: `pkg/` packages never import `internal/` or `cmd/`. `cmd/` packages are thin entry points only.

## pkg/protocol

`github.com/valpere/aga2aga/pkg/protocol`

Defines the canonical message type constants and the immutable type registry. No logic — pure data.

### MessageType Constants

24 types across four groups:

**Agent Evolution (11):**

| Constant               | Wire value                    |
|------------------------|-------------------------------|
| `AgentGenome`          | `agent.genome`                |
| `AgentSpawnProposal`   | `agent.spawn.proposal`        |
| `AgentSpawnApproval`   | `agent.spawn.approval`        |
| `AgentSpawnRejection`  | `agent.spawn.rejection`       |
| `AgentEvaluationRequest` | `agent.evaluation.request`  |
| `AgentEvaluationResult`  | `agent.evaluation.result`   |
| `AgentPromotion`       | `agent.promotion`             |
| `AgentRollback`        | `agent.rollback`              |
| `AgentRetirement`      | `agent.retirement`            |
| `AgentQuarantine`      | `agent.quarantine`            |
| `AgentRecombineProposal` | `agent.recombine.proposal`  |

**Agent Message (1):** fire-and-forget peer-to-peer; no outcome required.

| Constant       | Wire value      |
|----------------|-----------------|
| `AgentMessage` | `agent.message` |

**Task (4):** request-response work units; outcome required via `task.result` or `task.fail`.

| Constant       | Wire value         |
|----------------|--------------------|
| `TaskRequest`  | `task.request`     |
| `TaskResult`   | `task.result`      |
| `TaskFail`     | `task.fail`        |
| `TaskProgress` | `task.progress`    |

**Negotiation (8):**

| Constant               | Wire value               |
|------------------------|--------------------------|
| `NegotiationPropose`   | `negotiation.propose`    |
| `NegotiationAccept`    | `negotiation.accept`     |
| `NegotiationReject`    | `negotiation.reject`     |
| `NegotiationCounter`   | `negotiation.counter`    |
| `NegotiationClarify`   | `negotiation.clarify`    |
| `NegotiationDelegate`  | `negotiation.delegate`   |
| `NegotiationCommit`    | `negotiation.commit`     |
| `NegotiationAbort`     | `negotiation.abort`      |

### Registry

The `registry` map is unexported. Callers use `Lookup(mt)` (returns a copy of `TypeMeta`) and `Registered()` (returns a copy of all keys). This copy-on-read pattern prevents external mutation of the registry.

`TypeMeta` carries `RequiredFields []string` and `SchemaRef string`. Agent evolution types have a non-empty `SchemaRef` pointing to a `$def` name in the embedded JSON Schema. Task and negotiation types use structural validation only.

## pkg/document

`github.com/valpere/aga2aga/pkg/document`

The envelope document engine: the most substantial package in Phase 1.

### Wire Types

**`StringOrList`** — a `[]string` that marshals as a scalar when it contains a single element and as a YAML sequence otherwise. Used for the `to:` envelope field.

**`Envelope`** — the 13 standard wire fields present in every document header:

```
type      version     id          from        to
created_at  in_reply_to  thread_id   exec_id
ttl         status      signature   signing_key_id
```

`From` is self-reported and cryptographically unverified until Phase 3.

**`Document`** — `Envelope` (inlined) + `Extra map[string]any` (all non-envelope YAML fields) + `Body string` (Markdown after the header) + `Raw []byte` (original bytes).

### Parser

`Parse(raw []byte) (*Document, error)` enforces a hard 64 KiB limit (`MaxDocumentBytes`), splits the YAML front matter from the Markdown body, unmarshals the envelope, and stores the remaining YAML fields in `Extra`.

`Serialize(doc *Document) ([]byte, error)` reconstructs the canonical wire format: `---\nyaml\n---\nbody`.

`ParseAs[T any](raw []byte) (*T, error)` and `As[T any](doc *Document) (*T, error)` unmarshal type-specific structs from `Extra` via a YAML round-trip. `As[T]` strips all 13 envelope YAML keys from `Extra` before marshaling — preventing an attacker from shadowing envelope fields with values inside `Extra`.

### Validator (3 layers)

`DefaultValidator()` returns a process-wide singleton backed by the embedded `schema.yaml`. The singleton is initialized via `sync.Once` at `init()` time with all per-type schemas pre-compiled — safe for concurrent callers.

| Layer | Name | What it checks |
|-------|------|----------------|
| 1 | `ValidateStructural` | Required fields per the protocol registry (`TypeMeta.RequiredFields`) |
| 2 | `ValidateSchema` | JSON Schema 2020-12 validation against the `$def` for the message type (agent evolution types only) |
| 3 | `ValidateSemantic` | Lifecycle transition legality (`ValidTransition`) + self-action denial for governance types |

`Validate(doc)` runs all three layers in order and short-circuits if structural validation fails.

Semantic layer rules:
- For `agent.promotion` and `agent.rollback`: validates `from_status → to_status` against the transition table; denies `from == target_agent` (self-promotion/rollback).
- For `agent.quarantine` and `agent.retirement`: validates `from_status → to_status` when `from_status` is present on the wire; always denies `from == target_agent`.

### Builder

`NewBuilder(msgType)` creates a fluent builder. Setters: `ID`, `From`, `To`, `ExecID`, `TTL`, `Status`, `InReplyTo`, `ThreadID`, `Body`, `Field`. `Field(key, value)` rejects reserved envelope keys with a sticky error that propagates to `Build()`. `Build()` auto-sets `version: v1` and `created_at`, then runs full 3-layer validation.

Convenience constructors: `NewGenomeBuilder(agentID, kind)`, `NewSpawnProposalBuilder(parentID, proposedID)`, `NewTaskRequestBuilder(execID, from)`.

### Lifecycle State Machine

11 states from spec §16. Terminal states (no outgoing transitions) are `retired`, `rejected`, `failed_sandbox`, `rolled_back`.

```
                     +-----------+
                     | proposed  |
                     +-----+-----+
                           |
              +------------+------------+
              v                         v
  +--------------------+           +----------+
  | approved_for_sandbox|          | rejected |  (terminal)
  +--------+-----------+           +----------+
           |
           v
       +-------+
       | sandbox|
       +---+---+
           |
     +-----+-----+
     v           v
+----------+  +--------------+
|candidate |  |failed_sandbox|  (terminal)
+----+-----+  +--------------+
     |
     v
  +------+
  |active|<--------+
  +--+---+         |
     |             |
     +------+------+
            |
     +------+------+------+
     v             v       v
+----------+  +----------+ +--------+
| inactive |  |quarantined| |retired| (terminal)
+----+-----+  +-----+-----+ +--------+
     |               |
     v               v
  (active)        +--------+
                  |retired | (terminal)
                  +--------+

rolled_back  (terminal — reachable from candidate)
```

`ValidTransition(from, to LifecycleState) bool` enforces spec §16. The transition table is `DO_NOT_TOUCH` — modifying it breaks wire compatibility.

### Typed Message Structs

24 concrete Go structs across 5 source files:

| File | Types |
|------|-------|
| `types_task.go` | `TaskRequest`, `TaskResult`, `TaskFail`, `TaskProgress` |
| `types_genome.go` | `AgentGenome` (+ nested: `Identity`, `Lineage`, `Capabilities`, `Tools`, `ModelPolicy`, `PromptPolicy`, `EscalationRule`, `RoutingPolicy`, `MemoryPolicy`, `Thresholds`, `Economics`, `FitnessMetrics`, `Fitness`, `Constraints`, `MutationPolicy`, `RetirementPolicy`, `SandboxPolicy`, `GenomePatch`) |
| `types_lifecycle.go` | `Promotion`, `Rollback`, `Quarantine`, `Retirement`, `RecombineProposal` |
| `types_spawn.go` | `SpawnProposal`, `SpawnApproval`, `SpawnRejection` |
| `types_evaluation.go` | `SuccessCriteria`, `MetricsComparison`, `EvaluationRequest`, `EvaluationResult` |

`GenomePatch` contains only the mutable subset of genome fields — `DO_NOT_TOUCH` fields (`id`, `lineage`, `genome_version`, `created_at`, `kind`) are structurally absent, preventing patch-apply from overwriting immutable fields.

## cmd/enveloper

`github.com/valpere/aga2aga/cmd/enveloper`

CLI tool built with cobra. Three subcommands:

- `aga2aga-enveloper validate <file> [--strict]` — runs all 3 validation layers; `--strict` promotes semantic warnings to fatal errors.
- `aga2aga-enveloper create <type> [flags]` — builds any registered message type via fluent builder flags and writes to stdout or `--out`.
- `aga2aga-enveloper inspect <file> [--format text|json]` — prints envelope fields; JSON output nests `Extra` under `"extra"` to prevent key shadowing.

`readAndParseFile(path)` is the shared helper that resolves symlinks (`filepath.EvalSymlinks`), enforces the size limit, and calls `document.Parse`.

## Security Model

See [SECURITY.md](SECURITY.md) for the full treatment. Summary:

**Phase 1 trust boundary.** There is no cryptographic authentication in Phase 1. `Envelope.From` is a self-reported string. Authorization decisions must not rely on `From` alone until Phase 3 adds Ed25519 signing.

**`Document.Extra` is attacker-controlled.** Never use `Extra` directly for auth, signing, lifecycle, or routing decisions. Always extract typed structs via `As[T]`, which strips all 13 envelope YAML keys before marshaling to prevent injection attacks (CWE-20).

**Wire-reported status fields are unverified.** `Promotion.FromStatus`, `Rollback.FromStatus`, `Quarantine.FromStatus`, `Retirement.FromStatus` are self-reported strings on the wire. Executors must derive authoritative state from the state-store and call `ValidTransition()` — they must never trust wire values.

**Self-action governance.** The semantic validator denies `from == target_agent` for all four governance types (promotion, rollback, quarantine, retirement). This check fires regardless of whether `from_status` is present on the wire.

**Size limits.** `MaxDocumentBytes = 65536` (64 KiB) prevents YAML billion-laughs attacks (CWE-400).

## Future Layers

See [ROADMAP.md](ROADMAP.md) for detailed phase plans.

- **Phase 2** adds the MCP Gateway binary (`cmd/gateway`) and a Redis Streams implementation of `pkg/transport`.
- **Phase 3** implements `pkg/identity` with Ed25519 keypair generation, document signing, and `From` field verification.
- **Phase 4** implements the `pkg/negotiation` state machine for the 8 negotiation message types.
- **Phase 5** replaces Redis with a gossip P2P transport for fully offline operation.

## Design Principles

Applied throughout the codebase:

- **Copy-on-read** for shared data: `Registered()`, `AllowedTransitions()`, and `BaseEnvelopeFields()` all return copies. External callers cannot mutate registry or table state.
- **sync.Once singleton** for `DefaultValidator()`: eager initialization at `init()` time pre-compiles all per-type JSON schemas, making the validator safe for concurrent use with zero runtime locking on the hot path.
- **Sticky errors** in `Builder.Field()`: once a reserved key is used, the error is stored and propagated to `Build()`, eliminating partial-construction traps.
- **DO_NOT_TOUCH markers** protect spec §16 wire compatibility: the lifecycle transition table, `ValidTransition()` signature, `ProtocolVersion`, and immutable genome fields are annotated to prevent accidental modification.
- **YAGNI**: stub packages (`transport`, `identity`, `negotiation`, `gateway`) define interfaces and types now so import paths are stable, but contain no logic until the implementing phase.
