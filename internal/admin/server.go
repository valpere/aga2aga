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
	tmpls    *template.Template
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
	tmpls, err := template.New("").Funcs(funcMap).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		store:    store,
		sessions: NewSessions(hashKey, blockKey),
		tmpls:    tmpls,
	}, nil
}

// Handler returns an http.Handler with all admin routes registered.
func (srv *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static assets
	mux.Handle("GET /static/", http.FileServer(http.FS(assets)))

	// Public routes
	mux.HandleFunc("GET /login",  srv.handleLoginGet)
	mux.HandleFunc("POST /login", srv.handleLoginPost)
	mux.HandleFunc("POST /logout", srv.handleLogout)

	// Protected routes (require valid session)
	protected := srv.requireAuth

	mux.Handle("GET /",              protected(http.HandlerFunc(srv.handleDashboard)))
	mux.Handle("GET /agents",        protected(http.HandlerFunc(srv.handleAgentList)))
	mux.Handle("GET /agents/new",    protected(http.HandlerFunc(srv.handleAgentNewGet)))
	mux.Handle("POST /agents/new",   protected(requireRole(admin.RoleOperator, srv.handleAgentNewPost)))
	mux.Handle("GET /agents/{id}",   protected(http.HandlerFunc(srv.handleAgentDetail)))
	mux.Handle("POST /agents/{id}/suspend",  protected(requireRole(admin.RoleAdmin, srv.handleAgentSuspend)))
	mux.Handle("POST /agents/{id}/activate", protected(requireRole(admin.RoleAdmin, srv.handleAgentActivate)))
	mux.Handle("POST /agents/{id}/revoke",   protected(requireRole(admin.RoleAdmin, srv.handleAgentRevoke)))

	mux.Handle("GET /policies",           protected(http.HandlerFunc(srv.handlePolicyList)))
	mux.Handle("GET /policies/new",       protected(http.HandlerFunc(srv.handlePolicyNewGet)))
	mux.Handle("POST /policies/new",      protected(requireRole(admin.RoleOperator, srv.handlePolicyNewPost)))
	mux.Handle("GET /policies/{id}/edit", protected(http.HandlerFunc(srv.handlePolicyEditGet)))
	mux.Handle("POST /policies/{id}/edit",   protected(requireRole(admin.RoleOperator, srv.handlePolicyEditPost)))
	mux.Handle("POST /policies/{id}/delete", protected(requireRole(admin.RoleOperator, srv.handlePolicyDelete)))

	return mux
}

// render executes the named template with data, writing to w.
func (srv *Server) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := srv.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
