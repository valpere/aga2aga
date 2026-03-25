package admin

import (
	"context"
	"net/http"

	"github.com/valpere/aga2aga/pkg/admin"
)

type contextKey int

const (
	ctxSession contextKey = iota
)

// requireAuth is middleware that redirects unauthenticated requests to /login.
func (srv *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sd, err := srv.sessions.Get(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), ctxSession, sd)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole is middleware that returns 403 if the session user's role does
// not meet the minimum required role.
func requireRole(min admin.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sd := sessionFromCtx(r)
		if !roleAtLeast(admin.Role(sd.Role), min) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// sessionFromCtx extracts sessionData from the request context. It panics if
// called outside of requireAuth middleware.
func sessionFromCtx(r *http.Request) sessionData {
	return r.Context().Value(ctxSession).(sessionData)
}

// roleAtLeast returns true if role meets or exceeds the minimum required role.
func roleAtLeast(role, min admin.Role) bool {
	order := map[admin.Role]int{
		admin.RoleViewer:   1,
		admin.RoleOperator: 2,
		admin.RoleAdmin:    3,
	}
	return order[role] >= order[min]
}
