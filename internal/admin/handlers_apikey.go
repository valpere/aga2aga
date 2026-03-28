package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

// agentIDRE is the same pattern used by the gateway to validate agent identifiers.
var agentIDRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}[a-zA-Z0-9]$`) //nolint:gochecknoglobals

// isValidAgentID reports whether s satisfies the agent ID rules.
func isValidAgentID(s string) bool {
	return agentIDRE.MatchString(s)
}

type apiKeyListPage struct {
	Page    string
	Session sessionData
	Keys    []admin.APIKey
	NewKey  string // raw key shown once after creation; empty otherwise
}

func (srv *Server) handleAPIKeyList(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	keys, err := srv.store.ListAPIKeys(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load API keys", http.StatusInternalServerError)
		return
	}
	srv.render(w, "apikeys.html", apiKeyListPage{Page: "apikeys", Session: sd, Keys: keys})
}

func (srv *Server) handleAPIKeyNewPost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	role := admin.Role(r.FormValue("role"))
	agentID := r.FormValue("agent_id")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	switch role {
	case admin.RoleOperator, admin.RoleViewer:
		// no additional fields required
	case admin.RoleAgent:
		if !isValidAgentID(agentID) {
			http.Error(w, "role=agent requires a valid agent_id", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "role must be operator, viewer, or agent", http.StatusBadRequest)
		return
	}

	rawKey, hash, err := generateAPIKey()
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	k := &admin.APIKey{
		ID: uuid.New().String(), OrgID: sd.OrgID,
		Name: name, KeyHash: hash, Role: role, AgentID: agentID,
		CreatedBy: sd.UserID, CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateAPIKey(r.Context(), k); err != nil {
		http.Error(w, "failed to save key: "+err.Error(), http.StatusInternalServerError)
		return
	}
	recordAudit(r.Context(), srv.store, sd, "apikey.create", "apikey", k.ID,
		fmt.Sprintf("created API key %q with role %s", name, role))

	keys, err := srv.store.ListAPIKeys(r.Context(), sd.OrgID)
	if err != nil {
		log.Printf("handleAPIKeyNewPost: failed to list API keys for org %q: %v", sd.OrgID, err)
	}
	srv.render(w, "apikeys.html", apiKeyListPage{
		Page: "apikeys", Session: sd, Keys: keys, NewKey: rawKey,
	})
}

func (srv *Server) handleAPIKeyRevoke(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	id := r.PathValue("id")
	if err := srv.store.RevokeAPIKey(r.Context(), sd.OrgID, id); err != nil {
		http.Error(w, "revoke failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	recordAudit(r.Context(), srv.store, sd, "apikey.revoke", "apikey", id, "revoked API key "+id)
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

// generateAPIKey creates a 32-byte random key and returns the raw hex string
// (shown once) and its SHA-256 hex hash (stored in DB).
func generateAPIKey() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}
