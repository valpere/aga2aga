package gateway

import "time"

// Config holds runtime configuration for the Gateway.
type Config struct {
	// AgentID is the gateway's identity used in policy checks.
	AgentID string
	// TaskReadTimeout is the maximum time to wait for a task delivery
	// when handling get_task. Default: 5s.
	TaskReadTimeout time.Duration
	// PendingTTL is the maximum time a pending task entry lives in
	// PendingMap before being evicted. Default: 5m.
	PendingTTL time.Duration
}

// DefaultConfig returns a Config with production-safe defaults.
func DefaultConfig() Config {
	return Config{
		AgentID:         "mcp-gateway",
		TaskReadTimeout: 5 * time.Second,
		PendingTTL:      5 * time.Minute,
	}
}
