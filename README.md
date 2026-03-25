# aga2aga

[![CI](https://github.com/valpere/aga2aga/actions/workflows/ci.yml/badge.svg)](https://github.com/valpere/aga2aga/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/valpere/aga2aga.svg)](https://pkg.go.dev/github.com/valpere/aga2aga)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An MCP Gateway that bridges closed AI agents — Claude Code, Codex CLI, Gemini CLI — to a Redis Streams-based orchestration bus using the Skills Document protocol.

## The Problem

Closed AI agents have no SDK hooks. You cannot embed a Go library into a process you do not control. What all of them already speak natively is the Model Context Protocol (MCP): a standard JSON-RPC interface that every modern agent uses to call tools.

aga2aga exploits this: it exposes a small set of MCP tools the agents already understand and translates between MCP calls and a Redis Streams orchestration bus internally. No agent modification required.

## How It Works

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

The gateway maps four MCP tools to Redis operations:

| MCP Tool        | Redis operation                               |
|-----------------|-----------------------------------------------|
| `get_task`      | `XREADGROUP` from `agent.tasks.<agent-id>`    |
| `complete_task` | `XADD` to `agent.events.completed` + `XACK`  |
| `fail_task`     | `XADD` to `agent.events.failed`               |
| `heartbeat`     | health check only                             |

### Skills Document Format

All inter-agent messages are Markdown files with a YAML control header. The YAML is machine-parsed; the Markdown body goes to the agent verbatim:

```markdown
---
type: task.request
version: v1
id: 01HN7K2P3Q4R5S6T7U8V9W0X1Y
from: orchestrator
to: agent-alpha
exec_id: exec-2024-001
created_at: 2024-01-15T10:30:00Z
---

## Task: Analyze dependency graph

Review `pkg/document/` and identify all direct callers of `As[T]`.
Report each call site with file, line, and whether the caller
validates the error return value.
```

## Current Status

Phase 1 (Skills Document Engine) is complete. Phases 2-5 are planned.

| Component | Status | Phase |
|-----------|--------|-------|
| `pkg/protocol` — 24 message type constants + registry | complete | 1 |
| `pkg/document` — parser, validator, builder, lifecycle | complete | 1 |
| `cmd/aga2aga` — validate / create / inspect CLI | complete | 1 |
| `pkg/transport` — Transport interface stub | stub | 1 |
| `pkg/identity` — Identity + Signer interface stub | stub | 1 |
| `pkg/negotiation` — NegotiationState stub | stub | 1 |
| `cmd/gateway` — MCP Gateway binary | planned | 2 |
| Redis Streams transport implementation | planned | 2 |
| Ed25519 document signing | planned | 3 |
| Negotiation state machine | planned | 4 |
| Gossip P2P transport | planned | 5 |

See [docs/ROADMAP.md](docs/ROADMAP.md) for detailed phase plans.

## Quick Start

**Prerequisites:** Go 1.24+

```bash
git clone https://github.com/valpere/aga2aga.git
cd aga2aga
go build ./...
go test ./...
```

Install the `aga2aga` CLI:

```bash
go install github.com/valpere/aga2aga/cmd/aga2aga@latest
```

### Validate a document

```bash
aga2aga validate path/to/document.md
# valid_genome.md: OK

aga2aga validate --strict path/to/document.md
# --strict promotes semantic warnings to fatal errors
```

### Create a document

```bash
# Create a task request
aga2aga create task.request \
  --from orchestrator \
  --to agent-alpha \
  --exec-id exec-001 \
  --field "task=Analyze the dependency graph" \
  --out task.md

# Create an agent genome
aga2aga create agent.genome \
  --from meta-evolver \
  --field "kind=worker" \
  --field "version=1" \
  --out genome.md
```

### Inspect a document

```bash
aga2aga inspect genome.md
# type:     agent.genome
# id:       01HN7K2P3Q4R5S6T7U8V9W0X1Y
# from:     meta-evolver
# version:  v1
# created:  2024-01-15T10:30:00Z

aga2aga inspect genome.md --format json
```

## Package Structure

```
cmd/gateway/      MCP Gateway binary (Phase 2)
cmd/aga2aga/          CLI tool: validate, create, inspect

pkg/document/     Skills Document engine (Phase 1, complete)
                    - Parse, Serialize, SplitFrontMatter
                    - 3-layer Validator (structural / JSON Schema / semantic)
                    - Fluent Builder with sticky-error guard
                    - Lifecycle state machine: 11 states, ValidTransition
                    - Typed structs for all 24 message types

pkg/protocol/     Message type constants and registry (Phase 1, complete)
                    - 24 MessageType constants across 3 groups
                    - TypeMeta: required fields + schema ref
                    - Lookup, Registered, BaseEnvelopeFields

pkg/transport/    Transport interface stub (Phase 1, implemented Phase 2)
                    - Publish, Subscribe, Ack, Close

pkg/identity/     Ed25519 identity stub (Phase 1, implemented Phase 3)
                    - Identity struct, Signer interface

pkg/negotiation/  Negotiation engine stub (Phase 1, implemented Phase 4)
                    - NegotiationState constants, NegotiationTransition stub

internal/gateway/ MCP Gateway internals (Phase 2)

tests/testdata/   Test fixture documents
schemas/          Standalone JSON Schema 2020-12 reference copy
docs/             Extended documentation
```

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — detailed architecture: data flow, package dependency graph, API surface, security model
- [docs/API.md](docs/API.md) — full API reference: all exported types, functions, and CLI flags with examples
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) — TDD workflow, commit conventions, PR process, DO_NOT_TOUCH rules
- [docs/SECURITY.md](docs/SECURITY.md) — security invariants, trust boundaries, CWE references
- [docs/ROADMAP.md](docs/ROADMAP.md) — phase plans and non-goals

## License

MIT. See [LICENSE](LICENSE).
