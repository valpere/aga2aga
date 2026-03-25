package admin

import (
	"net/http"

	"github.com/valpere/aga2aga/pkg/admin"
)

type auditPage struct {
	Page   string
	Session sessionData
	Events []admin.AuditEvent
	Filter string
}

func (srv *Server) handleAuditList(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	filter := r.URL.Query().Get("action")

	events, err := srv.store.ListAuditEvents(r.Context(), sd.OrgID, 200)
	if err != nil {
		http.Error(w, "failed to load audit log", http.StatusInternalServerError)
		return
	}

	if filter != "" {
		var filtered []admin.AuditEvent
		for _, e := range events {
			if len(e.Action) >= len(filter) && e.Action[:len(filter)] == filter {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	srv.render(w, "audit.html", auditPage{
		Page: "audit", Session: sd, Events: events, Filter: filter,
	})
}
