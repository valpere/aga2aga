package admin

import (
	"log"
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
	policies, err := srv.store.ListPolicies(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load policies", http.StatusInternalServerError)
		return
	}
	srv.render(w, "policies_list.html", policyListPage{
		Page:     "policies",
		Session:  sd,
		Policies: policies,
		CanEdit:  roleAtLeast(admin.Role(sd.Role), admin.RoleOperator),
	})
}

func (srv *Server) handlePolicyNewGet(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	agents, err := srv.store.ListAgents(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load agents", http.StatusInternalServerError)
		return
	}
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
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p,
			Agents: srv.listAgentsForForm(r, sd.OrgID), IsNew: true, Error: err.Error(),
		})
		return
	}
	p.ID = uuid.New().String()
	p.CreatedAt = time.Now().UTC()
	if err := srv.store.CreatePolicy(r.Context(), &p); err != nil {
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p,
			Agents: srv.listAgentsForForm(r, sd.OrgID), IsNew: true,
			Error: "Failed to save policy: " + err.Error(),
		})
		return
	}
	recordAudit(r.Context(), srv.store, sd, "policy.create", "policy", p.ID,
		"created policy "+p.SourceID+"→"+p.TargetID+" "+string(p.Action))
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
	srv.render(w, "policy_edit.html", policyEditPage{
		Page: "policies", Session: sd, Policy: *p,
		Agents: srv.listAgentsForForm(r, sd.OrgID), IsNew: false,
	})
}

func (srv *Server) handlePolicyEditPost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	id := r.PathValue("id")
	p, err := srv.policyFromForm(r, sd.OrgID, sd.UserID)
	if err != nil {
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p,
			Agents: srv.listAgentsForForm(r, sd.OrgID), IsNew: false, Error: err.Error(),
		})
		return
	}
	p.ID = id
	if err := srv.store.UpdatePolicy(r.Context(), &p); err != nil {
		srv.render(w, "policy_edit.html", policyEditPage{
			Page: "policies", Session: sd, Policy: p,
			Agents: srv.listAgentsForForm(r, sd.OrgID), IsNew: false,
			Error: "Failed to update policy: " + err.Error(),
		})
		return
	}
	recordAudit(r.Context(), srv.store, sd, "policy.update", "policy", p.ID,
		"updated policy "+p.SourceID+"→"+p.TargetID+" "+string(p.Action))
	http.Redirect(w, r, "/policies", http.StatusSeeOther)
}

func (srv *Server) handlePolicyDelete(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	id := r.PathValue("id")
	if err := srv.store.DeletePolicy(r.Context(), id); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	recordAudit(r.Context(), srv.store, sd, "policy.delete", "policy", id, "deleted policy "+id)
	http.Redirect(w, r, "/policies", http.StatusSeeOther)
}

// listAgentsForForm returns the agent list for populating policy form dropdowns.
// On store error it logs and returns nil (empty dropdown); the caller already has a
// form validation error to display and an empty dropdown is still usable.
func (srv *Server) listAgentsForForm(r *http.Request, orgID string) []admin.RegisteredAgent {
	agents, err := srv.store.ListAgents(r.Context(), orgID)
	if err != nil {
		log.Printf("listAgentsForForm: failed to load agents for org %q: %v", orgID, err)
	}
	return agents
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
