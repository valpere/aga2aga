package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/valpere/aga2aga/internal/gateway"
	"github.com/valpere/aga2aga/pkg/admin"
)

// mockPolicyStore implements admin.PolicyStore for tests.
// Only ListPolicies is used; other methods call t.Fatalf if invoked unexpectedly.
type mockPolicyStore struct {
	t        *testing.T
	policies []admin.CommunicationPolicy
	err      error
}

func (m *mockPolicyStore) ListPolicies(_ context.Context, _ string) ([]admin.CommunicationPolicy, error) {
	return m.policies, m.err
}
func (m *mockPolicyStore) CreatePolicy(_ context.Context, _ *admin.CommunicationPolicy) error {
	m.t.Fatalf("mockPolicyStore: CreatePolicy called unexpectedly")
	return nil
}
func (m *mockPolicyStore) GetPolicy(_ context.Context, _ string) (*admin.CommunicationPolicy, error) {
	m.t.Fatalf("mockPolicyStore: GetPolicy called unexpectedly")
	return nil, nil
}
func (m *mockPolicyStore) UpdatePolicy(_ context.Context, _ *admin.CommunicationPolicy) error {
	m.t.Fatalf("mockPolicyStore: UpdatePolicy called unexpectedly")
	return nil
}
func (m *mockPolicyStore) DeletePolicy(_ context.Context, _ string) error {
	m.t.Fatalf("mockPolicyStore: DeletePolicy called unexpectedly")
	return nil
}

func TestEmbeddedEnforcer_Allowed(t *testing.T) {
	allowPolicy := admin.CommunicationPolicy{
		OrgID:     "org1",
		SourceID:  "agent-a",
		TargetID:  "agent-b",
		Direction: admin.DirectionUnidirectional,
		Action:    admin.PolicyActionAllow,
		Priority:  10,
	}

	tests := []struct {
		name      string
		policies  []admin.CommunicationPolicy
		storeErr  error
		source    string
		target    string
		wantAllow bool
		wantErr   bool
	}{
		{
			name:      "allow: matching policy exists",
			policies:  []admin.CommunicationPolicy{allowPolicy},
			source:    "agent-a",
			target:    "agent-b",
			wantAllow: true,
		},
		{
			name:      "deny: no matching policy (default deny)",
			policies:  []admin.CommunicationPolicy{},
			source:    "agent-a",
			target:    "agent-b",
			wantAllow: false,
		},
		{
			name:     "store error propagates",
			storeErr: errors.New("db down"),
			source:   "agent-a",
			target:   "agent-b",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockPolicyStore{t: t, policies: tc.policies, err: tc.storeErr}
			e := gateway.NewEmbeddedEnforcer(store, "org1")

			got, err := e.Allowed(context.Background(), tc.source, tc.target)

			if (err != nil) != tc.wantErr {
				t.Errorf("Allowed() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.wantAllow {
				t.Errorf("Allowed() = %v, want %v", got, tc.wantAllow)
			}
		})
	}
}

func TestHTTPEnforcer_Allowed(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		closeEarly bool
		wantAllow  bool
		wantErr    bool
	}{
		{
			name: "allow response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					http.Error(w, "bad auth", http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"action":"allow"}`)
			},
			wantAllow: true,
		},
		{
			name: "deny response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"action":"deny"}`)
			},
			wantAllow: false,
		},
		{
			name: "bad status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name:       "network error",
			closeEarly: true,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ts *httptest.Server
			if tc.closeEarly {
				ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				ts.Close()
			} else {
				ts = httptest.NewServer(tc.handler)
				defer ts.Close()
			}

			e, err := gateway.NewHTTPEnforcer(ts.URL, "test-token")
			if err != nil {
				t.Fatalf("NewHTTPEnforcer: %v", err)
			}
			got, err := e.Allowed(context.Background(), "agent-a", "agent-b")

			if (err != nil) != tc.wantErr {
				t.Errorf("Allowed() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.wantAllow {
				t.Errorf("Allowed() = %v, want %v", got, tc.wantAllow)
			}
		})
	}
}

func TestNewHTTPEnforcer_InvalidURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "empty string", baseURL: ""},
		{name: "file scheme", baseURL: "file:///etc/passwd"},
		{name: "no scheme", baseURL: "admin:8080"},
		{name: "no host", baseURL: "https://"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := gateway.NewHTTPEnforcer(tc.baseURL, "tok")
			if err == nil {
				t.Errorf("NewHTTPEnforcer(%q) expected error, got nil", tc.baseURL)
			}
		})
	}
}

// TestEmbeddedEnforcer_ListPoliciesFor verifies that EmbeddedEnforcer
// implements PolicyQuerier and filters policies by agent ID.
func TestEmbeddedEnforcer_ListPoliciesFor(t *testing.T) {
	policies := []admin.CommunicationPolicy{
		{ID: "p1", SourceID: "agent-a", TargetID: "agent-b", Action: "allow"},
		{ID: "p2", SourceID: "agent-b", TargetID: "agent-a", Action: "allow"},
		{ID: "p3", SourceID: "agent-c", TargetID: "agent-d", Action: "deny"},
		{ID: "p4", SourceID: "*",       TargetID: "agent-a", Action: "deny"},
	}
	store := &mockPolicyStore{t: t, policies: policies}
	enf := gateway.NewEmbeddedEnforcer(store, "org-1")

	// Must implement PolicyQuerier.
	querier, ok := any(enf).(gateway.PolicyQuerier)
	if !ok {
		t.Fatal("EmbeddedEnforcer does not implement PolicyQuerier")
	}

	got, err := querier.ListPoliciesFor(context.Background(), "agent-a")
	if err != nil {
		t.Fatalf("ListPoliciesFor: %v", err)
	}
	// Should return p1 (source=agent-a), p2 (target=agent-a), p4 (target=agent-a).
	if len(got) != 3 {
		t.Fatalf("got %d policies, want 3 (p1, p2, p4); got IDs: %v",
			len(got), policyIDs(got))
	}
	// p3 should NOT appear.
	for _, p := range got {
		if p.ID == "p3" {
			t.Error("p3 (unrelated to agent-a) should not be in results")
		}
	}
}

func policyIDs(ps []admin.CommunicationPolicy) []string {
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	return ids
}
