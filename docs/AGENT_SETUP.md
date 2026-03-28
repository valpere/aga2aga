# Agent Setup Guide

How to configure an AI agent (Claude Code, Codex CLI, Gemini CLI) to connect to the aga2aga MCP Gateway, exchange tasks, and send free-form messages to other agents.

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Configuring `.mcp.json`](#configuring-mcpjson)
   - [stdio transport (recommended)](#stdio-transport-recommended)
   - [HTTP transport](#http-transport)
4. [MCP Tools](#mcp-tools)
   - [get_task](#get_task)
   - [complete_task](#complete_task)
   - [fail_task](#fail_task)
   - [heartbeat](#heartbeat)
   - [send_message](#send_message)
   - [receive_message](#receive_message)
5. [Conversation Loop](#conversation-loop)
6. [Agent ID Rules](#agent-id-rules)
7. [Task Body Format](#task-body-format)
8. [Error Reference](#error-reference)
9. [Full Examples](#full-examples)

---

## Overview

The gateway bridges AI agents to a Redis Streams orchestration system. From the agent's perspective it is a standard MCP server that exposes six tools:

```
get_task         → pull the next task assigned to this agent
complete_task    → report success + deliver a result
fail_task        → report failure with an error message
heartbeat        → health check (returns ok immediately)
send_message     → send a free-form message to another agent
receive_message  → fetch the next message from this agent's inbox
```

The agent polls `get_task`, does its work, then calls `complete_task` or `fail_task`. The gateway handles all Redis Streams bookkeeping transparently. Agents can also exchange free-form messages (advice, warnings, suggestions) at any time using `send_message` and `receive_message` — independently of the task lifecycle.

---

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| aga2aga gateway running | See [RUNNING.md](RUNNING.md) for setup |
| Agent registered in Admin UI | Create an API key with `operator` role |
| Policy entry allowing the agent | Admin UI → Policies → allow `<your-agent-id>` → `orchestrator` |

---

## Configuring `.mcp.json`

Place `.mcp.json` in the project root (or the directory from which you launch the agent).

### stdio transport (recommended)

The gateway process is launched as a child process and communicates via stdin/stdout. No network port is needed.

```json
{
  "mcpServers": {
    "aga2aga": {
      "type": "stdio",
      "command": "/path/to/aga2aga-gateway",
      "args": [
        "--mcp-transport", "stdio",
        "--redis-addr",    "localhost:6379",
        "--agent-id",      "mcp-gateway",
        "--policy-mode",   "remote",
        "--admin-url",     "http://localhost:8087",
        "--task-read-timeout", "5s"
      ],
      "env": {
        "ADMIN_API_KEY": "<operator-api-key>"
      }
    }
  }
}
```

**Key fields:**

| Field | Description |
|-------|-------------|
| `command` | Absolute path to the `aga2aga-gateway` binary |
| `--redis-addr` | Redis address (`host:port`, default `localhost:6379`) |
| `--agent-id` | Identity of the gateway itself (used in policy checks, not the connecting agent's ID) |
| `--policy-mode` | `embedded` (read local SQLite) or `remote` (call Admin HTTP API) |
| `--admin-url` | Admin server base URL — required when `policy-mode=remote` |
| `ADMIN_API_KEY` | Bearer token for Admin API; **always pass via env, never via `--admin-api-key` flag** (the flag is visible in `/proc/<pid>/cmdline`) |
| `--task-read-timeout` | How long `get_task` waits for a delivery before returning empty (default `5s`) |
| `--pending-ttl` | How long a task ID is retained after delivery (default `1h`) |

### HTTP transport

Use HTTP when the gateway is a separate long-lived process (e.g. in Docker Compose).

```json
{
  "mcpServers": {
    "aga2aga": {
      "type": "http",
      "url": "http://localhost:3000/mcp"
    }
  }
}
```

Start the gateway with:

```bash
ADMIN_API_KEY=<key> aga2aga-gateway \
  --mcp-transport http \
  --addr          :3000 \
  --redis-addr    localhost:6379 \
  --policy-mode   remote \
  --admin-url     http://localhost:8087
```

> **Note:** The HTTP transport uses MCP Streamable-HTTP (SSE for server-to-client). Any HTTP client that supports Server-Sent Events works.

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

**Output:**

```json
{
  "status": "ok"
}
```

---

### heartbeat

Verifies the gateway is alive. No parameters needed.

**Input:**

```json
{
  "agent": "my-agent-01"
}
```

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
| `gateway: agent "..." not allowed` | No policy permits this agent | Admin UI → Policies → add `allow <agent-id> → orchestrator` |
| `gateway: policy check: ...` | Admin API unreachable or key invalid | Check `ADMIN_API_KEY` and `--admin-url` |
| `gateway: unknown task_id "..."` | `task_id` expired or already acknowledged | Do not reuse `task_id` after calling `complete_task` / `fail_task` |
| `gateway: result body exceeds maximum size` | Result > 65 536 bytes | Truncate or summarise the result |
| `gateway: error body exceeds maximum size` | Error message > 65 536 bytes | Shorten the error string |
| `gateway: message body exceeds maximum size` | `send_message` body > 65 536 bytes | Shorten the message |
| `gateway: publish message: ...` | Redis error while sending message | Check Redis connectivity |
| `gateway: subscribe: ...` | Redis error while subscribing to message stream | Check Redis connectivity |
| `gateway: ack message: ...` | Redis error while acknowledging a received message | Check Redis connectivity; message may be redelivered |

---

## Full Examples

### Claude Code (stdio)

`.mcp.json` in the project root where Claude Code is launched:

```json
{
  "mcpServers": {
    "aga2aga": {
      "type": "stdio",
      "command": "/usr/local/bin/aga2aga-gateway",
      "args": [
        "--mcp-transport",       "stdio",
        "--redis-addr",          "localhost:6379",
        "--policy-mode",         "remote",
        "--admin-url",           "http://localhost:8087",
        "--task-read-timeout",   "10s",
        "--agent-id",            "mcp-gateway"
      ],
      "env": {
        "ADMIN_API_KEY": "aga2_op_abc123..."
      }
    }
  }
}
```

Claude Code discovers the tools automatically on startup. Use them in conversation:

```
> Use get_task with agent "claude-code-01" to check for work.
```

### Codex CLI (stdio)

Same `.mcp.json` format. Codex CLI reads `.mcp.json` from the working directory.

### Programmatic HTTP client (Python example)

```python
import httpx, json, time

GATEWAY = "http://localhost:3000/mcp"
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
