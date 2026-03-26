package gateway_test

import (
	"testing"
	"time"

	"github.com/valpere/aga2aga/internal/gateway"
)

func TestDefaultConfig(t *testing.T) {
	cfg := gateway.DefaultConfig()

	if cfg.AgentID == "" {
		t.Error("AgentID must not be empty")
	}
	if cfg.TaskReadTimeout != 5*time.Second {
		t.Errorf("TaskReadTimeout = %v; want 5s", cfg.TaskReadTimeout)
	}
	if cfg.PendingTTL != 5*time.Minute {
		t.Errorf("PendingTTL = %v; want 5m", cfg.PendingTTL)
	}
}
