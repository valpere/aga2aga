package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/valpere/aga2aga/pkg/admin"
)

// AgentAuthenticator validates agent API keys presented with MCP tool calls.
// It returns the agent ID that the key is bound to, or an error if the key is
// invalid, revoked, or not an agent-role key.
//
// When an AgentAuthenticator is wired into the Gateway, every tool handler
// calls Authenticate before performing any work. If auth is nil (legacy mode),
// the gateway skips authentication and logs a warning — backward-compatible
// until --require-agent-key is set.
type AgentAuthenticator interface {
	Authenticate(ctx context.Context, rawKey string) (agentID string, err error)
}

// EmbeddedAuthenticator validates agent API keys directly against a local
// admin.APIKeyStore. Use this in embedded (single-node) deployments where the
// gateway and admin server share the same SQLite database.
type EmbeddedAuthenticator struct {
	store admin.APIKeyStore
}

// NewEmbeddedAuthenticator creates an EmbeddedAuthenticator backed by store.
func NewEmbeddedAuthenticator(store admin.APIKeyStore) *EmbeddedAuthenticator {
	return &EmbeddedAuthenticator{store: store}
}

// Authenticate hashes rawKey with SHA-256, looks it up in the store, verifies
// it is not revoked, and confirms the role is agent. Returns the bound agent ID.
func (a *EmbeddedAuthenticator) Authenticate(ctx context.Context, rawKey string) (string, error) {
	sum := sha256.Sum256([]byte(rawKey))
	hash := hex.EncodeToString(sum[:])

	k, err := a.store.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return "", fmt.Errorf("gateway: authentication failed: key not found: %w", err)
	}
	if !k.RevokedAt.IsZero() {
		return "", fmt.Errorf("gateway/auth: key is revoked")
	}
	if k.Role != admin.RoleAgent {
		return "", fmt.Errorf("gateway/auth: key role %q is not agent", k.Role)
	}
	return k.AgentID, nil
}

// maxAuthResponseBytes caps the /api/v1/auth response body to prevent
// unbounded memory allocation (CWE-400). The expected payload is small JSON.
const maxAuthResponseBytes = 4 * 1024

// HTTPAuthenticator validates agent API keys by calling the admin server's
// /api/v1/auth endpoint with the raw key as a Bearer token.
// Use this when the gateway and admin server run as separate processes.
//
// SECURITY: the raw key is transmitted as a Bearer token — always use an
// https baseURL in production to prevent credential interception.
type HTTPAuthenticator struct {
	baseURL string
	client  *http.Client
}

// NewHTTPAuthenticator creates an HTTPAuthenticator that calls
// baseURL/api/v1/auth. baseURL must be an http or https URL with a non-empty
// host; returns an error otherwise.
func NewHTTPAuthenticator(baseURL string) (*HTTPAuthenticator, error) {
	// Reuse the same URL validation logic as HTTPEnforcer.
	enf, err := NewHTTPEnforcer(baseURL, "placeholder")
	if err != nil {
		return nil, fmt.Errorf("gateway/auth: invalid admin baseURL: %w", err)
	}
	return &HTTPAuthenticator{
		baseURL: enf.baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// Authenticate calls POST /api/v1/auth with the raw key as a Bearer token and
// returns the bound agent ID from the response.
func (a *HTTPAuthenticator) Authenticate(ctx context.Context, rawKey string) (string, error) {
	endpoint := a.baseURL + "/api/v1/auth"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("gateway/auth: build request: %w", err)
	}
	// SECURITY: rawKey is a Bearer credential — never include it in error messages.
	req.Header.Set("Authorization", "Bearer "+rawKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway/auth: http auth: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("gateway/auth: authentication failed")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gateway/auth: auth endpoint returned status %d", resp.StatusCode)
	}

	var result struct {
		Valid    bool   `json:"valid"`
		AgentID  string `json:"agent_id"`
	}
	limited := io.LimitReader(resp.Body, maxAuthResponseBytes)
	if err := json.NewDecoder(limited).Decode(&result); err != nil {
		return "", fmt.Errorf("gateway/auth: decode response: %w", err)
	}
	if !result.Valid {
		return "", fmt.Errorf("gateway/auth: authentication failed")
	}
	return result.AgentID, nil
}

