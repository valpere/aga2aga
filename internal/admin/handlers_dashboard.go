package admin

import (
	"net/http"

	"github.com/valpere/aga2aga/pkg/admin"
)

type dashboardStats struct {
	AgentsActive    int
	AgentsSuspended int
	AgentsRevoked   int
	PoliciesAllow   int
	PoliciesDeny    int
}

type dashboardPage struct {
	Page         string
	Session      sessionData
	Stats        dashboardStats
	RecentEvents []admin.AuditEvent
}

func (srv *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	sd := sessionFromCtx(r)
	agents, err := srv.store.ListAgents(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load agents", http.StatusInternalServerError)
		return
	}
	policies, err := srv.store.ListPolicies(r.Context(), sd.OrgID)
	if err != nil {
		http.Error(w, "failed to load policies", http.StatusInternalServerError)
		return
	}

	var stats dashboardStats
	for _, a := range agents {
		switch a.Status {
		case admin.AgentStatusActive:
			stats.AgentsActive++
		case admin.AgentStatusSuspended:
			stats.AgentsSuspended++
		case admin.AgentStatusRevoked:
			stats.AgentsRevoked++
		}
	}
	for _, p := range policies {
		switch p.Action {
		case admin.PolicyActionAllow:
			stats.PoliciesAllow++
		case admin.PolicyActionDeny:
			stats.PoliciesDeny++
		}
	}

	recentEvents, err := srv.store.ListAuditEvents(r.Context(), sd.OrgID, 10)
	if err != nil {
		http.Error(w, "failed to load audit events", http.StatusInternalServerError)
		return
	}

	srv.render(w, "dashboard.html", dashboardPage{
		Page:         "dashboard",
		Session:      sd,
		Stats:        stats,
		RecentEvents: recentEvents,
	})
}
