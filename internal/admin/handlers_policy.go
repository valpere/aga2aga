package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/aga2aga/pkg/admin"
)

type policyListPage struct {
	Page     string
	Session  sessionData
	Policies []admin.CommunicationPolicy
	CanEdit  bool
}

type policyEditPage struct {
	Page    string
	Session sessionData
	Policy  admin.CommunicationPolicy
	Agents  []admin.RegisteredAgent
	IsNew   bool
	Error   string
}

func (srv *Server) handlePolicyList(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	policies, _ := srv.store.ListPolicies(r.Context(), sd.OrgID)
	srv.render(w, "policies_list.html", policyListPage{
		Page:     "policies",
		Session:  sd,
		Policies: policies,
		CanEdit:  roleAtLeast(admin.Role(sd.Role), admin.RoleOperator),
	})
}

func (srv *Server) handlePolicyNewGet(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	agents, _ := srv.store.ListAgents(r.Context(), sd.OrgID)
	srv.render(w, "policy_edit.html", policyEditPage{
		Page:    "policies",
		Session: sd,
		Policy:  admin.CommunicationPolicy{SourceID: "*", TargetID: "*", Direction: admin.DirectionUnidirectional, Priority: 10},
		Agents:  agents,
		IsNew:   true,
	})
}

func (srv *Server) handlePolicyNewPost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	p, err := srv.policyFromForm(r, sd.OrgID, sd.UserID)
	if err != nil {
		agents, _ := srv.store.ListAgents(r.Context(), sd.OrgID)
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p, Agents: agents, IsNew: true, Error: err.Error(),
		})
		return
	}
	p.ID = uuid.New().String()
	p.CreatedAt = time.Now().UTC()
	if err := srv.store.CreatePolicy(r.Context(), &p); err != nil {
		agents, _ := srv.store.ListAgents(r.Context(), sd.OrgID)
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p, Agents: agents, IsNew: true,
			Error: "Failed to save policy: " + err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/policies", http.StatusSeeOther)
}

func (srv *Server) handlePolicyEditGet(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	id := r.PathValue("id")
	p, err := srv.store.GetPolicy(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	agents, _ := srv.store.ListAgents(r.Context(), sd.OrgID)
	srv.render(w, "policy_edit.html", policyEditPage{
		Page: "policies", Session: sd, Policy: *p, Agents: agents, IsNew: false,
	})
}

func (srv *Server) handlePolicyEditPost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	id := r.PathValue("id")
	p, err := srv.policyFromForm(r, sd.OrgID, sd.UserID)
	if err != nil {
		agents, _ := srv.store.ListAgents(r.Context(), sd.OrgID)
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p, Agents: agents, IsNew: false, Error: err.Error(),
		})
		return
	}
	p.ID = id
	if err := srv.store.UpdatePolicy(r.Context(), &p); err != nil {
		agents, _ := srv.store.ListAgents(r.Context(), sd.OrgID)
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p, Agents: agents, IsNew: false,
			Error: "Failed to update policy: " + err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/policies", http.StatusSeeOther)
}

func (srv *Server) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := srv.store.DeletePolicy(r.Context(), id); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/policies", http.StatusSeeOther)
}

// policyFromForm parses and validates the policy form fields.
func (srv *Server) policyFromForm(r *http.Request, orgID, userID string) (admin.CommunicationPolicy, error) {
	if err := r.ParseForm(); err != nil {
		return admin.CommunicationPolicy{}, err
	}
	priority, err := strconv.Atoi(r.FormValue("priority"))
	if err != nil {
		priority = 10
	}
	return admin.CommunicationPolicy{
		OrgID:     orgID,
		SourceID:  r.FormValue("source_id"),
		TargetID:  r.FormValue("target_id"),
		Direction: admin.PolicyDirection(r.FormValue("direction")),
		Action:    admin.PolicyAction(r.FormValue("action")),
		Priority:  priority,
		CreatedBy: userID,
	}, nil
}
