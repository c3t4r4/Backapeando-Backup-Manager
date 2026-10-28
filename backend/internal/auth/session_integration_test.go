package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/repository"
)

// TestAuthenticateSlidesSessionAndCSRFCookiesTogether is a regression test
// for the "csrf token missing or invalid" bug reported after a user stayed
// logged in for a while: the session cookie's Expires was frozen at login
// time while the DB-backed session kept sliding forward via Extend, and the
// CSRF cookie had no Expires at all (browser-session-only), so it could be
// wiped by a browser/OS restart while the persistent session cookie
// survived. Authenticate must now re-send both cookies with a pushed-forward
// Expires on every authenticated request.
func TestAuthenticateSlidesSessionAndCSRFCookiesTogether(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	pool, err := repository.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	repos := repository.New(pool)

	manager := &auth.SessionManager{
		Sessions:    repos.AdminSessions,
		CookieName:  "backapeando_backup_session",
		IdleTTL:     time.Hour,
		AbsoluteTTL: 24 * time.Hour,
		Secure:      true,
	}

	userID := uuid.New().String()
	loginRec := httptest.NewRecorder()
	expiresAt, err := manager.Issue(ctx, loginRec, userID, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	t.Cleanup(func() {
		for _, c := range loginRec.Result().Cookies() {
			if c.Name == manager.CookieName {
				_ = repos.AdminSessions.Delete(ctx, c.Value)
			}
		}
	})

	sessionCookie := findCookieFrom(t, loginRec, manager.CookieName)
	csrfToken := "frontend-held-csrf-value"

	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	req.AddCookie(sessionCookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrfToken})

	rec := httptest.NewRecorder()
	session, err := manager.Authenticate(rec, req)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if session.UserID != userID {
		t.Fatalf("UserID = %q, want %q", session.UserID, userID)
	}

	refreshedSession := findCookieFrom(t, rec, manager.CookieName)
	if refreshedSession.Value != sessionCookie.Value {
		t.Fatalf("session cookie value changed: got %q, want %q", refreshedSession.Value, sessionCookie.Value)
	}
	if !refreshedSession.Expires.After(expiresAt) {
		t.Fatalf("session cookie Expires did not slide forward: got %v, original was %v", refreshedSession.Expires, expiresAt)
	}

	refreshedCSRF := findCookieFrom(t, rec, auth.CSRFCookieName)
	if refreshedCSRF.Value != csrfToken {
		t.Fatalf("CSRF cookie value changed (should not rotate): got %q, want %q", refreshedCSRF.Value, csrfToken)
	}
	if !refreshedCSRF.Expires.Equal(refreshedSession.Expires) {
		t.Fatalf("CSRF cookie Expires (%v) does not match session cookie Expires (%v)", refreshedCSRF.Expires, refreshedSession.Expires)
	}
}

func findCookieFrom(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %q not set", name)
	return nil
}
