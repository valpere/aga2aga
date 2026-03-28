package admin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	iadmin "github.com/valpere/aga2aga/internal/admin"
	"github.com/valpere/aga2aga/pkg/admin"
)

var testHashKey = []byte("test-hash-key-32-bytes-long!!!!!")
var testBlockKey = []byte("test-block-key-32bytes-long!!!!")

func newTestHandler(t *testing.T, s admin.Store) http.Handler {
	t.Helper()
	srv, err := iadmin.NewServer(s, testHashKey, testBlockKey)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Handler()
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TestHandleAPIAuth_ValidAgentKey(t *testing.T) {
	s := newTestStore(t)
	handler := newTestHandler(t, s)
	ctx := t.Context()

	org := &admin.Organization{ID: "org-auth-1", Name: "AuthOrg", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{ID: "usr-auth-1", OrgID: "org-auth-1", Username: "authuser", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()}
	_ = s.CreateUser(ctx, u)

	rawKey := "auth-test-raw-key-001"
	k := &admin.APIKey{
		ID: "key-auth-1", OrgID: "org-auth-1", Name: "agent-key",
		KeyHash: hashKey(rawKey), Role: admin.RoleAgent, AgentID: "agent-x",
		CreatedBy: "usr-auth-1", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Valid   bool   `json:"valid"`
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Valid {
		t.Error("valid = false, want true")
	}
	if resp.AgentID != "agent-x" {
		t.Errorf("agent_id = %q, want %q", resp.AgentID, "agent-x")
	}
}

func TestHandleAPIAuth_NoKey(t *testing.T) {
	s := newTestStore(t)
	handler := newTestHandler(t, s)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHandleAPIAuth_WrongRole(t *testing.T) {
	s := newTestStore(t)
	handler := newTestHandler(t, s)
	ctx := t.Context()

	org := &admin.Organization{ID: "org-auth-2", Name: "AuthOrg2", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{ID: "usr-auth-2", OrgID: "org-auth-2", Username: "authuser2", Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC()}
	_ = s.CreateUser(ctx, u)

	rawKey := "auth-test-op-key-002"
	k := &admin.APIKey{
		ID: "key-auth-2", OrgID: "org-auth-2", Name: "op-key",
		KeyHash: hashKey(rawKey), Role: admin.RoleOperator,
		CreatedBy: "usr-auth-2", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
