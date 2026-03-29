package admin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/valpere/aga2aga/pkg/admin"
)

// TestHandleLimitsList_RequiresAuth verifies that GET /limits redirects to
// login when no session cookie is provided.
func TestHandleLimitsList_RequiresAuth(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	req := httptest.NewRequest(http.MethodGet, "/limits", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// requireAuth redirects to /login
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303 (redirect to login)", w.Code)
	}
}

// TestHandleAPILimitsCheck_NoKey rejects requests without a valid API key.
func TestHandleAPILimitsCheck_NoKey(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits/check?agent=agent-a", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// TestHandleAPILimitsCheck_AgentKey returns limits for a valid agent key.
func TestHandleAPILimitsCheck_AgentKey(t *testing.T) {
	s := newTestStore(t)
	ctx := t.Context()

	org := &admin.Organization{ID: "org-lc-1", Name: "LimCheck", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{
		ID: "usr-lc-1", OrgID: "org-lc-1", Username: "lc-admin",
		Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateUser(ctx, u)

	rawKey := "lc-agent-key-001"
	k := &admin.APIKey{
		ID: "key-lc-1", OrgID: "org-lc-1", Name: "agent-key",
		KeyHash: hashKey(rawKey), Role: admin.RoleAgent, AgentID: "agent-lc",
		CreatedBy: "usr-lc-1", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)

	// Upsert limits for the agent
	_ = s.UpsertAgentLimits(ctx, &admin.AgentLimits{
		ID: "lim-lc-1", OrgID: "org-lc-1", AgentID: "agent-lc",
		MaxBodyBytes: 32768, MaxSendPerMin: 30,
		UpdatedAt: time.Now().UTC(), UpdatedBy: "usr-lc-1",
	})

	h := newTestHandler(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits/check?agent=agent-lc", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		MaxBodyBytes  int `json:"max_body_bytes"`
		MaxSendPerMin int `json:"max_send_per_min"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.MaxBodyBytes != 32768 {
		t.Errorf("max_body_bytes = %d, want 32768", resp.MaxBodyBytes)
	}
	if resp.MaxSendPerMin != 30 {
		t.Errorf("max_send_per_min = %d, want 30", resp.MaxSendPerMin)
	}
}

// TestHandleAPILimitsCheck_NoLimitsConfigured returns 200 with all-zero values
// when no limit rows exist for the agent.
func TestHandleAPILimitsCheck_NoLimitsConfigured(t *testing.T) {
	s := newTestStore(t)
	ctx := t.Context()

	org := &admin.Organization{ID: "org-lc-2", Name: "LimCheckNone", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{
		ID: "usr-lc-2", OrgID: "org-lc-2", Username: "lc-admin2",
		Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateUser(ctx, u)

	rawKey := "lc-agent-key-002"
	k := &admin.APIKey{
		ID: "key-lc-2", OrgID: "org-lc-2", Name: "agent-key",
		KeyHash: hashKey(rawKey), Role: admin.RoleAgent, AgentID: "agent-lc2",
		CreatedBy: "usr-lc-2", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)

	h := newTestHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits/check?agent=agent-lc2", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		MaxBodyBytes    int   `json:"max_body_bytes"`
		MaxSendPerMin   int   `json:"max_send_per_min"`
		MaxPendingTasks int   `json:"max_pending_tasks"`
		MaxStreamLen    int64 `json:"max_stream_len"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// All-zero means unlimited — not a panic.
	if resp.MaxBodyBytes != 0 || resp.MaxSendPerMin != 0 || resp.MaxPendingTasks != 0 || resp.MaxStreamLen != 0 {
		t.Errorf("expected all-zero limits for unconfigured agent, got %+v", resp)
	}
}

// TestHandleAPILimitsCheck_WrongRole rejects non-agent API keys.
func TestHandleAPILimitsCheck_WrongRole(t *testing.T) {
	s := newTestStore(t)
	ctx := t.Context()

	org := &admin.Organization{ID: "org-lc-3", Name: "LimCheckRole", CreatedAt: time.Now().UTC()}
	_ = s.CreateOrg(ctx, org)
	u := &admin.User{
		ID: "usr-lc-3", OrgID: "org-lc-3", Username: "lc-admin3",
		Password: "h", Role: admin.RoleAdmin, CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateUser(ctx, u)

	rawKey := "lc-operator-key-001"
	k := &admin.APIKey{
		ID: "key-lc-3", OrgID: "org-lc-3", Name: "operator-key",
		KeyHash: hashKey(rawKey), Role: admin.RoleOperator,
		CreatedBy: "usr-lc-3", CreatedAt: time.Now().UTC(),
	}
	_ = s.CreateAPIKey(ctx, k)

	h := newTestHandler(t, s)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits/check?agent=agent-x", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (operator key must not access limits check)", w.Code)
	}
}

// TestHandleLimitsNew_RequiresOperator verifies that POST /limits/new redirects
// without a session.
func TestHandleLimitsNew_RequiresOperator(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	form := url.Values{"agent_id": {"agent-x"}, "max_body_bytes": {"1024"}}
	req := httptest.NewRequest(http.MethodPost, "/limits/new",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// requireAuth redirects to /login when no session present
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303 (redirect to login)", w.Code)
	}
}
