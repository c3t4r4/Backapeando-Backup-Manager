package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"time"
)

const CSRFCookieName = "backapeando_backup_csrf"
const CSRFHeaderName = "X-CSRF-Token"

var ErrCSRFMismatch = errors.New("auth: csrf token missing or mismatched")

// IssueCSRFToken generates a fresh token and sets it as a readable (non
// HttpOnly) cookie, so the SPA's JS can read it and echo it back in the
// X-CSRF-Token header on mutating requests (double-submit cookie pattern).
// It complements — not replaces — the session cookie's SameSite=Strict,
// which already blocks cross-site delivery of the session cookie itself.
//
// expiresAt must match the session cookie's expiry: without an explicit
// expiry this cookie is browser-session-only (memory-only) while the
// session cookie is persistent (saved to disk), so a browser/OS restart
// would wipe the CSRF cookie while leaving a still-valid session cookie
// behind — the exact cause of a stale-but-authenticated tab suddenly
// failing every mutating request with "csrf token missing or invalid".
func IssueCSRFToken(w http.ResponseWriter, secure bool, expiresAt time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	})
	return token, nil
}

// RefreshCSRFCookie re-sends the CSRF cookie with the same value but a
// pushed-forward Expires, keeping it in lockstep with the session's sliding
// idle expiry. It does not rotate the token value, so concurrent in-flight
// requests reading the cookie before this response lands still match. If
// the request carries no CSRF cookie, it does nothing — RequireCSRF will
// still reject the next mutating request in that (now rare, pre-fix-era)
// case.
func RefreshCSRFCookie(w http.ResponseWriter, r *http.Request, secure bool, expiresAt time.Time) {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    cookie.Value,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	})
}

// ValidateCSRF checks that the header value matches the cookie value.
// An attacker on a different origin cannot read the cookie (same-origin
// policy), so they cannot produce a matching header even though the
// browser would attach the cookie automatically.
func ValidateCSRF(r *http.Request) error {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil || cookie.Value == "" {
		return ErrCSRFMismatch
	}
	header := r.Header.Get(CSRFHeaderName)
	if header == "" {
		return ErrCSRFMismatch
	}
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		return ErrCSRFMismatch
	}
	return nil
}
