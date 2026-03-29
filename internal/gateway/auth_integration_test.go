package gateway_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/valpere/aga2aga/internal/gateway"
)

// mockAuth implements AgentAuthenticator for testing tool authentication.
type mockAuth struct {
	agentID string
	err     error
}

func (m *mockAuth) Authenticate(_ context.Context, _ string) (string, error) {
	return m.agentID, m.err
}

func TestGateway_AuthenticateAgent_NilAuth(t *testing.T) {
	// When auth is nil, authenticateAgent must succeed (legacy mode).
	gw := gateway.New(nil, nil, nil, gateway.NewNoopMessageLogger(), gateway.NewNoopLimitEnforcer(), gateway.DefaultConfig())
	err := gw.AuthenticateAgentForTest(context.Background(), "agent-1", "any-key")
	if err != nil {
		t.Errorf("nil auth should allow all: %v", err)
	}
}

func TestGateway_AuthenticateAgent_ValidKey(t *testing.T) {
	auth := &mockAuth{agentID: "agent-1"}
	gw := gateway.New(nil, nil, auth, gateway.NewNoopMessageLogger(), gateway.NewNoopLimitEnforcer(), gateway.DefaultConfig())
	err := gw.AuthenticateAgentForTest(context.Background(), "agent-1", "valid-key")
	if err != nil {
		t.Errorf("valid key should succeed: %v", err)
	}
}

func TestGateway_AuthenticateAgent_IDMismatch(t *testing.T) {
	auth := &mockAuth{agentID: "agent-2"}
	gw := gateway.New(nil, nil, auth, gateway.NewNoopMessageLogger(), gateway.NewNoopLimitEnforcer(), gateway.DefaultConfig())
	err := gw.AuthenticateAgentForTest(context.Background(), "agent-1", "key-for-agent-2")
	if err == nil {
		t.Error("expected error for agent ID mismatch, got nil")
	}
}

func TestGateway_AuthenticateAgent_AuthError(t *testing.T) {
	auth := &mockAuth{err: fmt.Errorf("revoked")}
	gw := gateway.New(nil, nil, auth, gateway.NewNoopMessageLogger(), gateway.NewNoopLimitEnforcer(), gateway.DefaultConfig())
	err := gw.AuthenticateAgentForTest(context.Background(), "agent-1", "bad-key")
	if err == nil {
		t.Error("expected error from auth, got nil")
	}
}
