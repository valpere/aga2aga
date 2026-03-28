package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/valpere/aga2aga/pkg/admin"
)

// apiKeyFromRequest extracts, hashes, and looks up the Bearer token from the
// Authorization header. Returns the APIKey or nil if missing/invalid/revoked.
func (srv *Server) apiKeyFromRequest(r *http.Request) *admin.APIKey {
	auth := r.Header.Get("Authorization")
	raw, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok || raw == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	k, err := srv.store.GetAPIKeyByHash(r.Context(), hash)
	if err != nil || !k.RevokedAt.IsZero() {
		return nil
	}
	return k
}

// handleAPIAuth is the JSON endpoint the gateway calls to validate an agent
// API key. Returns 200 with the bound agent ID if the key is valid and has
// role=agent; otherwise 401.
//
// POST /api/v1/auth
// Authorization: Bearer <raw-api-key>
//
// Response (200): {"valid":true,"agent_id":"..."}
// Response (401): {"error":"unauthorized"}
func (srv *Server) handleAPIAuth(w http.ResponseWriter, r *http.Request) {
	k := srv.apiKeyFromRequest(r)
	if k == nil || k.Role != admin.RoleAgent {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"valid":    true,
		"agent_id": k.AgentID,
	})
}

// handleAPIEvaluate is the JSON endpoint the gateway calls to check whether a
// message from source to target is allowed.
//
// GET /api/v1/evaluate?source=<agent-id>&target=<agent-id>
// Authorization: Bearer <api-key>
//
// Response: {"action":"allow"} or {"action":"deny"}
func (srv *Server) handleAPIEvaluate(w http.ResponseWriter, r *http.Request) {
	k := srv.apiKeyFromRequest(r)
	if k == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	source := r.URL.Query().Get("source")
	target := r.URL.Query().Get("target")
	if source == "" || target == "" {
		http.Error(w, `{"error":"source and target are required"}`, http.StatusBadRequest)
		return
	}

	policies, err := srv.store.ListPolicies(r.Context(), k.OrgID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	action := admin.Evaluate(policies, source, target)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"action": string(action)})
}
