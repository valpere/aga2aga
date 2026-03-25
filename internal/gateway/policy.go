package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/valpere/aga2aga/pkg/admin"
)

// PolicyEnforcer checks whether a source agent is allowed to communicate
// with a target agent under the current policy set.
type PolicyEnforcer interface {
	Allowed(ctx context.Context, source, target string) (bool, error)
}

// EmbeddedEnforcer evaluates policies in-process by querying a PolicyStore
// and calling admin.Evaluate. Use this for single-node deployments where the
// gateway and admin server share the same database.
type EmbeddedEnforcer struct {
	store admin.PolicyStore
	orgID string
}

// NewEmbeddedEnforcer creates an EmbeddedEnforcer for the given org.
func NewEmbeddedEnforcer(store admin.PolicyStore, orgID string) *EmbeddedEnforcer {
	return &EmbeddedEnforcer{store: store, orgID: orgID}
}

// Allowed returns true if admin.Evaluate returns PolicyActionAllow for the
// current policy set. Returns false (not error) when the default deny applies;
// returns error only if the store call fails.
func (e *EmbeddedEnforcer) Allowed(ctx context.Context, source, target string) (bool, error) {
	policies, err := e.store.ListPolicies(ctx, e.orgID)
	if err != nil {
		return false, fmt.Errorf("gateway/policy: list policies: %w", err)
	}
	return admin.Evaluate(policies, source, target) == admin.PolicyActionAllow, nil
}

// HTTPEnforcer evaluates policies by calling the admin server's evaluate
// endpoint. Use this when the gateway and admin server are separate processes.
//
// SECURITY: token is a Bearer credential — never log it or include it in
// error messages.
type HTTPEnforcer struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewHTTPEnforcer creates an HTTPEnforcer that calls baseURL/api/v1/evaluate
// with the given Bearer token. Uses a 5-second HTTP timeout per call.
func NewHTTPEnforcer(baseURL, token string) *HTTPEnforcer {
	return &HTTPEnforcer{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// Allowed returns true if the admin server responds with {"action":"allow"}.
// Returns false (not error) on a deny response; returns error on non-200
// status or network failure.
func (e *HTTPEnforcer) Allowed(ctx context.Context, source, target string) (bool, error) {
	endpoint := e.baseURL + "/api/v1/evaluate?source=" + url.QueryEscape(source) + "&target=" + url.QueryEscape(target)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("gateway/policy: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)

	resp, err := e.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("gateway/policy: http evaluate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("gateway/policy: evaluate returned status %d", resp.StatusCode)
	}

	var result struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("gateway/policy: decode response: %w", err)
	}
	return result.Action == "allow", nil
}
