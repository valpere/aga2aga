package gateway

import (
	"fmt"
	"time"
)

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
	// DefaultAgentName is substituted for the agent field in any tool call
	// that omits it. Set from AGA2AGA_AGENT_NAME in stdio transport only.
	DefaultAgentName string
	// DefaultAgentKey is substituted for the api_key field in any tool call
	// that omits it. Set from AGA2AGA_API_KEY in stdio transport only.
	// Must never appear in logs or error messages (CWE-532).
	DefaultAgentKey string
}

// String returns a human-readable Config summary intended for logging and
// debug output only — do not use for serialisation or persistence.
// DefaultAgentKey is always redacted to prevent accidental credential exposure
// when Config is formatted with %v or %+v (CWE-532).
func (c Config) String() string {
	keyStatus := "<not set>"
	if c.DefaultAgentKey != "" {
		keyStatus = "<redacted>"
	}
	return fmt.Sprintf(
		"Config{AgentID:%q DefaultAgentName:%q DefaultAgentKey:%s TaskReadTimeout:%v PendingTTL:%v}",
		c.AgentID, c.DefaultAgentName, keyStatus, c.TaskReadTimeout, c.PendingTTL,
	)
}

// DefaultConfig returns a Config with production-safe defaults.
func DefaultConfig() Config {
	return Config{
		AgentID:         "mcp-gateway",
		TaskReadTimeout: 5 * time.Second,
		PendingTTL:      5 * time.Minute,
	}
}
