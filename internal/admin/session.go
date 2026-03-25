package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

const sessionCookieName = "aga2aga_admin"

// sessionData holds the values stored in the encrypted session cookie.
type sessionData struct {
	UserID    string
	OrgID     string
	Username  string
	Role      string
	ExpiresAt time.Time
}

// Sessions manages encrypted cookie sessions.
type Sessions struct {
	sc *securecookie.SecureCookie
}

// NewSessions creates a Sessions instance using the provided hash and
// encryption keys. Both keys must be non-empty. Use
// securecookie.GenerateRandomKey(32) to generate suitable keys at startup.
func NewSessions(hashKey, blockKey []byte) *Sessions {
	return &Sessions{sc: securecookie.New(hashKey, blockKey)}
}

// Set writes sd into an encrypted cookie on w.
func (s *Sessions) Set(w http.ResponseWriter, sd sessionData) error {
	encoded, err := s.sc.Encode(sessionCookieName, sd)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  sd.ExpiresAt,
	})
	return nil
}

// Get decodes and returns the session from r's cookie. Returns an error if
// the cookie is missing, tampered, or expired.
func (s *Sessions) Get(r *http.Request) (sessionData, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return sessionData{}, errors.New("no session cookie")
	}
	var sd sessionData
	if err := s.sc.Decode(sessionCookieName, cookie.Value, &sd); err != nil {
		return sessionData{}, errors.New("invalid session")
	}
	if time.Now().After(sd.ExpiresAt) {
		return sessionData{}, errors.New("session expired")
	}
	return sd, nil
}

// Clear removes the session cookie.
func (s *Sessions) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    sessionCookieName,
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
}
