package admin

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/valpere/aga2aga/pkg/admin"
)

//go:embed templates/* static/*
var assets embed.FS

// Server is the HTTP admin server. It holds all shared dependencies and
// registers all routes on a single *http.ServeMux.
type Server struct {
	store    admin.Store
	sessions *Sessions
	funcMap  template.FuncMap
}

// NewServer constructs a Server, parsing all embedded templates.
// hashKey and blockKey are used for cookie signing/encryption.
func NewServer(store admin.Store, hashKey, blockKey []byte) (*Server, error) {
	// Register a "string" function so templates can call `(string .Field)` to
	// convert custom string types to plain string for comparison.
	funcMap := template.FuncMap{
		"string": func(v interface{}) string {
			switch s := v.(type) {
			case string:
				return s
			case admin.AgentStatus:
				return string(s)
			case admin.PolicyAction:
				return string(s)
			case admin.PolicyDirection:
				return string(s)
			case admin.Role:
				return string(s)
			default:
				return ""
			}
		},
	}
	return &Server{
		store:    store,
		sessions: NewSessions(hashKey, blockKey),
		funcMap:  funcMap,
	}, nil
}

// Handler returns an http.Handler with all admin routes registered.
func (srv *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static assets
	mux.Handle("GET /static/", http.FileServer(http.FS(assets)))

	// Public routes
	mux.HandleFunc("GET /login", srv.handleLoginGet)
	mux.HandleFunc("POST /login", srv.handleLoginPost)
	mux.HandleFunc("POST /logout", srv.handleLogout)

	// Protected routes (require valid session)
	protected := srv.requireAuth

	mux.Handle("GET /profile", protected(http.HandlerFunc(srv.handleProfileGet)))
	mux.Handle("POST /profile", protected(http.HandlerFunc(srv.handleProfilePost)))

	mux.Handle("GET /", protected(http.HandlerFunc(srv.handleDashboard)))
	mux.Handle("GET /agents", protected(http.HandlerFunc(srv.handleAgentList)))
	mux.Handle("GET /agents/new", protected(http.HandlerFunc(srv.handleAgentNewGet)))
	mux.Handle("POST /agents/new", protected(requireRole(admin.RoleOperator, srv.handleAgentNewPost)))
	mux.Handle("GET /agents/{id}", protected(http.HandlerFunc(srv.handleAgentDetail)))
	mux.Handle("POST /agents/{id}/suspend", protected(requireRole(admin.RoleAdmin, srv.handleAgentSuspend)))
	mux.Handle("POST /agents/{id}/activate", protected(requireRole(admin.RoleAdmin, srv.handleAgentActivate)))
	mux.Handle("POST /agents/{id}/revoke", protected(requireRole(admin.RoleAdmin, srv.handleAgentRevoke)))

	mux.Handle("GET /policies", protected(http.HandlerFunc(srv.handlePolicyList)))
	mux.Handle("GET /policies/new", protected(http.HandlerFunc(srv.handlePolicyNewGet)))
	mux.Handle("POST /policies/new", protected(requireRole(admin.RoleOperator, srv.handlePolicyNewPost)))
	mux.Handle("GET /policies/{id}/edit", protected(http.HandlerFunc(srv.handlePolicyEditGet)))
	mux.Handle("POST /policies/{id}/edit", protected(requireRole(admin.RoleOperator, srv.handlePolicyEditPost)))
	mux.Handle("POST /policies/{id}/delete", protected(requireRole(admin.RoleOperator, srv.handlePolicyDelete)))

	mux.Handle("GET /audit", protected(http.HandlerFunc(srv.handleAuditList)))

	mux.Handle("GET /messages", protected(http.HandlerFunc(srv.handleMessageLogList)))

	mux.Handle("GET /limits", protected(http.HandlerFunc(srv.handleLimitsList)))
	mux.Handle("GET /limits/new", protected(http.HandlerFunc(srv.handleLimitsNewGet)))
	mux.Handle("POST /limits/new", protected(requireRole(admin.RoleOperator, srv.handleLimitsNewPost)))
	mux.Handle("GET /limits/{id}/edit", protected(http.HandlerFunc(srv.handleLimitsEditGet)))
	mux.Handle("POST /limits/{id}/edit", protected(requireRole(admin.RoleOperator, srv.handleLimitsEditPost)))
	mux.Handle("POST /limits/{id}/delete", protected(requireRole(admin.RoleOperator, srv.handleLimitsDelete)))

	mux.Handle("GET /api-keys", protected(requireRole(admin.RoleAdmin, srv.handleAPIKeyList)))
	mux.Handle("POST /api-keys/new", protected(requireRole(admin.RoleAdmin, srv.handleAPIKeyNewPost)))
	mux.Handle("POST /api-keys/{id}/revoke", protected(requireRole(admin.RoleAdmin, srv.handleAPIKeyRevoke)))

	// JSON API — authenticated by Bearer token (API key), not session cookie
	mux.HandleFunc("GET /api/v1/evaluate", srv.handleAPIEvaluate)
	mux.HandleFunc("POST /api/v1/auth", srv.handleAPIAuth)
	mux.HandleFunc("POST /api/v1/message-log", srv.handleAPIMessageLog)
	mux.HandleFunc("GET /api/v1/limits/check", srv.handleAPILimitsCheck)

	return mux
}

// render executes the named page template inside the shared layout.
//
// Templates are parsed per-request (layout.html + the named page template)
// so that {{define "title"}} and {{define "content"}} blocks from different
// pages do not overwrite each other in a shared namespace — a consequence of
// how Go's html/template merges all {{define}} blocks when templates are
// parsed together with ParseFS("templates/*.html").
func (srv *Server) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// login.html is a self-contained page with no layout wrapper.
	// All other pages define {{define "content"}} and are rendered inside layout.html.
	var (
		tmpl *template.Template
		err  error
	)
	if name == "login.html" {
		tmpl, err = template.New(name).Funcs(srv.funcMap).ParseFS(assets, "templates/"+name)
	} else {
		tmpl, err = template.New("layout.html").Funcs(srv.funcMap).ParseFS(assets, "templates/layout.html", "templates/"+name)
	}
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
