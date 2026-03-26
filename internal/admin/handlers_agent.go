package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

type agentListPage struct {
	Page         string
	Session      sessionData
	Agents       []admin.RegisteredAgent
	FilterStatus string
}

type agentDetailPage struct {
	Page     string
	Session  sessionData
	Agent    *admin.RegisteredAgent
	Policies []admin.CommunicationPolicy
}

type agentNewPage struct {
	Page        string
	Session     sessionData
	Error       string
	AgentID     string
	DisplayName string
}

func (srv *Server) handleAgentList(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	all, err := srv.store.ListAgents(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load agents", http.StatusInternalServerError)
		return
	}

	filter := r.URL.Query().Get("status")
	var agents []admin.RegisteredAgent
	for _, a := range all {
		if filter == "" || string(a.Status) == filter {
			agents = append(agents, a)
		}
	}
	srv.render(w, "agents_list.html", agentListPage{
		Page: "agents", Session: sd, Agents: agents, FilterStatus: filter,
	})
}

func (srv *Server) handleAgentNewGet(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	srv.render(w, "agent_new.html", agentNewPage{Page: "agents", Session: sd})
}

func (srv *Server) handleAgentNewPost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	agentID := r.FormValue("agent_id")
	displayName := r.FormValue("display_name")

	if agentID == "" {
		srv.render(w, "agent_new.html", agentNewPage{
			Page: "agents", Session: sd,
			Error: "Agent ID is required.", DisplayName: displayName,
		})
		return
	}

	// Check for duplicate within org
	if existing, err := srv.store.GetAgent(r.Context(), sd.OrgID, agentID); err == nil && existing != nil {
		srv.render(w, "agent_new.html", agentNewPage{
			Page: "agents", Session: sd,
			Error:       fmt.Sprintf("Agent %q is already registered.", agentID),
			AgentID:     agentID,
			DisplayName: displayName,
		})
		return
	}

	a := &admin.RegisteredAgent{
		ID:           uuid.New().String(),
		OrgID:        sd.OrgID,
		AgentID:      agentID,
		DisplayName:  displayName,
		Status:       admin.AgentStatusActive,
		RegisteredBy: sd.UserID,
		RegisteredAt: time.Now().UTC(),
	}
	if err := srv.store.RegisterAgent(r.Context(), a); err != nil {
		srv.render(w, "agent_new.html", agentNewPage{
			Page: "agents", Session: sd,
			Error:   "Failed to register agent: " + err.Error(),
			AgentID: agentID, DisplayName: displayName,
		})
		return
	}
	recordAudit(r.Context(), srv.store, sd, "agent.register", "agent", agentID,
		"registered "+agentID)
	http.Redirect(w, r, "/agents", http.StatusSeeOther)
}

func (srv *Server) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	agentID := r.PathValue("id")

	a, err := srv.store.GetAgent(r.Context(), sd.OrgID, agentID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	all, err := srv.store.ListPolicies(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load policies", http.StatusInternalServerError)
		return
	}
	var related []admin.CommunicationPolicy
	for _, p := range all {
		if p.SourceID == agentID || p.TargetID == agentID ||
			p.SourceID == admin.Wildcard || p.TargetID == admin.Wildcard {
			related = append(related, p)
		}
	}
	srv.render(w, "agent_detail.html", agentDetailPage{
		Page: "agents", Session: sd, Agent: a, Policies: related,
	})
}

func (srv *Server) handleAgentSuspend(w http.ResponseWriter, r *http.Request) {
	srv.setAgentStatus(w, r, admin.AgentStatusSuspended)
}

func (srv *Server) handleAgentActivate(w http.ResponseWriter, r *http.Request) {
	srv.setAgentStatus(w, r, admin.AgentStatusActive)
}

func (srv *Server) handleAgentRevoke(w http.ResponseWriter, r *http.Request) {
	srv.setAgentStatus(w, r, admin.AgentStatusRevoked)
}

func (srv *Server) setAgentStatus(w http.ResponseWriter, r *http.Request, status admin.AgentStatus) {
	sd := sessionFromCtx(r)
	agentID := r.PathValue("id")
	if err := srv.store.UpdateAgentStatus(r.Context(), sd.OrgID, agentID, status); err != nil {
		http.Error(w, "update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	recordAudit(r.Context(), srv.store, sd, "agent."+string(status), "agent", agentID,
		string(status)+" "+agentID)
	http.Redirect(w, r, "/agents/"+agentID, http.StatusSeeOther)
}
