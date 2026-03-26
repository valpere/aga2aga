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
//	--task-read-timeout  Max wait for a task delivery in get_task (default: 5s)
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	goredis "github.com/redis/go-redis/v9"

	iadmin "github.com/valpere/aga2aga/internal/admin"
	"github.com/valpere/aga2aga/internal/gateway"
	redistransport "github.com/valpere/aga2aga/pkg/transport/redis"
)

func main() {
	redisAddr       := flag.String("redis-addr", "localhost:6379", "Redis address")
	mcpTransport    := flag.String("mcp-transport", "stdio", "MCP transport: stdio or http")
	addr            := flag.String("addr", ":3000", "Listen address (HTTP transport only)")
	policyMode      := flag.String("policy-mode", "embedded", "Policy mode: embedded or remote")
	adminDB         := flag.String("admin-db", "admin.db", "SQLite path (embedded policy mode)")
	adminURL        := flag.String("admin-url", "", "Admin server URL (remote policy mode)")
	adminAPIKey     := flag.String("admin-api-key", "", "Bearer token (remote policy mode)")
	pendingTTL      := flag.Duration("pending-ttl", time.Hour, "PendingMap entry TTL")
	agentID         := flag.String("agent-id", "mcp-gateway", "Gateway identity used in policy checks")
	taskReadTimeout := flag.Duration("task-read-timeout", 5*time.Second, "Max wait for a task delivery in get_task")
	flag.Parse()

	// Redis Streams transport. Deferred close order: transport first (drains
	// in-flight I/O), then the underlying Redis client.
	rdb := goredis.NewClient(&goredis.Options{Addr: *redisAddr})
	trans := redistransport.New(rdb, redistransport.Options{})
	defer func() { _ = trans.Close() }()
	defer func() { _ = rdb.Close() }()

	// Policy enforcer.
	enf, closeEnf := mustEnforcer(*policyMode, *adminDB, *adminURL, *adminAPIKey)
	if closeEnf != nil {
		defer closeEnf()
	}

	// Gateway configuration.
	cfg := gateway.DefaultConfig()
	cfg.AgentID = *agentID
	cfg.TaskReadTimeout = *taskReadTimeout
	cfg.PendingTTL = *pendingTTL

	gw := gateway.New(trans, enf, cfg)

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

		handler := mcpsdk.NewStreamableHTTPHandler(
			func(_ *http.Request) *mcpsdk.Server { return gw.Server() },
			nil,
		)
		httpSrv := &http.Server{
			Addr:        *addr,
			Handler:     handler,
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
		store, err := iadmin.NewSQLiteStore(adminDB)
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

// isContextErr reports whether err indicates normal context cancellation or
// deadline, which happens on clean shutdown and should not be treated as fatal.
func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
