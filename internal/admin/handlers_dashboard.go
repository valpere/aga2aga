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
	RecentAgents []admin.RegisteredAgent
}

func (srv *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	sd := sessionFromCtx(r)
	agents, _ := srv.store.ListAgents(r.Context(), sd.OrgID)
	policies, _ := srv.store.ListPolicies(r.Context(), sd.OrgID)

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

	// Recent = first 10 (already ordered by registered_at DESC from store)
	recent := agents
	if len(recent) > 10 {
		recent = recent[:10]
	}

	srv.render(w, "dashboard.html", dashboardPage{
		Page:         "dashboard",
		Session:      sd,
		Stats:        stats,
		RecentAgents: recent,
	})
}
