# Running aga2aga

How to build images, start services, and connect an agent to the gateway.

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Docker Engine | 24+ | Container runtime |
| Docker Compose | v2 (plugin, not standalone) | Service orchestration |
| `redis-cli` | any | Quick connectivity check (optional) |

No Go toolchain is needed to run from images. Go 1.25+ is needed only to build from source (see [Building from source](#building-from-source)).

---

## Quick start

```
Redis :6380 → Admin :8087 → Gateway :3001
```

All services run as unprivileged containers bound to `127.0.0.1` only.

### Step 1 — Build images

```bash
# Both images must be built before first launch.
docker build -f Dockerfile       -t aga2aga-gateway .
docker build -f Dockerfile.admin -t aga2aga-admin   .
```

This requires internet access on the first run (pulls `golang:1.25-bookworm`, `gcr.io/distroless/static:nonroot`, and `alpine:3`). Subsequent builds use the layer cache and are fast.

### Step 2 — Start Redis and Admin

```bash
docker compose -f docker-compose.local.yml up -d redis admin
```

Wait ~5 seconds for the Admin healthcheck to pass:

```bash
docker compose -f docker-compose.local.yml ps
# admin should show "(healthy)"
```

### Step 3 — Create an API key

The Admin UI seeds one default user on first boot:

| Field | Value |
|-------|-------|
| URL | http://localhost:8087 |
| Username | `admin` |
| Password | `changeme` |

**Change the password immediately** (Profile → Change Password).

Then:

1. Go to **API Keys** in the nav bar.
2. Click **New API Key**.
3. Enter a name (e.g. `gateway-key`) and set role to **operator**.
4. Copy the key — it is shown **once only** and never stored in plaintext.

### Step 4 — Start the Gateway

```bash
ADMIN_API_KEY=<paste-key-here> \
  docker compose -f docker-compose.local.yml up -d gateway
```

### Step 5 — Verify everything is running

```bash
docker compose -f docker-compose.local.yml ps
```

Expected output:

```
NAME                SERVICE   STATUS              PORTS
aga2aga-redis-1     redis     Up N minutes (healthy)  127.0.0.1:6380->6379/tcp
aga2aga-admin-1     admin     Up N minutes (healthy)  127.0.0.1:8087->8080/tcp
aga2aga-gateway-1   gateway   Up N minutes            127.0.0.1:3001->3000/tcp
```

Spot checks:

```bash
redis-cli -p 6380 ping          # → PONG
curl -s -o /dev/null -w '%{http_code}' http://localhost:8087/
# → 303  (redirect to /login — admin is up)

curl -s -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer <your-key>" \
  "http://localhost:8087/api/v1/evaluate?source=agent-a&target=agent-b"
# → 200  (gateway can authenticate to admin)
```

---

## Stopping and restarting

```bash
# Stop all services (data is preserved in the admin-data volume)
docker compose -f docker-compose.local.yml down

# Restart — supply the same API key
ADMIN_API_KEY=<key> docker compose -f docker-compose.local.yml up -d
```

**The Admin session cookie is invalidated on each restart** (session keys are regenerated). Log in again at http://localhost:8087 after a restart.

The API key itself persists in the SQLite database (`admin-data` Docker volume) across restarts — you do not need to create a new one each time.

---

## Port configuration

Default ports avoid common conflicts (Open WebUI on 8080, local Redis on 6379). Override any port via environment variable or a `.env` file in the project root:

| Variable | Default | Service |
|----------|---------|---------|
| `REDIS_PORT` | `6380` | Redis host port |
| `ADMIN_PORT` | `8087` | Admin UI host port |
| `GATEWAY_PORT` | `3001` | MCP Gateway host port |
| `ADMIN_API_KEY` | *(required)* | Gateway → Admin Bearer token |

Example `.env` file:

```bash
# .env  (gitignored — never commit this file)
ADMIN_API_KEY=2a45ad1c63d55504fc5a070272c11b456dbb97b6ba53dcd81ee4d62664f196a9
REDIS_PORT=6380
ADMIN_PORT=8087
GATEWAY_PORT=3001
```

With a `.env` file, start with:

```bash
docker compose -f docker-compose.local.yml up -d
```

---

## Connecting an agent

The gateway speaks HTTP-based MCP at `http://localhost:3001` (or your `GATEWAY_PORT`).

### Claude Code

Add to `.mcp.json` or `~/.claude/mcp_settings.json`:

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

The gateway exposes four MCP tools:

| Tool | What it does |
|------|-------------|
| `get_task` | Pull the next task from `agent.tasks.<agent>` Redis stream |
| `complete_task` | Publish result to `agent.events.completed` + ACK |
| `fail_task` | Publish failure to `agent.events.failed` + ACK |
| `heartbeat` | Health check — returns `{status: "ok"}` |

### Agent policies

By default the gateway uses **deny-by-default** policy evaluation. Before an agent can call `get_task`, add a policy in the Admin UI:

1. Go to **Policies** → **New Policy**.
2. Set Source to the agent ID (e.g. `my-agent`), Target to `*` (wildcard), Action to `allow`.
3. Save.

Without a matching policy, `get_task` returns a policy-denied error.

---

## Building from source

```bash
# Compile all binaries
go build ./...

# Run tests
go test ./...

# Run integration tests (requires Docker)
go test -tags integration -timeout 120s ./tests/integration/...

# Build both Docker images
docker build -f Dockerfile       -t aga2aga-gateway .
docker build -f Dockerfile.admin -t aga2aga-admin   .
```

---

## Troubleshooting

### Port already in use

```
Bind for 127.0.0.1:8087 failed: address already in use
```

Set a different port via env var or `.env`:

```bash
ADMIN_PORT=8090 docker compose -f docker-compose.local.yml up -d admin
```

### Admin shows "template error" page

Rebuild the admin image — you may have an older build:

```bash
docker build -f Dockerfile.admin -t aga2aga-admin .
docker compose -f docker-compose.local.yml up -d --force-recreate admin
```

### SQLite "unable to open database file: out of memory"

The `admin-data` Docker volume was created with incorrect ownership (root instead of `nobody`). Delete the volume and recreate — **this wipes all admin data including API keys**:

```bash
docker compose -f docker-compose.local.yml down
docker volume rm aga2aga_admin-data
docker compose -f docker-compose.local.yml up -d redis admin
```

Then rebuild an API key (Step 3 above).

### Gateway exits immediately

Check logs:

```bash
docker logs aga2aga-gateway-1
```

Common causes:

- `ADMIN_API_KEY or --admin-api-key is required for --policy-mode=remote` — `ADMIN_API_KEY` was not set or is empty.
- `create HTTP enforcer: …` — `--admin-url` is unreachable. Ensure the admin container is healthy before starting the gateway.

### Admin login fails after restart

Session cookies are signed with keys generated fresh on each start. Old cookies are invalid. Open a private/incognito window or clear cookies for `localhost:8087`.
