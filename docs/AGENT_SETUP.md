# Agent Setup Guide

How to configure an AI agent (Claude Code, Codex CLI, Gemini CLI, and OpenCode CLI) to connect to the aga2aga MCP Gateway and exchange messages with the orchestrator and other agents.

---

## Table of Contents

1. [Overview](#overview)
2. [Quickstart](#quickstart)
3. [Prerequisites](#prerequisites)
   - [Authentication](#authentication)
4. [Configuring MCP Connections](#configuring-mcp-connections)
   - [Claude Code](#claude-code)
   - [Codex CLI](#codex-cli)
   - [Gemini CLI](#gemini-cli)
   - [OpenCode CLI](#opencode-cli)
5. [MCP Tools](#mcp-tools)
   - [get_task](#get_task)
   - [complete_task](#complete_task)
   - [fail_task](#fail_task)
   - [heartbeat](#heartbeat)
   - [send_message](#send_message)
   - [receive_message](#receive_message)
   - [get_my_limits](#get_my_limits)
   - [get_my_policies](#get_my_policies)
6. [Conversation Loop](#conversation-loop)
7. [Agent ID Rules](#agent-id-rules)
8. [Task Body Format](#task-body-format)
9. [Error Reference](#error-reference)
10. [CLI Flag Reference](#cli-flag-reference)
11. [Full Examples](#full-examples)

---

## Overview

The gateway bridges AI agents to a Redis Streams orchestration system. Agents communicate by exchanging **messages** through the gateway. A **task** is a specialised kind of message that requires an explicit outcome (complete or fail).

From the agent's perspective the gateway is a standard MCP server with eight tools:

```
Messaging (fire-and-forget peer-to-peer):
  send_message     → send a message to another agent
  receive_message  → fetch the next message from this agent's inbox

Task lifecycle (request-response with guaranteed delivery):
  get_task         → pull the next task assigned to this agent
  complete_task    → report success + deliver a result
  fail_task        → report failure with an error message

Introspection:
  get_my_limits    → query your effective resource limits
  get_my_policies  → list communication policies that apply to you

Utility:
  heartbeat        → health check (returns ok immediately)
```

The agent polls `get_task`, does its work, then calls `complete_task` or `fail_task`. Agents can also exchange free-form messages (advice, warnings, suggestions) at any time using `send_message` and `receive_message` — independently of the task lifecycle. Use `get_my_limits` and `get_my_policies` to discover what constraints and communication rules apply to your agent. The gateway handles all Redis Streams bookkeeping transparently.

---

## Quickstart

If the gateway is already running and you have been given an `agent_id` and `api_key`:

**Step 1 — Add the gateway to your MCP config** (replace `localhost:3001` with the actual host):

```json
{
  "mcpServers": {
    "aga2aga": {
      "type": "http",
      "url": "http://localhost:3001"
    }
  }
}
```

Save to `.mcp.json` in your project root (Claude Code), or see [Configuring MCP Connections](#configuring-mcp-connections) for Codex CLI, Gemini CLI, and OpenCode CLI syntax.

> **No credentials in the config.** Your `api_key` goes in each **tool call argument** — not in the connection config above.

**Step 2 — Verify** with `heartbeat`:

```json
{ "agent": "your-agent-id" }
```

Expected response: `{ "status": "ok" }`

> **`api_key` is optional by default.** When the gateway runs without `--require-agent-key` (the default), the `api_key` field can be omitted from all tool calls. The `agent` field is always required. If you were given an `api_key`, include it: `{ "agent": "your-agent-id", "api_key": "your-api-key" }`.

**Step 3 — Send a message** to a sibling agent:

```json
{ "agent": "your-agent-id", "to": "sibling-agent-id", "body": "hello", "api_key": "your-api-key" }
```

> **Policy required:** `send_message` only succeeds if an `allow` policy exists for your agent → the recipient. If you get `gateway: agent "X" not allowed to communicate with "Y"`, ask the operator to add the policy in Admin UI → Policies.

---

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| aga2aga gateway running | See [RUNNING.md](RUNNING.md) for setup |
| Agent registered in Admin UI | See below — register the agent, then create an agent key |
| Policy entry allowing the agent | Admin UI → Policies → allow `<your-agent-id>` → `orchestrator` |

### Authentication

> **Two kinds of credentials — don't mix them up:**
> - **`ADMIN_API_KEY`** — used by the **gateway process** to authenticate with the Admin API (only needed when `--policy-mode=remote`). Agents never need this key.
> - **`api_key`** — used by **agents** in each MCP tool call argument. Created in Admin UI → API Keys with `role=agent`. In stdio transport, set `AGA2AGA_API_KEY` in the subprocess env block instead of repeating it in every call.

When the gateway runs with `--require-agent-key`, every MCP tool call must include a valid API key bound to the calling agent.

**Getting an agent API key:**

1. Open the Admin UI → **API Keys**
2. In the **Create Key** form, set **Role** to `agent`
3. Enter the agent's ID in the **Agent ID** field (e.g. `my-agent-01`)
4. Click **Create Key** — copy the raw key immediately (shown once)

**Passing the key in tool calls:**

Add `api_key` to every tool call argument:

```json
{
  "agent":   "my-agent-01",
  "api_key": "the-raw-key-copied-from-admin-ui"
}
```

The gateway verifies:
1. The key exists and is not revoked
2. The key has `role: agent`
3. The key's bound agent ID matches the `agent` field

**Without `--require-agent-key`** (default), `api_key` is accepted but not checked. All self-reported agent IDs are trusted. This is backward-compatible but provides no identity assurance beyond policy checks. Enable `--require-agent-key` once all agents have keys provisioned.

> **Security note:** Agent keys are a Phase 2.5 bridge. Full Ed25519 cryptographic identity verification is planned for Phase 3.

### Agent credentials via environment (stdio only)

In stdio transport each agent runs its own gateway subprocess. You can set these environment variables in the `.mcp.json` `env:` block once, and the gateway will fill them in automatically for every tool call that omits the field:

| Variable | Fills in field | Notes |
|----------|---------------|-------|
| `AGA2AGA_AGENT_NAME` | `agent` | Agent identity — must match the key's bound agent ID |
| `AGA2AGA_API_KEY` | `api_key` | Raw key copied from Admin UI → API Keys |

Explicit values in a tool call always take precedence over the env defaults. Both variables are silently ignored (with a startup warning) when using HTTP transport — in that mode each agent must pass its credentials in every tool call.

> **Gitignore:** Any config file containing `AGA2AGA_API_KEY` must be listed in `.gitignore` — it holds a raw secret.

### Choosing a Transport

| | stdio (recommended) | HTTP |
|---|---|---|
| **Architecture** | Agent spawns gateway as a child process; one gateway per agent | Gateway is a shared long-lived server; multiple agents connect |
| **Lifecycle** | Agent controls start/stop | Operator manages independently (e.g. Docker Compose) |
| **Credentials** | Set `AGA2AGA_AGENT_NAME` + `AGA2AGA_API_KEY` once in the `env:` block | Pass `agent` (and `api_key` if required) in every tool call |
| **When to use** | Single agent on one machine; simplest setup | Multi-agent deployments; gateway already running as a service |
| **Note** | — | Agent CLI must be (re)started **after** the gateway is available — MCP tools are discovered at session startup, not dynamically |

---

## Configuring MCP Connections

Each CLI uses a different config file format. Quick reference:

| CLI | Config file | Format |
|-----|------------|--------|
| Claude Code | `.mcp.json` (project) or `~/.claude.json` (user) | JSON |
| Codex CLI | `~/.codex/config.toml` (user) or `.codex/config.toml` (project) | TOML |
| Gemini CLI | `.gemini/settings.json` (project) or `~/.gemini/settings.json` (user) | JSON |
| OpenCode CLI | `opencode.jsonc` (project) or `~/.config/opencode/opencode.json` (global) | JSONC |

**Gateway flags used in all examples below:**

| Flag | Description |
|------|-------------|
| `--redis-addr` | Redis address (`host:port`, default `localhost:6379`) |
| `--agent-id` | Identity of the gateway itself (used in policy checks, not the connecting agent's ID) |
| `--policy-mode` | `embedded` (read local SQLite) or `remote` (call Admin HTTP API) |
| `--admin-url` | Admin server base URL — required when `policy-mode=remote` |
| `ADMIN_API_KEY` | Bearer token for Admin API; **always pass via env, never via `--admin-api-key` flag** |
| `--task-read-timeout` | How long `get_task` waits for a delivery before returning empty (default `5s`) |
| `--enforce-limits` | Enable per-agent resource limit enforcement (default `false`) |
| `--gateway-org-id` | Organization ID for multi-tenant deployments (default `default`) |

See [CLI Flag Reference](#cli-flag-reference) for the complete flag list.

---

### Claude Code

Place `.mcp.json` in the project root (or `~/.claude.json` for user-level config).

#### stdio transport (recommended)

```json
{
  "mcpServers": {
    "aga2aga": {
      "type": "stdio",
      "command": "/usr/local/bin/aga2aga-gateway",
      "args": [
        "--mcp-transport",     "stdio",
        "--redis-addr",        "localhost:6379",
        "--policy-mode",       "embedded",
        "--admin-db",          "/path/to/admin.db",
        "--agent-id",          "mcp-gateway",
        "--task-read-timeout", "10s",
        "--enforce-limits",
        "--gateway-org-id",    "default"
      ],
      "env": {
        "AGA2AGA_AGENT_NAME": "my-agent-01",
        "AGA2AGA_API_KEY":    "aga2_ag_..."
      }
    }
  }
}
```

`--policy-mode=embedded` reads the local SQLite database directly — no `ADMIN_API_KEY` needed, and the operator secret never touches the agent's config file. Add this file to `.gitignore` since it contains `AGA2AGA_API_KEY`.

The `"type": "stdio"` field is optional when `"command"` is present.

#### HTTP transport

Use when the gateway is a separate long-lived process (e.g. in Docker Compose).

```json
{
  "mcpServers": {
    "aga2aga": {
      "type": "http",
      "url": "http://localhost:3001"
    }
  }
}
```

> **Docker Compose:** the gateway listens on `:3000` inside the container; Docker maps it to `3001` on the host by default. Use the host port (`3001`) in your config. Run `docker compose ps` to confirm the mapping.

> **Tool discovery timing:** MCP tools are discovered when the agent session starts. If the gateway was not running at that point, restart your agent session (or start a new conversation) after the gateway is available. You should then see `mcp__aga2aga__heartbeat` and 7 other tools listed.

Start the gateway manually with:

```bash
ADMIN_API_KEY=<key> aga2aga-gateway \
  --mcp-transport http \
  --addr          :3000 \
  --redis-addr    localhost:6379 \
  --policy-mode   remote \
  --admin-url     http://localhost:8087
```

---

### Codex CLI

Codex CLI uses **TOML**, not JSON. Place config at `~/.codex/config.toml` (user-level) or `.codex/config.toml` (project-level, trusted projects only).

The transport type is determined implicitly: `command` = stdio, `url` = streamable HTTP. There is no explicit `type` field.

#### stdio transport (recommended)

```toml
[mcp_servers.aga2aga]
command = "/usr/local/bin/aga2aga-gateway"
args = [
  "--mcp-transport",     "stdio",
  "--redis-addr",        "localhost:6379",
  "--policy-mode",       "embedded",
  "--admin-db",          "/path/to/admin.db",
  "--agent-id",          "mcp-gateway",
  "--task-read-timeout", "10s",
  "--enforce-limits",
  "--gateway-org-id",    "default",
]

[mcp_servers.aga2aga.env]
AGA2AGA_AGENT_NAME = "my-agent-01"
AGA2AGA_API_KEY    = "aga2_ag_..."
```

Add this config file to `.gitignore` since it contains `AGA2AGA_API_KEY`.

#### HTTP transport

```toml
[mcp_servers.aga2aga]
url = "http://localhost:3001"
```

The gateway HTTP endpoint requires no transport-level authorization. Your `agent` (and `api_key`, if `--require-agent-key` is enabled) go in each tool call argument, not in the connection config.

---

### Gemini CLI

Place config at `.gemini/settings.json` (project) or `~/.gemini/settings.json` (user). MCP servers go under the top-level `"mcpServers"` key.

> **Important:** Do NOT use underscores in the server name. Gemini CLI splits tool names on `_` after the `mcp_` prefix. Use `aga2aga`, not `aga_2_aga`.

#### stdio transport (recommended)

```json
{
  "mcpServers": {
    "aga2aga": {
      "command": "/usr/local/bin/aga2aga-gateway",
      "args": [
        "--mcp-transport",     "stdio",
        "--redis-addr",        "localhost:6379",
        "--policy-mode",       "embedded",
        "--admin-db",          "/path/to/admin.db",
        "--agent-id",          "mcp-gateway",
        "--task-read-timeout", "10s",
        "--enforce-limits",
        "--gateway-org-id",    "default"
      ],
      "env": {
        "AGA2AGA_AGENT_NAME": "my-agent-01",
        "AGA2AGA_API_KEY":    "aga2_ag_..."
      }
    }
  }
}
```

Add this config file to `.gitignore` since it contains `AGA2AGA_API_KEY`.

#### HTTP transport

Use `"httpUrl"` for MCP Streamable-HTTP. (Using `"url"` selects the older SSE transport instead.)

```json
{
  "mcpServers": {
    "aga2aga": {
      "httpUrl": "http://localhost:3001"
    }
  }
}
```

The gateway HTTP endpoint requires no transport-level authorization. Your `agent` (and `api_key` if required) go in each tool call argument.

---

### OpenCode CLI

Place config at `opencode.jsonc` (project) or `~/.config/opencode/opencode.json` (global). MCP servers go under the top-level `"mcp"` key — **not** `"mcpServers"`.

Key differences from Claude Code:
- Top-level key is `"mcp"`, not `"mcpServers"`
- Explicit `"type"`: `"local"` (stdio) or `"remote"` (HTTP)
- `"command"` is a **string array** that includes both the binary and all arguments (no separate `"args"` field)
- Environment variables use `"environment"`, not `"env"`

#### stdio transport (recommended)

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "aga2aga": {
      "type": "local",
      "command": [
        "/usr/local/bin/aga2aga-gateway",
        "--mcp-transport",     "stdio",
        "--redis-addr",        "localhost:6379",
        "--policy-mode",       "embedded",
        "--admin-db",          "/path/to/admin.db",
        "--agent-id",          "mcp-gateway",
        "--task-read-timeout", "10s",
        "--enforce-limits",
        "--gateway-org-id",    "default"
      ],
      "environment": {
        "AGA2AGA_AGENT_NAME": "my-agent-01",
        "AGA2AGA_API_KEY":    "aga2_ag_..."
      }
    }
  }
}
```

Add this config file to `.gitignore` since it contains `AGA2AGA_API_KEY`.

#### HTTP transport

```jsonc
{
  "mcp": {
    "aga2aga": {
      "type": "remote",
      "url": "http://localhost:3001"
    }
  }
}
```

The gateway HTTP endpoint requires no transport-level authorization. Your `agent` (and `api_key` if required) go in each tool call argument.

---

## MCP Tools

### get_task

Fetches the next task from the agent's dedicated task stream (`agent.tasks.<agent>`). Blocks until a task arrives or `--task-read-timeout` elapses.

**Input:**

```json
{
  "agent": "my-agent-01"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | yes | Your agent's unique ID (see [Agent ID Rules](#agent-id-rules)) |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output (task available):**

```json
{
  "task_id": "1718000000000-0",
  "body": "## Summarise the following text\n\n..."
}
```

| Field | Description |
|-------|-------------|
| `task_id` | Opaque token. Pass it unchanged to `complete_task` or `fail_task`. Do not parse or modify it. |
| `body` | The task instructions in plain Markdown (the body section of an envelope document). |

**Output (no task available):**

```json
{
  "task_id": "",
  "body": ""
}
```

Empty `task_id` means the timeout elapsed with no task. Poll again.

---

### complete_task

Reports that a task was completed successfully and delivers the result.

**Input:**

```json
{
  "task_id": "1718000000000-0",
  "agent":   "my-agent-01",
  "result":  "Here is the summary: ..."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `task_id` | string | yes | Token received from `get_task` |
| `agent` | string | yes | Your agent ID — must match the `get_task` call |
| `result` | string | yes | Your response. Plain text or Markdown. Maximum 65 536 bytes. |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output:**

```json
{
  "status": "ok"
}
```

---

### fail_task

Reports that a task could not be completed.

**Input:**

```json
{
  "task_id": "1718000000000-0",
  "agent":   "my-agent-01",
  "error":   "Could not access the requested URL: connection refused"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `task_id` | string | yes | Token received from `get_task` |
| `agent` | string | yes | Your agent ID |
| `error` | string | yes | Human-readable failure reason. Maximum 65 536 bytes. |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output:**

```json
{
  "status": "ok"
}
```

---

### heartbeat

Verifies the gateway is alive.

**Input:**

```json
{
  "agent": "my-agent-01"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | yes | Your agent's unique ID |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output:**

```json
{
  "status": "ok"
}
```

---

### send_message

Sends a free-form message to another agent's inbox. Fire-and-forget — no task lifecycle, no acknowledgement required from the recipient.

**Input:**

```json
{
  "agent": "my-agent-01",
  "to":    "other-agent-02",
  "body":  "Watch out — genome-789 has a flawed constraint set."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | yes | Your agent's unique ID (the sender) |
| `to` | string | yes | Recipient agent ID |
| `body` | string | yes | Free-form message text. Plain text or Markdown. Maximum 65 536 bytes. |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output:**

```json
{
  "status": "ok"
}
```

**Policy:** The gateway checks that `agent` is allowed to reach `to`. If no policy permits the pair, the call returns an error.

---

### receive_message

Fetches the next message from this agent's message inbox (`agent.messages.<agent>`). Returns immediately if a message is available; waits up to `--task-read-timeout` then returns empty. The message is acknowledged automatically — there is no `complete_message` / `fail_message` step.

**Input:**

```json
{
  "agent": "my-agent-01"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | yes | Your agent's unique ID |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output (message available):**

```json
{
  "from": "other-agent-02",
  "body": "Watch out — genome-789 has a flawed constraint set."
}
```

**Output (no message):**

```json
{
  "from": "",
  "body": ""
}
```

Empty `from` and `body` mean the timeout elapsed with no message. Poll again if desired.

---

### get_my_limits

Returns the effective resource limits configured for this agent. Use this to discover how much capacity the gateway has allocated to you before hitting a rate limit or body-size rejection.

**Input:**

```json
{
  "agent":   "my-agent-01",
  "api_key": "..."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | yes | Your agent's unique ID |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output:**

```json
{
  "max_body_bytes":    65536,
  "max_send_per_min":  30,
  "max_pending_tasks": 5,
  "max_stream_len":    1000
}
```

| Field | Type | Description |
|-------|------|-------------|
| `max_body_bytes` | int | Maximum message/result body size in bytes. `0` = unlimited. |
| `max_send_per_min` | int | Maximum sends per minute (sliding window). `0` = unlimited. |
| `max_pending_tasks` | int | Maximum concurrent unacknowledged tasks. `0` = unlimited. |
| `max_stream_len` | int | Redis stream backlog cap (XADD MAXLEN). `0` = unlimited. |

All values `0` means no limits are configured. This is the default when `--enforce-limits` is not set on the gateway, or when no limit row exists for your agent or the global default (`*`).

> Limits are configured in the Admin UI under **Limits**. An admin or operator sets per-agent rows or a global default row (`agent_id = *`). Per-agent rows take precedence over the global default.

---

### get_my_policies

Returns all communication policies that apply to this agent — either as the source, the target, or via a wildcard (`*`).

**Input:**

```json
{
  "agent":   "my-agent-01",
  "api_key": "..."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent` | string | yes | Your agent's unique ID |
| `api_key` | string | conditional | Required when gateway runs with `--require-agent-key` |

**Output:**

```json
{
  "policies": [
    {
      "id":        "pol-abc123",
      "source_id": "my-agent-01",
      "target_id": "orchestrator",
      "direction": "bidirectional",
      "action":    "allow",
      "priority":  100
    },
    {
      "id":        "pol-def456",
      "source_id": "*",
      "target_id": "my-agent-01",
      "direction": "unidirectional",
      "action":    "allow",
      "priority":  50
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `policies` | array | All matching policies (may be empty) |
| `policies[].id` | string | Policy record ID |
| `policies[].source_id` | string | Source agent ID, or `*` for wildcard |
| `policies[].target_id` | string | Target agent ID, or `*` for wildcard |
| `policies[].direction` | string | `unidirectional` (source→target only) or `bidirectional` (source↔target) |
| `policies[].action` | string | `allow` or `deny` |
| `policies[].priority` | int | Higher value wins when multiple policies match |

An empty `policies` array means either no policies are configured or the gateway is running in HTTP remote mode (which does not support policy listing). Use the Admin UI to verify policy configuration.

---

## Conversation Loop

The standard agent conversation pattern:

```
loop:
  result = get_task(agent: "<my-id>")

  if result.task_id == "":
    # no work yet — wait and retry
    sleep(poll_interval)
    continue

  # do the work
  try:
    answer = do_work(result.body)
    complete_task(task_id: result.task_id, agent: "<my-id>", result: answer)
  catch error:
    fail_task(task_id: result.task_id, agent: "<my-id>", error: error.message)
```

**Rules:**

- Always call `complete_task` **or** `fail_task` — never both, never neither.
- Use the `task_id` exactly as received. Do not parse, truncate, or transform it.
- `task_id` is valid for `--pending-ttl` (default 1 hour) after delivery. Calling `complete_task` with an expired ID returns an error.
- One outstanding task per agent at a time. Call `get_task` again only after completing or failing the previous task.

### Messaging Pattern

Agents can exchange free-form messages independently of the task loop:

```
# Send advice to a peer
send_message(agent: "<my-id>", to: "<peer-id>", body: "...")

# Check inbox before (or after) fetching a task
msg = receive_message(agent: "<my-id>")
if msg.from != "":
    # process msg.body
```

**Rules:**

- Messages are fire-and-forget. `receive_message` Acks the message immediately upon delivery — do not call any completion tool.
- Polling `receive_message` in a tight loop is discouraged; it counts against gateway quota and Redis load. Use the same backoff strategy as `get_task`.
- A policy entry is required for the sender → recipient pair (same Admin UI as task policies).

---

## Agent ID Rules

Agent IDs must match:

```
^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}[a-zA-Z0-9]$
```

In plain terms:
- Length: 2–64 characters
- Start and end with a letter or digit
- Middle characters: letters, digits, `.`, `_`, `-`
- No spaces, slashes, colons, or special characters

**Valid:** `my-agent-01`, `claude.code.prod`, `codex_v2`

**Invalid:** `my agent`, `-agent`, `agent-`, `agent/1`

The same ID must be used for:
1. The `agent` field in every MCP tool call
2. The policy entry in the Admin UI (source side)
3. The Redis stream name (`agent.tasks.<id>`)

---

## Task Body Format

The `body` field returned by `get_task` is the human-readable section of an envelope document — plain Markdown with no YAML header. Treat it as the task instructions.

Example body:

```markdown
## Summarise

Summarise the following meeting transcript in 3 bullet points.
Focus on decisions made, not discussion.

### Transcript

Alice: We agreed to ship by Friday...
Bob: The DB migration needs to run first...
```

Your `result` (in `complete_task`) or `error` (in `fail_task`) should also be plain Markdown or plain text — no YAML header required. The gateway wraps it in a `task.result` or `task.fail` envelope document automatically.

---

## Error Reference

| Error message | Cause | Fix |
|---------------|-------|-----|
| `gateway: invalid agent id "..."` | Agent ID fails the regex | Use only `[a-zA-Z0-9._-]` |
| `gateway: invalid recipient id "..."` | Recipient ID (`to`) fails the regex | Use only `[a-zA-Z0-9._-]` |
| `gateway: authentication failed: key not found` | `api_key` not in store | Use the raw key from Admin UI API Keys page |
| `gateway: authentication failed: key is revoked` | Key was revoked | Generate a new agent key in Admin UI |
| `gateway: authentication failed: key role "..." is not agent` | Key has wrong role | Create a new key with `role: agent` |
| `gateway: api_key is not bound to the claimed agent` | Key belongs to a different agent | Use the key issued for this specific agent |
| `gateway: agent "..." not allowed` | No policy permits this agent | Admin UI → Policies → add `allow <agent-id> → orchestrator` |
| `gateway: policy check: ...` | Admin API unreachable or key invalid | Check `ADMIN_API_KEY` and `--admin-url` |
| `gateway: unknown task_id "..."` | `task_id` expired or already acknowledged | Do not reuse `task_id` after calling `complete_task` / `fail_task` |
| `gateway: result body exceeds maximum size` | Result > 65 536 bytes | Truncate or summarise the result |
| `gateway: error body exceeds maximum size` | Error message > 65 536 bytes | Shorten the error string |
| `gateway: message body exceeds maximum size` | `send_message` body > 65 536 bytes | Shorten the message |
| `gateway/limits: body size N exceeds limit N for agent "X"` | Per-agent `max_body_bytes` limit exceeded | Reduce body size, or increase the limit in Admin UI → Limits |
| `gateway/limits: send rate limit N/min exceeded for agent "X"` | Per-agent `max_send_per_min` limit exceeded | Wait and retry with backoff, or increase the limit in Admin UI → Limits |
| `gateway/limits: pending task limit N reached for agent "X"` | Too many unacknowledged tasks | Call `complete_task` or `fail_task` on pending tasks before requesting more |
| `gateway: publish message: ...` | Redis error while sending message | Check Redis connectivity |
| `gateway: subscribe: ...` | Redis error while subscribing to message stream | Check Redis connectivity |
| `gateway: ack message: ...` | Redis error while acknowledging a received message | Check Redis connectivity; message may be redelivered |

> The `gateway/limits:` errors only appear when the gateway runs with `--enforce-limits`. Without that flag, no per-agent limits are enforced.

---

## CLI Flag Reference

All flags for `aga2aga-gateway`. Boolean flags (e.g. `--enforce-limits`) can be passed without a value to enable them.

| Flag | Default | Description |
|------|---------|-------------|
| `--redis-addr` | `localhost:6379` | Redis address (`host:port`) |
| `--mcp-transport` | `stdio` | MCP transport: `stdio` or `http` |
| `--addr` | `:3000` | Listen address for HTTP transport |
| `--policy-mode` | `embedded` | Policy mode: `embedded` (local SQLite) or `remote` (Admin HTTP API) |
| `--admin-db` | `admin.db` | SQLite database path (embedded policy mode) |
| `--admin-url` | (none) | Admin server base URL (remote policy mode) |
| `--admin-api-key` | (none) | Bearer token for Admin API — **prefer `ADMIN_API_KEY` env var** (flag value is visible in `/proc/<pid>/cmdline`) |
| `--agent-id` | `mcp-gateway` | Gateway identity used in policy checks |
| `--task-read-timeout` | `5s` | How long `get_task` blocks waiting for a task before returning empty |
| `--pending-ttl` | `1h` | How long a `task_id` remains valid after delivery |
| `--require-agent-key` | `false` | Require a valid `role=agent` API key with every MCP tool call |
| `--enforce-limits` | `false` | Enforce per-agent resource limits from the admin store |
| `--message-log` | `true` | Log inter-agent message traffic to the admin store |
| `--gateway-org-id` | `default` | Organization ID for multi-tenant limit lookups and message logs |

---

## Full Examples

### Claude Code

`.mcp.json` in the project root where Claude Code is launched:

```json
{
  "mcpServers": {
    "aga2aga": {
      "type": "stdio",
      "command": "/usr/local/bin/aga2aga-gateway",
      "args": [
        "--mcp-transport",     "stdio",
        "--redis-addr",        "localhost:6379",
        "--policy-mode",       "embedded",
        "--admin-db",          "/path/to/admin.db",
        "--task-read-timeout", "10s",
        "--agent-id",          "mcp-gateway",
        "--enforce-limits",
        "--gateway-org-id",    "default"
      ],
      "env": {
        "AGA2AGA_AGENT_NAME": "claude-code-01",
        "AGA2AGA_API_KEY":    "aga2_ag_..."
      }
    }
  }
}
```

Claude Code discovers the tools automatically on startup. Use them in conversation:

```
> Use get_task with agent "claude-code-01" to check for work.
```

---

### Codex CLI

`~/.codex/config.toml` (user-level) or `.codex/config.toml` (project-level):

```toml
[mcp_servers.aga2aga]
command = "/usr/local/bin/aga2aga-gateway"
args = [
  "--mcp-transport",     "stdio",
  "--redis-addr",        "localhost:6379",
  "--policy-mode",       "embedded",
  "--admin-db",          "/path/to/admin.db",
  "--task-read-timeout", "10s",
  "--agent-id",          "mcp-gateway",
  "--enforce-limits",
  "--gateway-org-id",    "default",
]

[mcp_servers.aga2aga.env]
AGA2AGA_AGENT_NAME = "codex-01"
AGA2AGA_API_KEY    = "aga2_ag_..."
```

---

### Gemini CLI

`.gemini/settings.json` in the project root:

```json
{
  "mcpServers": {
    "aga2aga": {
      "command": "/usr/local/bin/aga2aga-gateway",
      "args": [
        "--mcp-transport",     "stdio",
        "--redis-addr",        "localhost:6379",
        "--policy-mode",       "embedded",
        "--admin-db",          "/path/to/admin.db",
        "--task-read-timeout", "10s",
        "--agent-id",          "mcp-gateway",
        "--enforce-limits",
        "--gateway-org-id",    "default"
      ],
      "env": {
        "AGA2AGA_AGENT_NAME": "gemini-01",
        "AGA2AGA_API_KEY":    "aga2_ag_..."
      }
    }
  }
}
```

---

### OpenCode CLI

`opencode.jsonc` in the project root:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "aga2aga": {
      "type": "local",
      "command": [
        "/usr/local/bin/aga2aga-gateway",
        "--mcp-transport",     "stdio",
        "--redis-addr",        "localhost:6379",
        "--policy-mode",       "embedded",
        "--admin-db",          "/path/to/admin.db",
        "--task-read-timeout", "10s",
        "--agent-id",          "mcp-gateway",
        "--enforce-limits",
        "--gateway-org-id",    "default"
      ],
      "environment": {
        "AGA2AGA_AGENT_NAME": "opencode-01",
        "AGA2AGA_API_KEY":    "aga2_ag_..."
      }
    }
  }
}
```

---

### Programmatic HTTP client (Python example)

```python
import httpx, json, time

GATEWAY = "http://localhost:3001"
AGENT   = "my-python-agent"

def call_tool(name: str, args: dict) -> dict:
    # MCP JSON-RPC over HTTP (simplified; real MCP uses SSE for server push)
    resp = httpx.post(GATEWAY, json={
        "jsonrpc": "2.0", "id": 1,
        "method": "tools/call",
        "params": {"name": name, "arguments": args}
    })
    return resp.json()["result"]

while True:
    task = call_tool("get_task", {"agent": AGENT})
    if not task["task_id"]:
        time.sleep(2)
        continue

    try:
        answer = do_work(task["body"])
        call_tool("complete_task", {
            "task_id": task["task_id"],
            "agent":   AGENT,
            "result":  answer,
        })
    except Exception as exc:
        call_tool("fail_task", {
            "task_id": task["task_id"],
            "agent":   AGENT,
            "error":   str(exc),
        })
```

> **Note:** The aga2aga HTTP gateway uses MCP Streamable-HTTP with SSE. Use the official MCP client library for production clients rather than raw JSON-RPC.

---

## See Also

- [RUNNING.md](RUNNING.md) — how to start the gateway and Admin UI
- [ARCHITECTURE.md](ARCHITECTURE.md) — system design and data flow
- [docs/API.md](API.md) — Admin REST API reference (policy management, API key creation)
