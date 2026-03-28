package admin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

// TestSQLiteStore_AgentKey verifies that API keys with role=agent store and
// retrieve the bound AgentID field correctly.
func TestSQLiteStore_AgentKey(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	org := &admin.Organization{ID: "agorg-1", Name: "AgentOrg", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{ID: "aguser-1", OrgID: "agorg-1", Username: "agadmin", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()}
	_ = s.CreateUser(ctx, u)

	t.Run("CreateAndRetrieve", func(t *testing.T) {
		k := &admin.APIKey{
			ID:        uuid.New().String(),
			OrgID:     "agorg-1",
			Name:      "agent-alpha-key",
			KeyHash:   "deadbeef01",
			Role:      admin.RoleAgent,
			AgentID:   "agent-alpha",
			CreatedBy: "aguser-1",
			CreatedAt: time.Now().UTC(),
		}
		if err := s.CreateAPIKey(ctx, k); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}

		got, err := s.GetAPIKeyByHash(ctx, "deadbeef01")
		if err != nil {
			t.Fatalf("GetAPIKeyByHash: %v", err)
		}
		if got.Role != admin.RoleAgent {
			t.Errorf("Role = %q, want %q", got.Role, admin.RoleAgent)
		}
		if got.AgentID != "agent-alpha" {
			t.Errorf("AgentID = %q, want %q", got.AgentID, "agent-alpha")
		}
	})

	t.Run("ListIncludesAgentID", func(t *testing.T) {
		k := &admin.APIKey{
			ID:        uuid.New().String(),
			OrgID:     "agorg-1",
			Name:      "agent-beta-key",
			KeyHash:   "cafebabe02",
			Role:      admin.RoleAgent,
			AgentID:   "agent-beta",
			CreatedBy: "aguser-1",
			CreatedAt: time.Now().UTC(),
		}
		if err := s.CreateAPIKey(ctx, k); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		keys, err := s.ListAPIKeys(ctx, "agorg-1")
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		var found bool
		for _, kk := range keys {
			if kk.KeyHash == "cafebabe02" {
				found = true
				if kk.AgentID != "agent-beta" {
					t.Errorf("AgentID = %q, want %q", kk.AgentID, "agent-beta")
				}
			}
		}
		if !found {
			t.Error("agent-beta-key not found in ListAPIKeys")
		}
	})

}
