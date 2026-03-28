# API Reference

Full reference for all exported APIs in aga2aga. Phase 1 (Skills Document Engine) and Phase 2 (MCP Gateway + Redis Transport + Admin) are complete. Stub packages are described with their interface definitions; concrete implementations are Phase 3-5 deliverables.

---

## pkg/protocol

`github.com/valpere/aga2aga/pkg/protocol`

Defines the canonical message type constants and the immutable type registry. No logic — pure data. Has no imports from other aga2aga packages.

### Constants

```go
const ProtocolVersion = "v1"
```

Wire version constant. **DO_NOT_TOUCH** — changing this breaks all existing agents.

### type MessageType

```go
type MessageType string
```

A string constant identifying the message type on the wire (the `type:` field in the YAML envelope).

#### Agent Evolution types (11)

| Constant | Wire value |
|----------|-----------|
| `AgentGenome` | `agent.genome` |
| `AgentSpawnProposal` | `agent.spawn.proposal` |
| `AgentSpawnApproval` | `agent.spawn.approval` |
| `AgentSpawnRejection` | `agent.spawn.rejection` |
| `AgentEvaluationRequest` | `agent.evaluation.request` |
| `AgentEvaluationResult` | `agent.evaluation.result` |
| `AgentPromotion` | `agent.promotion` |
| `AgentRollback` | `agent.rollback` |
| `AgentRetirement` | `agent.retirement` |
| `AgentQuarantine` | `agent.quarantine` |
| `AgentRecombineProposal` | `agent.recombine.proposal` |

#### Task types (5)

| Constant | Wire value |
|----------|-----------|
| `TaskRequest` | `task.request` |
| `TaskResult` | `task.result` |
| `TaskFail` | `task.fail` |
| `TaskProgress` | `task.progress` |
| `AgentMessage` | `agent.message` |

#### Negotiation types (8)

| Constant | Wire value |
|----------|-----------|
| `NegotiationPropose` | `negotiation.propose` |
| `NegotiationAccept` | `negotiation.accept` |
| `NegotiationReject` | `negotiation.reject` |
| `NegotiationCounter` | `negotiation.counter` |
| `NegotiationClarify` | `negotiation.clarify` |
| `NegotiationDelegate` | `negotiation.delegate` |
| `NegotiationCommit` | `negotiation.commit` |
| `NegotiationAbort` | `negotiation.abort` |

### type TypeMeta

```go
type TypeMeta struct {
    RequiredFields []string
    SchemaRef      string
}
```

Metadata for a registered message type.

- `RequiredFields` — field names checked by `ValidateStructural`. The two base envelope fields (`type`, `version`) are always required; this slice adds type-specific required fields.
- `SchemaRef` — name of the `$def` in the embedded JSON Schema used by `ValidateSchema`. Empty for task and negotiation types (structural-only validation).

### func Lookup

```go
func Lookup(mt MessageType) (TypeMeta, bool)
```

Returns a copy of the `TypeMeta` for the given message type, and whether it was found. Returns a copy to prevent external mutation of the registry.

### func Registered

```go
func Registered() []MessageType
```

Returns a copy of all registered message type keys. Order is not guaranteed. Returns a copy to prevent external mutation.

### func BaseEnvelopeFields

```go
func BaseEnvelopeFields() []string
```

Returns the two base envelope fields that are required for every message type: `["type", "version"]`. Returns a copy.

---

## pkg/document

`github.com/valpere/aga2aga/pkg/document`

The Skills Document engine. Imports `pkg/protocol`. Has no imports from `internal/` or `cmd/`.

### Wire Types

#### type StringOrList

```go
type StringOrList []string
```

A `[]string` that marshals as a YAML scalar when it contains a single element, and as a YAML sequence for multiple elements. Used for the `to:` envelope field.

```yaml
to: single-agent          # scalar — StringOrList{"single-agent"}
to:                       # sequence — StringOrList{"agent-a", "agent-b"}
  - agent-a
  - agent-b
```

#### type Envelope

```go
type Envelope struct {
    Type         MessageType  `yaml:"type"`
    Version      string       `yaml:"version"`
    ID           string       `yaml:"id,omitempty"`
    From         string       `yaml:"from,omitempty"`
    To           StringOrList `yaml:"to,omitempty"`
    CreatedAt    string       `yaml:"created_at,omitempty"`
    InReplyTo    string       `yaml:"in_reply_to,omitempty"`
    ThreadID     string       `yaml:"thread_id,omitempty"`
    ExecID       string       `yaml:"exec_id,omitempty"`
    TTL          string       `yaml:"ttl,omitempty"`
    Status       string       `yaml:"status,omitempty"`
    Signature    string       `yaml:"signature,omitempty"`
    SigningKeyID  string       `yaml:"signing_key_id,omitempty"`
}
```

The 13 standard wire fields present in every document's YAML header.

**Security:** `From` is a self-reported string. Authorization decisions MUST NOT be based on `From` alone until Phase 3 adds Ed25519 verification. See [SECURITY.md](SECURITY.md).

#### type Document

```go
type Document struct {
    Envelope                     // inlined — all 13 envelope fields promoted
    Extra    map[string]any       // all non-envelope YAML fields
    Body     string               // Markdown content after the YAML header
    Raw      []byte               // original bytes as parsed
}
```

The in-memory representation of a parsed Skills Document.

**Security:** `Extra` contains all YAML fields not defined in `Envelope`. It is unvalidated and must be treated as attacker-controlled on any network-facing path. Never use `Extra` directly for auth, signing, lifecycle state transitions, or routing decisions — always use `As[T]`.

#### type ValidationError

```go
type ValidationError struct {
    Layer   string // "structural", "schema", or "semantic"
    Field   string // YAML field path or field name
    Message string
}
```

A single validation finding. Returned as a slice by all validator methods.

Severity constants:

```go
const (
    LayerStructural = "structural"
    LayerSchema     = "schema"
    LayerSemantic   = "semantic"
)
```

### Parser

#### const MaxDocumentBytes

```go
const MaxDocumentBytes = 64 * 1024 // 64 KiB
```

Hard size limit enforced in `Parse` and `readAndParseFile`. Prevents YAML billion-laughs attacks (CWE-400). The intermediate JSON representation is bounded at `4 * MaxDocumentBytes` (256 KiB).

#### func SplitFrontMatter

```go
func SplitFrontMatter(raw []byte) (yamlBytes []byte, body string, err error)
```

Splits a `---`-delimited YAML front matter block from the Markdown body. Returns an error if the opening `---` delimiter is missing or the closing delimiter is not found.

#### func Parse

```go
func Parse(raw []byte) (*Document, error)
```

Parses a raw Skills Document byte slice into a `*Document`. Enforces the 64 KiB size limit. Splits the YAML front matter, unmarshals the envelope, and stores all remaining YAML fields in `Extra`.

Returns a non-nil error if the document exceeds `MaxDocumentBytes`, is missing required delimiters, or has malformed YAML.

#### func ParseAs

```go
func ParseAs[T any](raw []byte) (*T, error)
```

Parses a raw document and immediately converts it to a typed struct `T`. Equivalent to calling `Parse` followed by `As[T]`. Returns a non-nil error on parse failure or type conversion failure.

#### func Serialize

```go
func Serialize(doc *Document) ([]byte, error)
```

Reconstructs the canonical wire format from a `*Document`: `---\nyaml\n---\nbody`. All envelope fields and `Extra` fields are merged into the YAML header.

#### func As

```go
func As[T any](doc *Document) (*T, error)
```

Converts a `*Document` to a typed struct `T` via a YAML round-trip. Before marshaling, strips all 13 envelope YAML keys from `Extra` — this prevents an attacker from crafting a document where `Extra` contains a key like `from` or `type` that would shadow the real envelope value inside the typed struct (CWE-20).

```go
genome, err := document.As[document.AgentGenome](doc)
if err != nil {
    return fmt.Errorf("convert genome: %w", err)
}
```

### Validator

#### type Validator

```go
type Validator struct { /* unexported */ }
```

A 3-layer document validator. All per-type JSON schemas are pre-compiled at construction time, making the validator safe for concurrent use with no locking on the hot path.

#### func NewValidator

```go
func NewValidator(schemaBytes []byte) (*Validator, error)
```

Creates a new `Validator` from a JSON Schema 2020-12 YAML byte slice. Pre-compiles all `$def` schemas found in the registry. Returns an error if the schema is malformed or any `$def` referenced by the registry cannot be compiled.

#### func DefaultValidator

```go
func DefaultValidator() (*Validator, error)
```

Returns the process-wide singleton `Validator` backed by the embedded `schema.yaml`. The singleton is initialized via `sync.Once` with eager compilation at `init()` time. Safe for concurrent use. Returns a non-nil error only if schema compilation failed at startup (indicates a corrupted binary).

#### func (*Validator) ValidateStructural

```go
func (v *Validator) ValidateStructural(doc *Document) []ValidationError
```

Layer 1: checks that all fields in `TypeMeta.RequiredFields` are present. Returns an empty slice if all required fields are present. Returns `LayerStructural` errors for each missing field.

#### func (*Validator) ValidateSchema

```go
func (v *Validator) ValidateSchema(doc *Document) []ValidationError
```

Layer 2: validates the document against its JSON Schema 2020-12 `$def`. Only runs for agent evolution types (types with a non-empty `SchemaRef`). Returns `LayerSchema` errors for each schema violation.

#### func (*Validator) ValidateSemantic

```go
func (v *Validator) ValidateSemantic(doc *Document) []ValidationError
```

Layer 3: validates business rules not expressible in JSON Schema.

Current semantic rules:

- **Promotion / Rollback:** validates `from_status → to_status` against the lifecycle transition table; denies `from == target_agent` (self-promotion/rollback is forbidden).
- **Quarantine / Retirement:** validates `from_status → to_status` when `from_status` is present on the wire; always denies `from == target_agent`.

Returns `LayerSemantic` errors. In `--strict` mode (`aga2aga validate --strict`), semantic errors are treated as fatal.

#### func (*Validator) Validate

```go
func (v *Validator) Validate(doc *Document) []ValidationError
```

Runs all three layers in order: structural → schema → semantic. Short-circuits if structural validation fails (schema and semantic layers are skipped). Returns the combined error list.

### Builder

#### type Builder

```go
type Builder struct { /* unexported */ }
```

A fluent document builder. Methods chain in any order. A sticky-error guard captures the first error and propagates it to `Build()`, preventing silent partial construction.

#### func NewBuilder

```go
func NewBuilder(msgType protocol.MessageType) *Builder
```

Creates a new builder for the given message type.

#### Setter methods

| Method | Envelope field |
|--------|---------------|
| `ID(id string) *Builder` | `id` |
| `From(from string) *Builder` | `from` |
| `To(targets ...string) *Builder` | `to` |
| `ExecID(execID string) *Builder` | `exec_id` |
| `TTL(ttl string) *Builder` | `ttl` |
| `Status(status string) *Builder` | `status` |
| `InReplyTo(inReplyTo string) *Builder` | `in_reply_to` |
| `ThreadID(threadID string) *Builder` | `thread_id` |
| `Body(body string) *Builder` | Markdown body |

#### func (*Builder) Field

```go
func (b *Builder) Field(key string, value any) *Builder
```

Sets a type-specific (non-envelope) YAML field. Returns `b` unchanged but records a sticky error if `key` is a reserved envelope key (`type`, `version`, `id`, `from`, `to`, `created_at`, `in_reply_to`, `thread_id`, `exec_id`, `ttl`, `status`, `signature`, `signing_key_id`). The error is returned by `Build()`.

#### func (*Builder) Build

```go
func (b *Builder) Build() (*Document, error)
```

Finalizes the document. Auto-sets `version: v1` and `created_at` (RFC 3339 UTC, if not already set). Runs full 3-layer validation. Returns any sticky error accumulated by `Field()`. Returns an error if validation finds structural or schema violations.

#### Convenience builders

```go
func NewGenomeBuilder(agentID, kind string) *Builder
```
Pre-configures a builder for `agent.genome` with `from` set to `agentID` and the `kind` field set.

```go
func NewSpawnProposalBuilder(parentID, proposedID string) *Builder
```
Pre-configures a builder for `agent.spawn.proposal` with `from` set to `parentID` and the `proposed_id` field set.

```go
func NewTaskRequestBuilder(execID, from string) *Builder
```
Pre-configures a builder for `task.request` with `exec_id` and `from` set.

### Lifecycle State Machine

#### type LifecycleState

```go
type LifecycleState string
```

An agent lifecycle state. 11 constants from spec §16:

| Constant | Wire value | Terminal? |
|----------|-----------|-----------|
| `StateProposed` | `proposed` | no |
| `StateApprovedForSandbox` | `approved_for_sandbox` | no |
| `StateRejected` | `rejected` | yes |
| `StateSandbox` | `sandbox` | no |
| `StateCandidate` | `candidate` | no |
| `StateFailedSandbox` | `failed_sandbox` | yes |
| `StateActive` | `active` | no |
| `StateInactive` | `inactive` | no |
| `StateQuarantined` | `quarantined` | no |
| `StateRetired` | `retired` | yes |
| `StateRolledBack` | `rolled_back` | yes |

Terminal states have no outgoing transitions.

#### func ValidTransition

```go
func ValidTransition(from, to LifecycleState) bool
```

Returns true if the transition `from → to` is permitted by spec §16. The transition table is **DO_NOT_TOUCH** — modifying it breaks wire compatibility with agents running different versions.

**Security:** Wire-reported status fields (`Promotion.FromStatus`, `Rollback.FromStatus`, etc.) are self-reported strings. Executors MUST derive authoritative state from the state-store and call `ValidTransition` — never trust wire values.

#### func AllowedTransitions

```go
func AllowedTransitions(from LifecycleState) []LifecycleState
```

Returns a copy of all states reachable from `from`. Returns nil for terminal states. Returns a copy to prevent external mutation of the transition table.

### Typed Message Structs

All 24 message types have corresponding typed Go structs, used with `As[T]` or `ParseAs[T]`.

#### Task types (`types_task.go`)

```go
type TaskRequest  struct { Task string; Context string; ... }
type TaskResult   struct { Output string; ExecID string; ... }
type TaskFail     struct { Reason string; ExecID string; ... }
type TaskProgress struct { Step string; Progress int; ... }
```

#### Genome types (`types_genome.go`)

`AgentGenome` and its nested types:

```go
type AgentGenome struct {
    Kind          string
    Version       int
    Identity      Identity
    Lineage       Lineage
    Capabilities  Capabilities
    Tools         []Tools
    ModelPolicy   ModelPolicy
    PromptPolicy  PromptPolicy
    RoutingPolicy RoutingPolicy
    MemoryPolicy  MemoryPolicy
    Constraints   Constraints
    MutationPolicy MutationPolicy
    RetirementPolicy RetirementPolicy
    SandboxPolicy SandboxPolicy
    Fitness       Fitness
    FitnessMetrics FitnessMetrics
    Economics     Economics
}
```

**Security notes:**
- `PromptPolicy.Style` is `map[string]any` (open vocabulary per spec §4.3) — attacker-controlled; callers MUST sanitize before auth/signing/lifecycle use.
- `RoutingPolicy.Accepts`, `DelegatesTo`, `EscalationRules` are wire strings; dispatchers MUST sanitize against the protocol registry / state-store.
- `EscalationRule.Condition` is an opaque label — MUST NOT be executed as code (CWE-20); `EscalationRule.Target` MUST be validated in agent registry before dispatch (CWE-601).

#### Lifecycle types (`types_lifecycle.go`)

```go
type Promotion struct {
    TargetAgent string
    FromStatus  string  // self-reported wire string — DO NOT trust for auth
    ToStatus    string  // self-reported wire string — DO NOT trust for auth
    Reason      string  // opaque logging label only
}

type Rollback struct {
    TargetAgent string
    FromStatus  string
    ToStatus    string
    Reason      string
}

type Quarantine struct {
    TargetAgent string
    FromStatus  string  // optional (omitempty)
    ToStatus    string  // optional (omitempty)
    Reason      string
}

type Retirement struct {
    TargetAgent string
    FromStatus  string  // optional (omitempty)
    Reason      string
}

type RecombineProposal struct {
    CandidateID string  // self-reported — verify in state-store before use
    ParentIDs   []string
    Strategy    string
}
```

#### Spawn types (`types_spawn.go`)

```go
type SpawnProposal struct {
    ProposedID  string
    ParentID    string
    GenomePatch *GenomePatch  // mutable fields only — DO_NOT_TOUCH fields absent
}

type SpawnApproval  struct { ProposedID string; ApprovedBy string }
type SpawnRejection struct { ProposedID string; Reason string }
```

`GenomePatch` contains only mutable genome fields. The immutable fields (`id`, `lineage`, `genome_version`, `created_at`, `kind`) are structurally absent — patch-apply code cannot overwrite them.

#### Evaluation types (`types_evaluation.go`)

```go
type EvaluationRequest struct {
    TargetAgent    string
    Benchmarks     []string
    SuccessCriteria SuccessCriteria
}

type EvaluationResult struct {
    TargetAgent string
    Passed      bool
    Score       float64
    Metrics     MetricsComparison
}
```

---

## pkg/transport

`github.com/valpere/aga2aga/pkg/transport`

**Status: interface complete. Concrete Redis implementation in `pkg/transport/redis` (Phase 2 — done). Gossip P2P in Phase 5.**

### type Delivery

```go
type Delivery struct {
    Doc      *document.Document // parsed Skills Document — read-only after delivery
    MsgID    string             // opaque transport-layer token; use only for Ack calls
    RecvedAt time.Time          // wall-clock receive time; for monitoring, not business logic
}
```

Pairs a received document with its transport-layer acknowledgement token. `MsgID` is assigned by the concrete transport on receive (e.g., a Redis Streams entry ID). **SECURITY:** callers MUST use `Delivery.MsgID` for Ack calls — never derive it from document content or `Document.Extra`, which is attacker-controlled (CWE-20).

### type Transport

```go
type Transport interface {
    // Publish sends doc to the named topic.
    Publish(ctx context.Context, topic string, doc *document.Document) error

    // Subscribe returns a channel that yields Deliveries received on topic.
    // The channel is closed when ctx is cancelled or an unrecoverable error
    // occurs. Callers must drain the channel promptly to avoid blocking.
    Subscribe(ctx context.Context, topic string) (<-chan Delivery, error)

    // Ack acknowledges a specific message on a topic. topic and msgID must
    // come from a Delivery obtained via Subscribe — never from document
    // content or Document.Extra, which is attacker-controlled.
    Ack(ctx context.Context, topic, msgID string) error

    // Close shuts down the transport and releases resources.
    Close() error
}
```

---

## pkg/transport/redis

`github.com/valpere/aga2aga/pkg/transport/redis`

**Status: complete (Phase 2).** Implements `transport.Transport` over Redis Streams using `go-redis/v9`. Requires a running Redis instance.

### type RedisTransport

```go
type RedisTransport struct { /* unexported */ }
```

Implements `transport.Transport` using Redis Streams with consumer groups.

#### func New

```go
func New(rdb *redis.Client, groupID string) *RedisTransport
```

Creates a transport bound to the given Redis client. `groupID` is the consumer group name used for `XREADGROUP` operations. The client must already be connected and reachable.

#### func (*RedisTransport) Publish

```go
func (t *RedisTransport) Publish(ctx context.Context, topic string, doc *document.Document) error
```

Serializes `doc` to Skills Document wire format and appends it to the Redis stream named `topic` via `XADD`.

#### func (*RedisTransport) Subscribe

```go
func (t *RedisTransport) Subscribe(ctx context.Context, topic string) (<-chan transport.Delivery, error)
```

Subscribes to `topic` using `XREADGROUP`. Delivers one message per call — not a long-running goroutine. The channel is closed after the first delivery or when `ctx` is cancelled.

#### func (*RedisTransport) Ack

```go
func (t *RedisTransport) Ack(ctx context.Context, topic, msgID string) error
```

Acknowledges `msgID` on `topic` via `XACK`, removing it from the consumer group's Pending Entry List (PEL).

### type PendingMap

```go
type PendingMap struct { /* unexported */ }
```

Thread-safe map from task ID to the transport-layer `(topic, msgID)` pair needed to Ack on task completion or failure.

**SECURITY:** task IDs MUST be transport-layer entry IDs (`Delivery.MsgID`), never `Envelope.ID` or any document field.

```go
func NewPendingMap() *PendingMap
func (pm *PendingMap) Store(taskID, topic, msgID string)
func (pm *PendingMap) Load(taskID string) (topic, msgID string, ok bool)
func (pm *PendingMap) LoadAndDelete(taskID string) (topic, msgID string, ok bool)
func (pm *PendingMap) Delete(taskID string)
func (pm *PendingMap) StartCleanup(ctx context.Context, ttl time.Duration)
```

`StartCleanup` is idempotent via `sync.Once` — safe to call multiple times.

---

## pkg/identity

`github.com/valpere/aga2aga/pkg/identity`

**Status: stub — concrete Ed25519 implementation in Phase 3.**

### type Identity

```go
type Identity struct {
    Pseudonym string             // human-readable agent identifier
    PublicKey ed25519.PublicKey  // Ed25519 public key (32 bytes)
}
```

Uses `crypto/ed25519.PublicKey` (not `[]byte`) to enforce the 32-byte semantic at compile time.

### type Signer

```go
type Signer interface {
    // Sign returns an Ed25519 signature over data using the agent's private key.
    Sign(data []byte) ([]byte, error)

    // Verify checks whether sig is a valid Ed25519 signature over data for this identity.
    // Returns (false, nil) for a valid key material that simply doesn't match.
    // Returns (false, error) when the operation itself failed (bad key, I/O error).
    // Callers MUST check error before trusting the bool — a non-nil error means
    // the result is indeterminate (CWE-252).
    Verify(data, sig []byte) (bool, error)
}
```

---

## pkg/negotiation

`github.com/valpere/aga2aga/pkg/negotiation`

**Status: stub — full state machine in Phase 4.**

### type NegotiationState

```go
type NegotiationState string
```

Constants derived from `pkg/protocol` to prevent drift:

| Constant | Wire value |
|----------|-----------|
| `StatePropose` | `negotiation.propose` |
| `StateAccept` | `negotiation.accept` |
| `StateReject` | `negotiation.reject` |
| `StateCounter` | `negotiation.counter` |
| `StateClarify` | `negotiation.clarify` |
| `StateDelegate` | `negotiation.delegate` |
| `StateCommit` | `negotiation.commit` |
| `StateAbort` | `negotiation.abort` |

### func NegotiationTransition

```go
func NegotiationTransition(from, to NegotiationState) bool
```

**STUB:** Always returns false for every input pair. Do NOT use as a gate or guard before Phase 4 implements the real state machine.

---

## pkg/admin

`github.com/valpere/aga2aga/pkg/admin`

**Status: complete (Phase 2).** Domain types, persistence interfaces, and policy evaluation used by both the admin server and the gateway enforcer. Has no imports from `internal/` or `cmd/`.

### Domain types

| Type | Description |
|------|-------------|
| `Organization` | Top-level tenant. Owns users, agents, and policies. |
| `User` | Human operator. Has a `Role` (admin/operator/viewer). Password stored as bcrypt hash. |
| `RegisteredAgent` | An agent authorized to operate within an org. `AgentID` matches `Envelope.From`. |
| `CommunicationPolicy` | Permit or deny message flow between two principals. Supports `Wildcard` (`"*"`). Priority-ordered; first match wins; default deny. |
| `AuditEvent` | Append-only log entry for security-relevant operations. |
| `APIKey` | Bearer key for machine-to-machine admin API access. `KeyHash` is a SHA-256 hex digest; the raw key is never stored. |

### Persistence interfaces

```go
type Store interface {
    OrgStore
    UserStore
    AgentStore
    PolicyStore
    AuditStore
    APIKeyStore
}
```

Each sub-interface covers CRUD for its domain type. The concrete implementation is `internal/admin.SQLiteStore`.

**Security note (`APIKeyStore`):** `RevokeAPIKey(ctx, orgID, id)` requires `orgID` to prevent cross-org revocations (CWE-639). `ListAPIKeys` excludes revoked keys — callers never see them.

### func Evaluate

```go
func Evaluate(policies []CommunicationPolicy, source, target string) PolicyAction
```

Returns `PolicyActionAllow` if any policy in `policies` permits `source → target`; otherwise `PolicyActionDeny`. Evaluation order: sorted by `Priority` descending (highest first). Wildcard matching: `*` matches any principal within the same policy set. Bidirectional policies match both `source → target` and `target → source`.

---

## internal/gateway

`github.com/valpere/aga2aga/internal/gateway`

**Status: complete (Phase 2).** MCP Gateway implementation. Not importable from `pkg/` or `cmd/` (Clean Architecture boundary).

### type PolicyEnforcer

```go
type PolicyEnforcer interface {
    Allowed(ctx context.Context, source, target string) (bool, error)
}
```

Checks whether `source` may communicate with `target` under the current policy set.

Two implementations ship:

**`EmbeddedEnforcer`** — in-process evaluation via `admin.PolicyStore`. Use for single-node deployments where the gateway and admin share a database.

```go
func NewEmbeddedEnforcer(store admin.PolicyStore, orgID string) *EmbeddedEnforcer
```

**`HTTPEnforcer`** — remote evaluation via the admin server's `GET /api/v1/evaluate` endpoint. Use for separate-process deployments.

```go
func NewHTTPEnforcer(baseURL, token string) (*HTTPEnforcer, error)
```

`baseURL` must be `http` or `https` with a non-empty host (validated at construction; CWE-918). Responses are capped at 4 KiB (CWE-400). The Bearer token is never included in error messages (CWE-532).

### type Config

```go
type Config struct {
    AgentID         string        // gateway identity used in policy checks
    TaskReadTimeout time.Duration // max wait for get_task / receive_message (default: 5s)
    PendingTTL      time.Duration // pending task entry TTL (default: 5m)
}

func DefaultConfig() Config
```

### type Gateway

```go
type Gateway struct { /* unexported */ }

func New(t transport.Transport, e PolicyEnforcer, cfg Config) *Gateway
func (g *Gateway) Run(ctx context.Context, mcpTransport mcpsdk.Transport) error
func (g *Gateway) Server() *mcpsdk.Server
func (g *Gateway) StartCleanup(ctx context.Context)
```

`New` registers all 6 MCP tools. `Run` starts `PendingMap` cleanup and calls `mcpsdk.Server.Run`. `Server()` exposes the underlying MCP server for HTTP/SSE integrations that bypass `Run`. `StartCleanup` is idempotent (backed by `sync.Once`).

### MCP Tools

| Tool | Stream operation | Policy check |
|------|-----------------|--------------|
| `get_task` | `XREADGROUP` from `agent.tasks.<agent>` | `agent → orchestrator` |
| `complete_task` | `XADD` to `agent.events.completed` + `XACK` | `agent → orchestrator` |
| `fail_task` | `XADD` to `agent.events.failed` + `XACK` | `agent → orchestrator` |
| `heartbeat` | no-op | none |
| `send_message` | `XADD` to `agent.messages.<to>` | `agent → recipient` (peer-to-peer) |
| `receive_message` | `XREADGROUP` from `agent.messages.<agent>` + `XACK` | `agent → orchestrator` |

**Security invariants:**
- Agent and recipient IDs validated via DNS-label regex `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}[a-zA-Z0-9]$` (CWE-20/CWE-74)
- Bodies capped at `document.MaxDocumentBytes` (64 KiB) on both send and receive paths (CWE-400)
- `taskID = Delivery.MsgID` — never `Envelope.ID` or any document field
- `SECURITY(Phase 3)` comment at every `Allowed()` call site — `agent` field is self-reported until Ed25519 verification lands (CWE-287)

---

## cmd/admin

`github.com/valpere/aga2aga/cmd/admin`

Binary name: `aga2aga-admin`. HTTP admin server.

```
aga2aga-admin --addr :8080 --db ./admin.db
```

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | Listen address |
| `--db` | `./admin.db` | SQLite database path |

Seeds a default `admin` user on first run (password must be changed on first login). Exposes the `GET /api/v1/evaluate?source=X&target=Y` endpoint for `HTTPEnforcer`.

---

## cmd/gateway

`github.com/valpere/aga2aga/cmd/gateway`

Binary name: `aga2aga-gateway`. MCP Gateway server.

```
aga2aga-gateway [flags]
```

| Flag | Env override | Default | Description |
|------|-------------|---------|-------------|
| `--agent-id` | | `mcp-gateway` | Gateway identity |
| `--redis-addr` | | `localhost:6379` | Redis address |
| `--redis-password` | | | Redis password |
| `--redis-db` | | `0` | Redis database number |
| `--group-id` | | `aga2aga` | Redis consumer group |
| `--transport` | | `stdio` | MCP transport: `stdio` or `http` |
| `--http-addr` | | `:8081` | Listen address (HTTP transport) |
| `--admin-url` | | | Admin server URL for `HTTPEnforcer` |
| `--admin-db` | | | SQLite path for `EmbeddedEnforcer` |
| `--task-read-timeout` | | `5s` | Timeout for `get_task` / `receive_message` |
| `ADMIN_API_KEY` | env | | Bearer token for `HTTPEnforcer` (CWE-214: prefer env over flag) |

`--admin-db` path is resolved through `filepath.EvalSymlinks` before use (CWE-22/61). The HTTP server uses `WriteTimeout: 0` to support long-lived SSE connections.

---

## cmd/aga2aga

`github.com/valpere/aga2aga/cmd/aga2aga`

CLI tool built with cobra. Three subcommands.

### aga2aga validate

```
aga2aga validate <file> [--strict]
```

Runs all 3 validation layers against `<file>`. Prints each `ValidationError` with its layer and field. Exits zero on success.

| Flag | Default | Description |
|------|---------|-------------|
| `--strict` | false | Treat semantic warnings as fatal errors |

```bash
aga2aga validate tests/testdata/valid_genome.md
# valid_genome.md: OK

aga2aga validate --strict tests/testdata/invalid_doc.md
# invalid_doc.md: [structural] type: required field missing
# exit 1
```

### aga2aga create

```
aga2aga create <type> [flags]
```

Builds a document of the given message type via the fluent `Builder` and writes it to stdout or `--out`.

`<type>` must be a registered `MessageType` wire value (e.g., `task.request`, `agent.genome`).

| Flag | Description |
|------|-------------|
| `--id <string>` | Document ID |
| `--from <string>` | Sender agent ID |
| `--to <string>` | Recipient — repeat for multiple |
| `--exec-id <string>` | Execution/workflow ID (`exec_id` field) |
| `--field key=value` | Type-specific field — repeat for multiple |
| `--out <file>` | Write to file instead of stdout |

```bash
aga2aga create task.request \
  --from orchestrator \
  --to agent-alpha \
  --exec-id exec-001 \
  --field "task=Analyze the dependency graph"

aga2aga create agent.genome \
  --from meta-evolver \
  --field "kind=worker" \
  --field "version=1" \
  --out genome.md
```

### aga2aga inspect

```
aga2aga inspect <file> [--format text|json]
```

Prints envelope fields from `<file>`.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

```bash
aga2aga inspect genome.md
# type:     agent.genome
# id:       01HN7K2P3Q4R5S6T7U8V9W0X1Y
# version:  v1
# from:     meta-evolver
# created:  2024-01-15T10:30:00Z

aga2aga inspect genome.md --format json
# {
#   "type": "agent.genome",
#   "id": "...",
#   "extra": { "kind": "worker", "version": 1 }
# }
```

**Security:** JSON output nests `doc.Extra` under the `"extra"` key to prevent envelope key shadowing. Full paths are never shown in output — only `filepath.Base(path)` — to avoid leaking directory structure (CWE-200).

---

## Usage Examples

### Parse and validate

```go
import (
    "github.com/valpere/aga2aga/pkg/document"
)

raw, err := os.ReadFile("genome.md")
if err != nil {
    return err
}

doc, err := document.Parse(raw)
if err != nil {
    return fmt.Errorf("parse: %w", err)
}

v, err := document.DefaultValidator()
if err != nil {
    return err
}

errs := v.Validate(doc)
for _, e := range errs {
    fmt.Printf("[%s] %s: %s\n", e.Layer, e.Field, e.Message)
}
```

### Build a document

```go
doc, err := document.NewTaskRequestBuilder("exec-001", "orchestrator").
    To("agent-alpha").
    Field("task", "Analyze the dependency graph").
    Body("## Task\n\nAnalyze `pkg/document/` for all `As[T]` callers.").
    Build()
if err != nil {
    return fmt.Errorf("build: %w", err)
}

raw, err := document.Serialize(doc)
```

### Access typed struct

```go
genome, err := document.As[document.AgentGenome](doc)
if err != nil {
    return fmt.Errorf("convert: %w", err)
}
fmt.Println(genome.Kind, genome.Version)
```

### Lifecycle transition check

```go
import "github.com/valpere/aga2aga/pkg/document"

ok := document.ValidTransition(document.StateCandidate, document.StateActive)
// ok == true

ok = document.ValidTransition(document.StateRetired, document.StateActive)
// ok == false — retired is terminal

allowed := document.AllowedTransitions(document.StateActive)
// ["inactive", "quarantined", "retired"]
```

### Registry lookup

```go
import "github.com/valpere/aga2aga/pkg/protocol"

meta, ok := protocol.Lookup(protocol.TaskRequest)
if !ok {
    return errors.New("unknown message type")
}
fmt.Println(meta.RequiredFields) // ["exec_id", "from"]
```
