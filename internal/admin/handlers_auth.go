package admin

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type loginPage struct {
	Error    string
	Username string
}

func (srv *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	// Already logged in?
	if _, err := srv.sessions.Get(r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	srv.render(w, "login.html", loginPage{})
}

func (srv *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	u, err := srv.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		srv.render(w, "login.html", loginPage{Error: "Invalid username or password.", Username: username})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		srv.render(w, "login.html", loginPage{Error: "Invalid username or password.", Username: username})
		return
	}

	sd := sessionData{
		UserID:    u.ID,
		OrgID:     u.OrgID,
		Username:  u.Username,
		Role:      string(u.Role),
		ExpiresAt: time.Now().Add(8 * time.Hour),
	}
	if err := srv.sessions.Set(w, sd); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	recordAudit(r.Context(), srv.store, sd, "user.login", "session", u.ID, "logged in")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sd, err := srv.sessions.Get(r); err == nil {
		recordAudit(r.Context(), srv.store, sd, "user.logout", "session", sd.UserID, "logged out")
	}
	srv.sessions.Clear(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
