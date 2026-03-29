package gateway_test

import (
	"context"
	"testing"
	"time"

	"github.com/valpere/aga2aga/internal/gateway"
	"github.com/valpere/aga2aga/pkg/admin"
)

// --- stub LimitStore ---

type stubLimitStore struct {
	global *admin.AgentLimits
	agents map[string]*admin.AgentLimits
}

func (s *stubLimitStore) UpsertAgentLimits(_ context.Context, l *admin.AgentLimits) error {
	if l.AgentID == "*" {
		s.global = l
	} else {
		if s.agents == nil {
			s.agents = make(map[string]*admin.AgentLimits)
		}
		s.agents[l.AgentID] = l
	}
	return nil
}

func (s *stubLimitStore) GetEffectiveLimits(_ context.Context, _, agentID string) (*admin.AgentLimits, error) {
	if l, ok := s.agents[agentID]; ok {
		return l, nil
	}
	return s.global, nil
}

func (s *stubLimitStore) ListAgentLimits(_ context.Context, _ string) ([]admin.AgentLimits, error) {
	return nil, nil
}

func (s *stubLimitStore) DeleteAgentLimits(_ context.Context, _ string) error {
	return nil
}

// --- NoopLimitEnforcer ---

func TestNoopLimitEnforcer(t *testing.T) {
	lim := gateway.NewNoopLimitEnforcer()
	ctx := context.Background()

	if err := lim.CheckSend(ctx, "agent-a", 999999); err != nil {
		t.Errorf("CheckSend: unexpected error: %v", err)
	}
	lim.RecordSend(ctx, "agent-a") // must not panic
	if err := lim.CheckPendingTasks(ctx, "agent-a", 999999); err != nil {
		t.Errorf("CheckPendingTasks: unexpected error: %v", err)
	}
	if n := lim.GetStreamMaxLen(ctx, "agent-a"); n != 0 {
		t.Errorf("GetStreamMaxLen = %d, want 0", n)
	}
	info, err := lim.GetEffectiveLimits(ctx, "agent-a")
	if err != nil {
		t.Errorf("GetEffectiveLimits: unexpected error: %v", err)
	}
	if info == nil {
		t.Error("GetEffectiveLimits: want non-nil LimitInfo, got nil")
	}
}

// --- EmbeddedLimitEnforcer ---

func TestEmbeddedLimitEnforcer_NoLimits(t *testing.T) {
	store := &stubLimitStore{}
	lim := gateway.NewEmbeddedLimitEnforcer(store, "org-1")
	ctx := context.Background()

	// No limits configured — everything should pass.
	if err := lim.CheckSend(ctx, "agent-a", 65536); err != nil {
		t.Errorf("CheckSend with no limits: %v", err)
	}
	if err := lim.CheckPendingTasks(ctx, "agent-a", 100); err != nil {
		t.Errorf("CheckPendingTasks with no limits: %v", err)
	}
	if n := lim.GetStreamMaxLen(ctx, "agent-a"); n != 0 {
		t.Errorf("GetStreamMaxLen with no limits = %d, want 0", n)
	}
}

func TestEmbeddedLimitEnforcer_BodySizeEnforced(t *testing.T) {
	store := &stubLimitStore{
		global: &admin.AgentLimits{
			OrgID: "org-1", AgentID: "*",
			MaxBodyBytes: 100,
		},
	}
	lim := gateway.NewEmbeddedLimitEnforcer(store, "org-1")
	ctx := context.Background()

	if err := lim.CheckSend(ctx, "agent-a", 50); err != nil {
		t.Errorf("CheckSend(50 <= 100): unexpected error: %v", err)
	}
	if err := lim.CheckSend(ctx, "agent-a", 101); err == nil {
		t.Error("CheckSend(101 > 100): expected error, got nil")
	}
}

func TestEmbeddedLimitEnforcer_RateLimit(t *testing.T) {
	store := &stubLimitStore{
		global: &admin.AgentLimits{
			OrgID: "org-1", AgentID: "*",
			MaxSendPerMin: 3,
		},
	}
	lim := gateway.NewEmbeddedLimitEnforcer(store, "org-1")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := lim.CheckSend(ctx, "agent-a", 0); err != nil {
			t.Fatalf("CheckSend[%d]: unexpected error: %v", i, err)
		}
		lim.RecordSend(ctx, "agent-a")
	}
	// 4th send should be rejected.
	if err := lim.CheckSend(ctx, "agent-a", 0); err == nil {
		t.Error("4th send within a minute: expected rate-limit error, got nil")
	}
}

func TestEmbeddedLimitEnforcer_PendingTasksEnforced(t *testing.T) {
	store := &stubLimitStore{
		global: &admin.AgentLimits{
			OrgID: "org-1", AgentID: "*",
			MaxPendingTasks: 2,
		},
	}
	lim := gateway.NewEmbeddedLimitEnforcer(store, "org-1")
	ctx := context.Background()

	if err := lim.CheckPendingTasks(ctx, "agent-a", 2); err != nil {
		t.Errorf("CheckPendingTasks(2 == 2): unexpected error: %v", err)
	}
	if err := lim.CheckPendingTasks(ctx, "agent-a", 3); err == nil {
		t.Error("CheckPendingTasks(3 > 2): expected error, got nil")
	}
}

func TestEmbeddedLimitEnforcer_StreamMaxLen(t *testing.T) {
	store := &stubLimitStore{
		global: &admin.AgentLimits{
			OrgID: "org-1", AgentID: "*",
			MaxStreamLen: 500,
		},
	}
	lim := gateway.NewEmbeddedLimitEnforcer(store, "org-1")
	ctx := context.Background()

	if n := lim.GetStreamMaxLen(ctx, "any-agent"); n != 500 {
		t.Errorf("GetStreamMaxLen = %d, want 500", n)
	}
}

func TestEmbeddedLimitEnforcer_AgentSpecificOverridesGlobal(t *testing.T) {
	store := &stubLimitStore{
		global: &admin.AgentLimits{OrgID: "org-1", AgentID: "*", MaxBodyBytes: 100},
		agents: map[string]*admin.AgentLimits{
			"vip-agent": {OrgID: "org-1", AgentID: "vip-agent", MaxBodyBytes: 9999},
		},
	}
	lim := gateway.NewEmbeddedLimitEnforcer(store, "org-1")
	ctx := context.Background()

	// regular agent subject to global limit
	if err := lim.CheckSend(ctx, "regular", 200); err == nil {
		t.Error("regular agent 200 > 100: expected error")
	}
	// vip-agent has higher limit
	if err := lim.CheckSend(ctx, "vip-agent", 5000); err != nil {
		t.Errorf("vip-agent 5000 <= 9999: unexpected error: %v", err)
	}
}

func TestEmbeddedLimitEnforcer_GetEffectiveLimits(t *testing.T) {
	store := &stubLimitStore{
		global: &admin.AgentLimits{
			OrgID: "org-1", AgentID: "*",
			MaxBodyBytes: 1024, MaxSendPerMin: 10, MaxPendingTasks: 5, MaxStreamLen: 100,
		},
	}
	lim := gateway.NewEmbeddedLimitEnforcer(store, "org-1")
	ctx := context.Background()

	info, err := lim.GetEffectiveLimits(ctx, "agent-a")
	if err != nil {
		t.Fatalf("GetEffectiveLimits: %v", err)
	}
	if info.MaxBodyBytes != 1024 {
		t.Errorf("MaxBodyBytes = %d, want 1024", info.MaxBodyBytes)
	}
	if info.MaxSendPerMin != 10 {
		t.Errorf("MaxSendPerMin = %d, want 10", info.MaxSendPerMin)
	}
	if info.MaxStreamLen != 100 {
		t.Errorf("MaxStreamLen = %d, want 100", info.MaxStreamLen)
	}
}

func TestEmbeddedLimitEnforcer_CacheTTL(t *testing.T) {
	callCount := 0
	store := &countingLimitStore{calls: &callCount}
	lim := gateway.NewEmbeddedLimitEnforcerWithCacheTTL(store, "org-1", 50*time.Millisecond)
	ctx := context.Background()

	// Two calls within TTL → one store hit
	_ = lim.CheckSend(ctx, "agent-a", 0)
	_ = lim.CheckSend(ctx, "agent-a", 0)
	if callCount != 1 {
		t.Errorf("within TTL: expected 1 store call, got %d", callCount)
	}

	// After TTL expires, next call should hit the store again.
	time.Sleep(60 * time.Millisecond)
	_ = lim.CheckSend(ctx, "agent-a", 0)
	if callCount != 2 {
		t.Errorf("after TTL: expected 2 store calls, got %d", callCount)
	}
}

// countingLimitStore counts GetEffectiveLimits calls for cache TTL test.
type countingLimitStore struct {
	calls *int
}

func (c *countingLimitStore) UpsertAgentLimits(_ context.Context, _ *admin.AgentLimits) error {
	return nil
}
func (c *countingLimitStore) GetEffectiveLimits(_ context.Context, _, _ string) (*admin.AgentLimits, error) {
	*c.calls++
	return nil, nil
}
func (c *countingLimitStore) ListAgentLimits(_ context.Context, _ string) ([]admin.AgentLimits, error) {
	return nil, nil
}
func (c *countingLimitStore) DeleteAgentLimits(_ context.Context, _ string) error { return nil }
