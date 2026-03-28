package gateway_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	iadmin "github.com/valpere/aga2aga/internal/admin"
	"github.com/valpere/aga2aga/internal/gateway"
	"github.com/valpere/aga2aga/pkg/admin"
)

func newAuthStore(t *testing.T) admin.Store {
	t.Helper()
	s, err := iadmin.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	_ = s.CreateOrg(ctx, &admin.Organization{ID: "org-1", Name: "Acme", CreatedAt: time.Now().UTC()})
	_ = s.CreateUser(ctx, &admin.User{ID: "u-1", OrgID: "org-1", Username: "admin", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()})
	return s
}

func rawKeyAndHash(raw string) (string, string) {
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:])
}

func TestEmbeddedAuthenticator_Success(t *testing.T) {
	s := newAuthStore(t)
	ctx := context.Background()

	raw, hash := rawKeyAndHash("test-raw-key-alpha")
	k := &admin.APIKey{
		ID: "k-1", OrgID: "org-1", Name: "alpha-key",
		KeyHash: hash, Role: admin.RoleAgent, AgentID: "agent-alpha",
		CreatedBy: "u-1", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)

	auth := gateway.NewEmbeddedAuthenticator(s)
	agentID, err := auth.Authenticate(ctx, raw)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if agentID != "agent-alpha" {
		t.Errorf("agentID = %q, want %q", agentID, "agent-alpha")
	}
}

func TestEmbeddedAuthenticator_WrongRole(t *testing.T) {
	s := newAuthStore(t)
	ctx := context.Background()

	raw, hash := rawKeyAndHash("test-raw-key-op")
	k := &admin.APIKey{
		ID: "k-2", OrgID: "org-1", Name: "op-key",
		KeyHash: hash, Role: admin.RoleOperator,
		CreatedBy: "u-1", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)

	auth := gateway.NewEmbeddedAuthenticator(s)
	_, err := auth.Authenticate(ctx, raw)
	if err == nil {
		t.Fatal("expected error for non-agent role, got nil")
	}
}

func TestEmbeddedAuthenticator_Revoked(t *testing.T) {
	s := newAuthStore(t)
	ctx := context.Background()

	raw, hash := rawKeyAndHash("test-raw-key-revoked")
	k := &admin.APIKey{
		ID: "k-3", OrgID: "org-1", Name: "revoked-key",
		KeyHash: hash, Role: admin.RoleAgent, AgentID: "agent-beta",
		CreatedBy: "u-1", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)
	_ = s.RevokeAPIKey(ctx, "org-1", "k-3")

	auth := gateway.NewEmbeddedAuthenticator(s)
	_, err := auth.Authenticate(ctx, raw)
	if err == nil {
		t.Fatal("expected error for revoked key, got nil")
	}
}

func TestEmbeddedAuthenticator_UnknownKey(t *testing.T) {
	s := newAuthStore(t)
	auth := gateway.NewEmbeddedAuthenticator(s)
	_, err := auth.Authenticate(context.Background(), "no-such-key")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestHTTPAuthenticator_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"valid":true,"agent_id":"agent-gamma"}`))
	}))
	t.Cleanup(srv.Close)

	auth, err := gateway.NewHTTPAuthenticator(srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPAuthenticator: %v", err)
	}

	agentID, err := auth.Authenticate(context.Background(), "valid-key")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if agentID != "agent-gamma" {
		t.Errorf("agentID = %q, want %q", agentID, "agent-gamma")
	}
}

func TestHTTPAuthenticator_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	auth, err := gateway.NewHTTPAuthenticator(srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPAuthenticator: %v", err)
	}
	_, err = auth.Authenticate(context.Background(), "bad-key")
	if err == nil {
		t.Fatal("expected error for unauthorized key, got nil")
	}
}
