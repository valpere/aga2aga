package admin

import (
	"log"
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

type profilePage struct {
	Page    string
	Session sessionData
	Success string
	Error   string
}

func (srv *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	srv.render(w, "profile.html", profilePage{Page: "profile", Session: sd})
}

func (srv *Server) handleProfilePost(w http.ResponseWriter, r *http.Request) {
	sd := sessionFromCtx(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	current := r.FormValue("current_password")
	newPw := r.FormValue("new_password")
	confirm := r.FormValue("confirm_password")

	renderErr := func(msg string) {
		srv.render(w, "profile.html", profilePage{Page: "profile", Session: sd, Error: msg})
	}

	const minPasswordLen = 8
	const maxPasswordLen = 72 // bcrypt silently truncates beyond 72 bytes
	if len(newPw) < minPasswordLen {
		renderErr("Password must be at least 8 characters.")
		return
	}
	if len(newPw) > maxPasswordLen {
		renderErr("Password must not exceed 72 characters.")
		return
	}
	if newPw != confirm {
		renderErr("New password and confirmation do not match.")
		return
	}

	u, err := srv.store.GetUserByID(r.Context(), sd.UserID)
	if err != nil {
		renderErr("Could not load user.")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(current)); err != nil {
		renderErr("Current password is incorrect.")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := srv.store.UpdateUserPassword(r.Context(), sd.UserID, string(hashed)); err != nil {
		log.Printf("handleProfilePost: UpdateUserPassword for user %q: %v", sd.UserID, err)
		http.Error(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	recordAudit(r.Context(), srv.store, sd, "user.password_change", "user", sd.UserID, "changed password")
	srv.render(w, "profile.html", profilePage{Page: "profile", Session: sd, Success: "Password changed successfully."})
}

func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sd, err := srv.sessions.Get(r); err == nil {
		recordAudit(r.Context(), srv.store, sd, "user.logout", "session", sd.UserID, "logged out")
	}
	srv.sessions.Clear(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
