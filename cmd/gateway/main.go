// Package main is the entry point for the aga2aga MCP Gateway.
// It parses configuration from flags, wires the Redis transport and policy
// enforcer, creates the gateway, and serves MCP via stdio or HTTP.
//
// Usage:
//
//	aga2aga-gateway [flags]
//
// Flags:
//
//	--redis-addr         Redis address (default: localhost:6379)
//	--mcp-transport      MCP transport: stdio or http (default: stdio)
//	--addr               Listen address for HTTP transport (default: :3000)
//	--policy-mode        Policy mode: embedded or remote (default: embedded)
//	--admin-db           SQLite path for embedded policy mode (default: admin.db)
//	--admin-url          Admin server URL for remote policy mode
//	--admin-api-key      Bearer token for remote policy mode
//	--pending-ttl        PendingMap entry TTL (default: 1h)
//	--agent-id           Gateway identity used in policy checks (default: mcp-gateway)
//	--task-read-timeout   Max wait for a task delivery in get_task (default: 5s)
//	--require-agent-key   Require a valid role=agent API key with every MCP tool call (default: false)
//	--enforce-limits      Enforce per-agent resource limits from the admin store (default: false)
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	goredis "github.com/redis/go-redis/v9"

	iadmin "github.com/valpere/aga2aga/internal/admin"
	"github.com/valpere/aga2aga/internal/gateway"
	redistransport "github.com/valpere/aga2aga/pkg/transport/redis"
)

func main() {
	redisAddr := flag.String("redis-addr", "localhost:6379", "Redis address")
	mcpTransport := flag.String("mcp-transport", "stdio", "MCP transport: stdio or http")
	addr := flag.String("addr", ":3000", "Listen address (HTTP transport only)")
	policyMode := flag.String("policy-mode", "embedded", "Policy mode: embedded or remote")
	adminDB := flag.String("admin-db", "admin.db", "SQLite path (embedded policy mode)")
	adminURL := flag.String("admin-url", "", "Admin server URL (remote policy mode)")
	adminAPIKey := flag.String("admin-api-key", "", "Bearer token for remote policy mode (prefer ADMIN_API_KEY env var)")
	pendingTTL := flag.Duration("pending-ttl", time.Hour, "PendingMap entry TTL")
	agentID := flag.String("agent-id", "mcp-gateway", "Gateway identity used in policy checks")
	taskReadTimeout := flag.Duration("task-read-timeout", 5*time.Second, "Max wait for a task delivery in get_task")
	requireAgentKey := flag.Bool("require-agent-key", false, "Require agents to present a valid role=agent API key with every MCP tool call")
	messageLog    := flag.Bool("message-log", true, "Log inter-agent message traffic to the admin store")
	enforceLimits := flag.Bool("enforce-limits", false, "Enforce per-agent resource limits from the admin store")
	gatewayOrgID  := flag.String("gateway-org-id", "default", "Organization ID used when writing message logs and limit lookups")
	flag.Parse()

	// SECURITY: prefer ADMIN_API_KEY env var over --admin-api-key flag.
	// The flag is visible to other processes via /proc/<pid>/cmdline; the env
	// var is not (on Linux, /proc/<pid>/environ requires the same UID).
	if envKey := os.Getenv("ADMIN_API_KEY"); envKey != "" {
		if *adminAPIKey != "" {
			log.Printf("warning: ADMIN_API_KEY env var overrides --admin-api-key flag")
		}
		*adminAPIKey = envKey
	}
	if *policyMode == "remote" && *adminAPIKey == "" {
		log.Fatal("ADMIN_API_KEY or --admin-api-key is required for --policy-mode=remote")
	}

	// Redis Streams transport. Defer rdb first so it runs last (LIFO); trans
	// is deferred second so it runs first — draining in-flight I/O before
	// closing the underlying client.
	rdb := goredis.NewClient(&goredis.Options{Addr: *redisAddr})
	defer func() { _ = rdb.Close() }()
	trans := redistransport.New(rdb, redistransport.Options{})
	defer func() { _ = trans.Close() }()

	// Policy enforcer.
	enf, closeEnf := mustEnforcer(*policyMode, *adminDB, *adminURL, *adminAPIKey)
	if closeEnf != nil {
		defer closeEnf()
	}

	// Agent authenticator (nil when --require-agent-key is false).
	var auth gateway.AgentAuthenticator
	if *requireAgentKey {
		var closeAuth func()
		auth, closeAuth = mustAuthenticator(*policyMode, *adminDB, *adminURL)
		if closeAuth != nil {
			defer closeAuth()
		}
		log.Printf("agent key authentication enabled")
	} else {
		log.Printf("agent key authentication disabled (--require-agent-key=false); all self-reported agent IDs accepted")
	}

	// Gateway configuration.
	cfg := gateway.DefaultConfig()
	cfg.AgentID = *agentID
	cfg.TaskReadTimeout = *taskReadTimeout
	cfg.PendingTTL = *pendingTTL

	// AGA2AGA_AGENT_NAME / AGA2AGA_API_KEY: stdio-only defaults for tool call fields.
	// In HTTP transport a single gateway process serves multiple agents, so a single
	// identity default would be a security defect — zero both fields and warn.
	cfg.DefaultAgentName = os.Getenv("AGA2AGA_AGENT_NAME")
	cfg.DefaultAgentKey = os.Getenv("AGA2AGA_API_KEY")
	if *mcpTransport == "http" {
		if cfg.DefaultAgentName != "" {
			log.Printf("warning: AGA2AGA_AGENT_NAME is ignored in HTTP transport mode " +
				"(single-identity defaults are unsafe when multiple agents share one gateway)")
		}
		if cfg.DefaultAgentKey != "" {
			// Log presence only — never the value (CWE-532).
			log.Printf("warning: AGA2AGA_API_KEY is set but will NOT be used in HTTP transport mode; " +
				"callers that omit api_key will be rejected when --require-agent-key is enabled")
		}
		cfg.DefaultAgentName = ""
		cfg.DefaultAgentKey = ""
	}

	// Message logger (nil-safe: New treats nil as NoopMessageLogger).
	msgLogger, closeMsgLogger := mustMessageLogger(*messageLog, *policyMode, *adminDB, *adminURL, *adminAPIKey, *gatewayOrgID)
	if closeMsgLogger != nil {
		defer closeMsgLogger()
	}

	limiter, closeLimiter := mustLimitEnforcer(*enforceLimits, *policyMode, *adminDB, *gatewayOrgID)
	if closeLimiter != nil {
		defer closeLimiter()
	}

	gw := gateway.New(trans, enf, auth, msgLogger, limiter, cfg)

	// Root context cancelled on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch *mcpTransport {
	case "stdio":
		log.Printf("aga2aga gateway starting (stdio transport)")
		if err := gw.Run(ctx, &mcpsdk.StdioTransport{}); err != nil && !isContextErr(err) {
			log.Fatalf("gateway: %v", err)
		}

	case "http":
		gw.StartCleanup(ctx)

		httpSrv := &http.Server{
			Addr:        *addr,
			Handler:     gateway.NewMCPHTTPHandler(gw.Server()),
			ReadTimeout: 15 * time.Second,
			// WriteTimeout is intentionally 0: SSE streams used by the MCP
			// streamable-HTTP transport are long-lived responses. A non-zero
			// write deadline would terminate every MCP session after that
			// duration. Per-write deadlines can be set via http.ResponseController
			// inside the handler if needed in the future.
			IdleTimeout: 60 * time.Second,
		}
		go func() {
			log.Printf("aga2aga gateway listening on %s (HTTP transport)", *addr)
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("http: %v", err)
			}
		}()

		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			log.Printf("http shutdown error: %v", err)
		}
		shutCancel()

	default:
		log.Fatalf("unknown --mcp-transport %q (want stdio or http)", *mcpTransport)
	}
}

// mustEnforcer creates a PolicyEnforcer for the given mode.
// Returns a close function if the enforcer holds a resource (e.g. SQLite store);
// close may be nil for resource-free enforcers.
func mustEnforcer(mode, adminDB, adminURL, adminAPIKey string) (gateway.PolicyEnforcer, func()) {
	switch mode {
	case "embedded":
		// SECURITY: resolve symlinks so the opened file is the real path.
		// Consistent with the guard in cmd/enveloper/helpers.go (CWE-22/61).
		resolvedDB, err := filepath.EvalSymlinks(adminDB)
		if err != nil {
			log.Fatalf("resolve admin-db path: %v", err)
		}
		store, err := iadmin.NewSQLiteStore(resolvedDB)
		if err != nil {
			log.Fatalf("open admin store: %v", err)
		}
		return gateway.NewEmbeddedEnforcer(store, "default"), func() { _ = store.Close() }

	case "remote":
		if adminURL == "" {
			log.Fatal("--admin-url is required for remote policy mode")
		}
		enf, err := gateway.NewHTTPEnforcer(adminURL, adminAPIKey)
		if err != nil {
			log.Fatalf("create HTTP enforcer: %v", err)
		}
		return enf, nil

	default:
		log.Fatalf("unknown --policy-mode %q (want embedded or remote)", mode)
		return nil, nil // unreachable
	}
}

// mustAuthenticator creates an AgentAuthenticator for the given policy mode.
// Returns a close function if the authenticator holds a resource (e.g. SQLite
// store); close may be nil for resource-free authenticators.
// In embedded mode a new SQLite connection is opened independently of mustEnforcer.
func mustAuthenticator(mode, adminDB, adminURL string) (gateway.AgentAuthenticator, func()) {
	switch mode {
	case "embedded":
		// SECURITY: resolve symlinks so the opened file is the real path (CWE-22/61).
		resolvedDB, err := filepath.EvalSymlinks(adminDB)
		if err != nil {
			log.Fatalf("resolve admin-db path for authenticator: %v", err)
		}
		store, err := iadmin.NewSQLiteStore(resolvedDB)
		if err != nil {
			log.Fatalf("open admin store for authenticator: %v", err)
		}
		return gateway.NewEmbeddedAuthenticator(store), func() { _ = store.Close() }

	case "remote":
		if adminURL == "" {
			log.Fatal("--admin-url is required for remote policy mode with --require-agent-key")
		}
		auth, err := gateway.NewHTTPAuthenticator(adminURL)
		if err != nil {
			log.Fatalf("create HTTP authenticator: %v", err)
		}
		return auth, nil

	default:
		log.Fatalf("unknown --policy-mode %q (want embedded or remote)", mode)
		return nil, nil // unreachable
	}
}

// mustMessageLogger returns a MessageLogger for the embedded store when enabled,
// or a NoopMessageLogger when --message-log=false. Uses embedded mode only
// (the admin server holds the store; the gateway logs into the same DB file).
// Returns a close function if a new SQLite connection was opened.
func mustMessageLogger(enabled bool, mode, adminDB, adminURL, adminAPIKey, orgID string) (gateway.MessageLogger, func()) {
	if !enabled {
		log.Printf("message logging disabled (--message-log=false)")
		return gateway.NewNoopMessageLogger(), nil
	}
	switch mode {
	case "embedded":
		// SECURITY: resolve symlinks (CWE-22/61).
		resolvedDB, err := filepath.EvalSymlinks(adminDB)
		if err != nil {
			log.Fatalf("resolve admin-db path for message logger: %v", err)
		}
		store, err := iadmin.NewSQLiteStore(resolvedDB)
		if err != nil {
			log.Fatalf("open admin store for message logger: %v", err)
		}
		logger := gateway.NewEmbeddedMessageLogger(store, orgID)
		return logger, func() {
			logger.Close()
			_ = store.Close()
		}
	case "remote":
		if adminURL == "" {
			log.Printf("message logging: --admin-url not set; logging disabled")
			return gateway.NewNoopMessageLogger(), nil
		}
		logger, err := gateway.NewHTTPMessageLogger(adminURL, adminAPIKey)
		if err != nil {
			log.Fatalf("create HTTP message logger: %v", err)
		}
		return logger, nil
	default:
		log.Printf("message logging: unknown policy mode %q; logging disabled", mode)
		return gateway.NewNoopMessageLogger(), nil
	}
}

// mustLimitEnforcer returns a LimitEnforcer for the given mode.
// When disabled (--enforce-limits=false), returns NoopLimitEnforcer.
// In embedded mode, opens a new SQLite connection and returns EmbeddedLimitEnforcer.
// In remote mode, returns NoopLimitEnforcer (HTTPLimitEnforcer is a Phase 3 stub).
func mustLimitEnforcer(enabled bool, mode, adminDB, orgID string) (gateway.LimitEnforcer, func()) {
	if !enabled {
		log.Printf("limit enforcement disabled (--enforce-limits=false)")
		return gateway.NewNoopLimitEnforcer(), nil
	}
	switch mode {
	case "embedded":
		// SECURITY: resolve symlinks (CWE-22/61).
		resolvedDB, err := filepath.EvalSymlinks(adminDB)
		if err != nil {
			log.Fatalf("resolve admin-db path for limit enforcer: %v", err)
		}
		store, err := iadmin.NewSQLiteStore(resolvedDB)
		if err != nil {
			log.Fatalf("open admin store for limit enforcer: %v", err)
		}
		limiter := gateway.NewEmbeddedLimitEnforcer(store, orgID)
		log.Printf("limit enforcement enabled (embedded store)")
		return limiter, func() { _ = store.Close() }
	case "remote":
		log.Printf("limit enforcement: remote mode not yet implemented; using noop")
		return gateway.NewNoopLimitEnforcer(), nil
	default:
		log.Printf("limit enforcement: unknown policy mode %q; using noop", mode)
		return gateway.NewNoopLimitEnforcer(), nil
	}
}

// isContextErr reports whether err indicates normal context cancellation or
// deadline, which happens on clean shutdown and should not be treated as fatal.
func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
