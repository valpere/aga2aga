# Roadmap

aga2aga is being built in five phases, each independently deployable and testable. Phase 1 is complete. Phases 2–5 are planned but not yet implemented.

---

## Phase 1: Skills Document Engine (complete)

Goal: parse, validate, and create all 24 Skills Document message types.

- [x] `pkg/protocol` — 24 `MessageType` constants, immutable type registry, `TypeMeta`
- [x] `pkg/document` — `Document`, `Envelope`, `StringOrList` wire types
- [x] `pkg/document` — `Parse`, `ParseAs[T]`, `Serialize`, `SplitFrontMatter`, `As[T]`
- [x] `pkg/document` — 3-layer `Validator` (structural / JSON Schema 2020-12 / semantic)
- [x] `pkg/document` — `DefaultValidator()` singleton, embedded `schema.yaml`
- [x] `pkg/document` — Fluent `Builder` with sticky-error guard and full validation on `Build()`
- [x] `pkg/document` — Lifecycle state machine: 11 states, `ValidTransition`, `AllowedTransitions`
- [x] `pkg/document` — Typed structs for all 24 message types (task, genome, lifecycle, spawn, evaluation)
- [x] `cmd/aga` — `aga validate`, `aga create`, `aga inspect` CLI tool
- [x] Stub interfaces for future phases: `pkg/transport`, `pkg/identity`, `pkg/negotiation`
- [x] CI pipeline: golangci-lint v2, coverage reporting, go mod tidy drift check
- [x] 100+ tests, all green

---

## Phase 2: MCP Gateway + Redis Transport

Goal: a running gateway that accepts MCP calls from Claude Code / Codex and routes them through Redis Streams.

Planned deliverables:

- `pkg/transport` — Redis Streams implementation of the `Transport` interface
  - `Publish`: `XADD` to topic stream
  - `Subscribe`: `XREADGROUP` consumer loop, returns `<-chan *document.Document`
  - `Ack`: `XACK` on the originating stream
- `internal/gateway` — MCP Gateway implementation
  - MCP tool handlers: `get_task`, `complete_task`, `fail_task`, `heartbeat`
  - Pending map: `taskID → msgID` for deferred ACKs
  - Session state management for stateless closed agents
- `cmd/gateway` — Gateway binary with config (Redis URL, agent ID, topic prefix)
- Docker: multi-stage build, distroless runtime image
- Integration tests against a real Redis instance

Design references: `../context/preparation/01-part/02-build_gateway.md`, `01-part/00-mcp.md`

---

## Phase 3: Ed25519 Identity

Goal: every document is signed by the sending agent and every gateway verifies signatures before routing.

Planned deliverables:

- `pkg/identity` — Ed25519 keypair generation, `NewIdentity` constructor
- `pkg/identity` — Concrete `Signer` implementation using `crypto/ed25519`
- `pkg/document` — `Sign(doc, signer)` and `Verify(doc, identity)` functions
- `internal/gateway` — Sign outgoing documents; verify `From` against `Signature` + `SigningKeyID`
- Trust-on-first-use (TOFU) for new agents

Design references: `../context/preparation/01-part/09-P2P_identity+trust_graph.md`, `02-part/01-agent_name_is_not_identity.md`

---

## Phase 4: Negotiation Protocol

Goal: agents can negotiate task parameters, resource allocation, and collaboration terms using the structured negotiation message types.

Planned deliverables:

- `pkg/negotiation` — Full `NegotiationTransition` state machine (replacing the current stub)
- `pkg/negotiation` — `NegotiationSession` type tracking conversation state
- `internal/gateway` — Negotiation message routing and session lifecycle
- Round handling: propose → accept/reject/counter → commit/abort
- Clarify and delegate sub-flows

Design references: `../context/preparation/01-part/04-agent-to-agent_negotiation_protocol.md`

---

## Phase 5: Gossip P2P + Offline-First

Goal: eliminate Redis as a hard dependency. Agents can form a swarm and exchange Skills Documents directly via gossip, with no centralized broker.

Planned deliverables:

- `pkg/transport` — Gossip P2P transport replacing Redis Streams
- Raft-lite consensus layer for deterministic agreement on state changes
- CRDT-based conflict resolution for concurrent genome mutations
- Local-first: full functionality with no network, sync on reconnect
- Agent discovery via gossip without a registry server

Design references: `../context/preparation/01-part/05-self-organizing_agent_swarm.md`, `06-consensus_layer.md`, `07-gossip_protocol.md`, `08-fully_offline-first_swarm.md`

---

## Research-Grade (No Timeline)

These directions are explored in the design docs but are not scheduled for implementation:

- **ZK Trust** — agents prove trustworthiness without revealing why (zero-knowledge proofs). Reference: `01-part/10-zero-knowledge_trust.md`, `01-part/11-zk-identity.md`
- **Privacy-preserving collaboration** — secure multi-party computation between agents. Reference: `01-part/12-privacy-preserving_multi-agent_collaboration.md`
- **Federated learning** — agents improve shared models without sharing raw data. Reference: `01-part/13-secure_federated_learning_between_agents.md`
- **Self-improving swarm** — continuous autonomous improvement loop. Reference: `01-part/14-self-improving_swarm.md`
- **Open-ended evolution** — agents design and spawn new agent architectures. Reference: `01-part/15-open-ended_evolution.md`

---

## Non-Goals

The following are explicitly out of scope at all phases:

- **Embedding an SDK into closed agents.** The gateway pattern exists precisely because this is impossible for Claude Code, Codex, and Gemini CLI.
- **Building a general-purpose framework.** aga2aga is purpose-built for the AI agent orchestration use case.
- **Centralized control plane.** The architecture moves toward fully decentralized operation in Phase 5. No hosted service, no SaaS, no control plane.
- **Supporting non-MCP agent interfaces.** MCP is the only supported agent-facing protocol.
