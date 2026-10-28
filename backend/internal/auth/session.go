package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

var ErrNoSession = errors.New("auth: no valid session")

type SessionManager struct {
	Sessions    *repository.AdminSessionRepo
	CookieName  string
	IdleTTL     time.Duration
	AbsoluteTTL time.Duration
	// Secure controls the cookie's Secure flag. Disable only for local
	// plain-HTTP development; production must always run behind TLS.
	Secure bool
}

// Issue creates a session row and sets the session cookie on the response.
// The cookie is HttpOnly + SameSite=Strict so it is never readable from JS
// and is not sent on cross-site requests — appropriate for a same-origin
// SPA that also holds SSH private keys and SAS tokens server-side.
// It returns the expiry it used so the caller can issue the paired CSRF
// cookie (auth.IssueCSRFToken) with the same Expires.
func (m *SessionManager) Issue(ctx context.Context, w http.ResponseWriter, userID, userAgent, ip string) (time.Time, error) {
	expiresAt := time.Now().Add(m.IdleTTL)
	session, err := m.Sessions.Create(ctx, userID, expiresAt, userAgent, ip)
	if err != nil {
		return time.Time{}, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     m.CookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	})
	return expiresAt, nil
}

// Authenticate reads the session cookie, validates it against the store,
// and slides the idle expiry forward — but never past the absolute TTL
// measured from session creation. It re-sends both the session cookie and
// the paired CSRF cookie with the new Expires: the DB row sliding forward
// via Extend is not enough on its own, since the browser enforces the
// Expires attribute it was given at Issue time regardless of what the
// server considers valid.
func (m *SessionManager) Authenticate(w http.ResponseWriter, r *http.Request) (domain.AdminSession, error) {
	cookie, err := r.Cookie(m.CookieName)
	if err != nil {
		return domain.AdminSession{}, ErrNoSession
	}

	session, err := m.Sessions.GetValid(r.Context(), cookie.Value)
	if errors.Is(err, repository.ErrNotFound) {
		return domain.AdminSession{}, ErrNoSession
	}
	if err != nil {
		return domain.AdminSession{}, err
	}

	absoluteCutoff := session.CreatedAt.Add(m.AbsoluteTTL)
	now := time.Now()
	if now.After(absoluteCutoff) {
		_ = m.Sessions.Delete(r.Context(), session.ID)
		return domain.AdminSession{}, ErrNoSession
	}

	newExpiry := now.Add(m.IdleTTL)
	if newExpiry.After(absoluteCutoff) {
		newExpiry = absoluteCutoff
	}
	_ = m.Sessions.Extend(r.Context(), session.ID, newExpiry)

	http.SetCookie(w, &http.Cookie{
		Name:     m.CookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  newExpiry,
	})
	RefreshCSRFCookie(w, r, m.Secure, newExpiry)

	return session, nil
}

// Clear deletes the session server-side (so it is immediately unusable, not
// just logically expired) and instructs the browser to drop both the
// session cookie and its paired CSRF cookie — leaving the CSRF cookie
// behind after logout wouldn't be exploitable on its own (it authenticates
// nothing without a live session), but there's no reason to leave it.
func (m *SessionManager) Clear(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(m.CookieName); err == nil {
		_ = m.Sessions.Delete(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     m.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   m.Secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
