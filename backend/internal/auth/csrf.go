package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
)

const CSRFCookieName = "backapeando_backup_csrf"
const CSRFHeaderName = "X-CSRF-Token"

var ErrCSRFMismatch = errors.New("auth: csrf token missing or mismatched")

// IssueCSRFToken generates a fresh token and sets it as a readable (non
// HttpOnly) cookie, so the SPA's JS can read it and echo it back in the
// X-CSRF-Token header on mutating requests (double-submit cookie pattern).
// It complements — not replaces — the session cookie's SameSite=Strict,
// which already blocks cross-site delivery of the session cookie itself.
func IssueCSRFToken(w http.ResponseWriter, secure bool) (string, error) {
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
	})
	return token, nil
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
