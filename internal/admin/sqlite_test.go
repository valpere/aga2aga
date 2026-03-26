package admin_test

import (
	"context"
	"testing"
	"time"

	iadmin "github.com/valpere/aga2aga/internal/admin"
	"github.com/valpere/aga2aga/pkg/admin"
)

func newTestStore(t *testing.T) admin.Store {
	t.Helper()
	s, err := iadmin.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLiteStore_OrgRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()}
	if err := s.CreateOrg(ctx, org); err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	got, err := s.GetOrgByID(ctx, "org-1")
	if err != nil {
		t.Fatalf("GetOrgByID: %v", err)
	}
	if got.Name != "Acme" {
		t.Errorf("org.Name = %q, want %q", got.Name, "Acme")
	}
}

func TestSQLiteStore_UserRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)

	u := &admin.User{
		ID: "user-1", OrgID: "org-1", Username: "alice",
		Password: "hashed", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got.Role != admin.RoleAdmin {
		t.Errorf("user.Role = %q, want %q", got.Role, admin.RoleAdmin)
	}

	got2, err := s.GetUserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got2.Username != "alice" {
		t.Errorf("user.Username = %q, want %q", got2.Username, "alice")
	}
}

func TestSQLiteStore_AgentRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{ID: "user-1", OrgID: "org-1", Username: "alice", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()}
	_ = s.CreateUser(ctx, u)

	a := &admin.RegisteredAgent{
		ID: "reg-1", OrgID: "org-1", AgentID: "agent-alpha",
		DisplayName: "Alpha", Status: admin.AgentStatusActive,
		RegisteredBy: "user-1", RegisteredAt: time.Now().UTC(),
	}
	if err := s.RegisterAgent(ctx, a); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	got, err := s.GetAgent(ctx, "org-1", "agent-alpha")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.DisplayName != "Alpha" {
		t.Errorf("agent.DisplayName = %q, want %q", got.DisplayName, "Alpha")
	}

	if err := s.UpdateAgentStatus(ctx, "org-1", "agent-alpha", admin.AgentStatusSuspended); err != nil {
		t.Fatalf("UpdateAgentStatus: %v", err)
	}
	got, _ = s.GetAgent(ctx, "org-1", "agent-alpha")
	if got.Status != admin.AgentStatusSuspended {
		t.Errorf("agent.Status = %q, want %q", got.Status, admin.AgentStatusSuspended)
	}

	list, err := s.ListAgents(ctx, "org-1")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len(agents) = %d, want 1", len(list))
	}
}

func TestSQLiteStore_PolicyRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{ID: "user-1", OrgID: "org-1", Username: "alice", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()}
	_ = s.CreateUser(ctx, u)

	p := &admin.CommunicationPolicy{
		ID: "pol-1", OrgID: "org-1",
		SourceID: "agent-alpha", TargetID: "agent-beta",
		Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow,
		Priority: 10, CreatedBy: "user-1", CreatedAt: time.Now().UTC(),
	}
	if err := s.CreatePolicy(ctx, p); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	got, err := s.GetPolicy(ctx, "pol-1")
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if got.Action != admin.PolicyActionAllow {
		t.Errorf("policy.Action = %q, want %q", got.Action, admin.PolicyActionAllow)
	}

	got.Action = admin.PolicyActionDeny
	if err := s.UpdatePolicy(ctx, got); err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}
	got, _ = s.GetPolicy(ctx, "pol-1")
	if got.Action != admin.PolicyActionDeny {
		t.Errorf("after update policy.Action = %q, want %q", got.Action, admin.PolicyActionDeny)
	}

	list, err := s.ListPolicies(ctx, "org-1")
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len(policies) = %d, want 1", len(list))
	}

	if err := s.DeletePolicy(ctx, "pol-1"); err != nil {
		t.Fatalf("DeletePolicy: %v", err)
	}
	list, _ = s.ListPolicies(ctx, "org-1")
	if len(list) != 0 {
		t.Errorf("after delete len(policies) = %d, want 0", len(list))
	}
}

func TestSQLiteStore_AuditRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{ID: "user-1", OrgID: "org-1", Username: "alice", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()}
	_ = s.CreateUser(ctx, u)

	e := &admin.AuditEvent{
		ID: "evt-1", OrgID: "org-1", UserID: "user-1", Username: "alice",
		Action: "agent.register", TargetType: "agent", TargetID: "agent-alpha",
		Detail: "registered agent-alpha", CreatedAt: time.Now().UTC(),
	}
	if err := s.AppendAuditEvent(ctx, e); err != nil {
		t.Fatalf("AppendAuditEvent: %v", err)
	}

	events, err := s.ListAuditEvents(ctx, "org-1", 10)
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Action != "agent.register" {
		t.Errorf("event.Action = %q, want %q", events[0].Action, "agent.register")
	}
	if events[0].Username != "alice" {
		t.Errorf("event.Username = %q, want %q", events[0].Username, "alice")
	}
}

func TestSQLiteStore_APIKeyRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{ID: "user-1", OrgID: "org-1", Username: "alice", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()}
	_ = s.CreateUser(ctx, u)

	k := &admin.APIKey{
		ID: "key-1", OrgID: "org-1", Name: "gateway-prod",
		KeyHash: "abc123hash", Role: admin.RoleOperator,
		CreatedBy: "user-1", CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAPIKey(ctx, k); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	got, err := s.GetAPIKeyByHash(ctx, "abc123hash")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash: %v", err)
	}
	if got.Name != "gateway-prod" {
		t.Errorf("key.Name = %q, want %q", got.Name, "gateway-prod")
	}
	if got.RevokedAt.IsZero() == false {
		t.Errorf("key.RevokedAt should be zero (active)")
	}

	list, err := s.ListAPIKeys(ctx, "org-1")
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len(keys) = %d, want 1", len(list))
	}

	// Cross-org revocation must be rejected.
	if err := s.RevokeAPIKey(ctx, "org-other", "key-1"); err == nil {
		t.Error("RevokeAPIKey with wrong orgID: expected error, got nil")
	}

	if err := s.RevokeAPIKey(ctx, "org-1", "key-1"); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	got, _ = s.GetAPIKeyByHash(ctx, "abc123hash")
	if got.RevokedAt.IsZero() {
		t.Errorf("after revoke, key.RevokedAt should be non-zero")
	}

	// Revoked keys must not appear in ListAPIKeys.
	listAfterRevoke, err := s.ListAPIKeys(ctx, "org-1")
	if err != nil {
		t.Fatalf("ListAPIKeys after revoke: %v", err)
	}
	if len(listAfterRevoke) != 0 {
		t.Errorf("ListAPIKeys after revoke: got %d keys, want 0 (revoked keys must be excluded)", len(listAfterRevoke))
	}
}

func TestSQLiteStore_UpdateUserPassword(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{
		ID: "user-1", OrgID: "org-1", Username: "alice",
		Password: "old-hash", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateUser(ctx, u)

	if err := s.UpdateUserPassword(ctx, "user-1", "new-hash"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}

	got, err := s.GetUserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetUserByID after update: %v", err)
	}
	if got.Password != "new-hash" {
		t.Errorf("password = %q, want %q", got.Password, "new-hash")
	}

	// Updating a non-existent user must return an error.
	if err := s.UpdateUserPassword(ctx, "no-such-user", "x"); err == nil {
		t.Error("UpdateUserPassword with unknown id: expected error, got nil")
	}
}
