package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

type limitsListPage struct {
	Page    string
	Session sessionData
	Limits  []admin.AgentLimits
	CanEdit bool
}

type limitsEditPage struct {
	Page    string
	Session sessionData
	Limit   admin.AgentLimits
	IsNew   bool
	Error   string
}

// handleLimitsList renders GET /limits — the agent limits management page.
func (srv *Server) handleLimitsList(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	limits, err := srv.store.ListAgentLimits(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load limits", http.StatusInternalServerError)
		return
	}
	srv.render(w, "limits.html", limitsListPage{
		Page:    "limits",
		Session: sd,
		Limits:  limits,
		CanEdit: roleAtLeast(admin.Role(sd.Role), admin.RoleOperator),
	})
}

// handleLimitsNewGet renders GET /limits/new — the create-limit form.
func (srv *Server) handleLimitsNewGet(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	srv.render(w, "limits_edit.html", limitsEditPage{
		Page:    "limits",
		Session: sd,
		Limit:   admin.AgentLimits{AgentID: "*"},
		IsNew:   true,
	})
}

// handleLimitsNewPost processes POST /limits/new.
func (srv *Server) handleLimitsNewPost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	l := admin.AgentLimits{
		ID:              uuid.NewString(),
		OrgID:           sd.OrgID,
		AgentID:         r.FormValue("agent_id"),
		MaxBodyBytes:    formInt(r, "max_body_bytes"),
		MaxSendPerMin:   formInt(r, "max_send_per_min"),
		MaxPendingTasks: formInt(r, "max_pending"),
		MaxStreamLen:    int64(formInt(r, "max_stream_len")),
		UpdatedAt:       time.Now().UTC(),
		UpdatedBy:       sd.UserID,
	}
	if l.AgentID == "" || (!admin.IsValidAgentID(l.AgentID) && l.AgentID != "*") {
		srv.render(w, "limits_edit.html", limitsEditPage{
			Page: "limits", Session: sd, Limit: l, IsNew: true,
			Error: "Agent ID must be a valid agent ID or '*' for global default.",
		})
		return
	}

	if err := srv.store.UpsertAgentLimits(r.Context(), &l); err != nil {
		http.Error(w, "failed to save limits", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/limits", http.StatusSeeOther)
}

// handleLimitsEditGet renders GET /limits/{id}/edit.
func (srv *Server) handleLimitsEditGet(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	id := r.PathValue("id")

	limits, err := srv.store.ListAgentLimits(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load limits", http.StatusInternalServerError)
		return
	}
	for _, l := range limits {
		if l.ID == id {
			srv.render(w, "limits_edit.html", limitsEditPage{
				Page: "limits", Session: sd, Limit: l,
			})
			return
		}
	}
	http.NotFound(w, r)
}

// handleLimitsEditPost processes POST /limits/{id}/edit.
func (srv *Server) handleLimitsEditPost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	l := admin.AgentLimits{
		ID:              id,
		OrgID:           sd.OrgID,
		AgentID:         r.FormValue("agent_id"),
		MaxBodyBytes:    formInt(r, "max_body_bytes"),
		MaxSendPerMin:   formInt(r, "max_send_per_min"),
		MaxPendingTasks: formInt(r, "max_pending"),
		MaxStreamLen:    int64(formInt(r, "max_stream_len")),
		UpdatedAt:       time.Now().UTC(),
		UpdatedBy:       sd.UserID,
	}
	if l.AgentID == "" || (!admin.IsValidAgentID(l.AgentID) && l.AgentID != "*") {
		srv.render(w, "limits_edit.html", limitsEditPage{
			Page: "limits", Session: sd, Limit: l,
			Error: "Agent ID must be a valid agent ID or '*' for global default.",
		})
		return
	}

	if err := srv.store.UpsertAgentLimits(r.Context(), &l); err != nil {
		http.Error(w, "failed to update limits", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/limits", http.StatusSeeOther)
}

// handleLimitsDelete processes POST /limits/{id}/delete.
func (srv *Server) handleLimitsDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := srv.store.DeleteAgentLimits(r.Context(), id); err != nil {
		http.Error(w, "failed to delete limits", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/limits", http.StatusSeeOther)
}

// --- JSON API ----------------------------------------------------------------

// handleAPILimitsCheck returns effective limits for the requesting agent.
// Used by HTTPLimitEnforcer (Phase 3). Auth: Bearer token with role agent.
//
// GET /api/v1/limits/check?agent=<agentID>
func (srv *Server) handleAPILimitsCheck(w http.ResponseWriter, r *http.Request) {
	k := srv.apiKeyFromRequest(r)
	// SECURITY: this endpoint is a gateway-service call used by HTTPLimitEnforcer
	// (Phase 3). Require operator or admin — individual agent keys must not probe
	// the limits of other agents (CWE-285). Agents query their own limits via the
	// get_my_limits MCP tool instead.
	if k == nil || (k.Role != admin.RoleOperator && k.Role != admin.RoleAdmin) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("agent")
	if agentID == "" {
		http.Error(w, `{"error":"agent is required"}`, http.StatusBadRequest)
		return
	}
	if !admin.IsValidAgentID(agentID) {
		http.Error(w, `{"error":"invalid agent id"}`, http.StatusBadRequest)
		return
	}

	l, err := srv.store.GetEffectiveLimits(r.Context(), k.OrgID, agentID)
	if err != nil {
		http.Error(w, `{"error":"store error"}`, http.StatusInternalServerError)
		return
	}

	// l is nil when no agent-specific or global-default row exists;
	// return all-zero values (unlimited) rather than panicking. (CWE-476)
	var out struct {
		MaxBodyBytes    int   `json:"max_body_bytes"`
		MaxSendPerMin   int   `json:"max_send_per_min"`
		MaxPendingTasks int   `json:"max_pending_tasks"`
		MaxStreamLen    int64 `json:"max_stream_len"`
	}
	if l != nil {
		out.MaxBodyBytes = l.MaxBodyBytes
		out.MaxSendPerMin = l.MaxSendPerMin
		out.MaxPendingTasks = l.MaxPendingTasks
		out.MaxStreamLen = l.MaxStreamLen
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// formInt reads a non-negative integer form value.
// Returns 0 on parse failure or negative input (0 = unlimited). (CWE-20)
func formInt(r *http.Request, key string) int {
	v, _ := strconv.Atoi(r.FormValue(key))
	if v < 0 {
		return 0
	}
	return v
}
