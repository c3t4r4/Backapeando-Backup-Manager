package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIssueCSRFTokenSetsExpiry(t *testing.T) {
	rec := httptest.NewRecorder()
	expiresAt := time.Now().Add(12 * time.Hour).Truncate(time.Second)

	token, err := IssueCSRFToken(rec, true, expiresAt)
	if err != nil {
		t.Fatalf("IssueCSRFToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	cookie := findCookie(t, rec, CSRFCookieName)
	if cookie.Value != token {
		t.Fatalf("cookie value = %q, want %q", cookie.Value, token)
	}
	if !cookie.Expires.Equal(expiresAt) {
		t.Fatalf("cookie Expires = %v, want %v (persistent, matching the session cookie)", cookie.Expires, expiresAt)
	}
	if cookie.HttpOnly {
		t.Fatal("CSRF cookie must not be HttpOnly — the SPA needs to read it")
	}
}

func TestRefreshCSRFCookieSlidesExpiryWithoutRotatingValue(t *testing.T) {
	const existingToken = "existing-token-value"
	req := httptest.NewRequest(http.MethodPost, "/api/retention-sweep", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: existingToken})

	rec := httptest.NewRecorder()
	newExpiry := time.Now().Add(24 * time.Hour).Truncate(time.Second)

	RefreshCSRFCookie(rec, req, true, newExpiry)

	cookie := findCookie(t, rec, CSRFCookieName)
	if cookie.Value != existingToken {
		t.Fatalf("expected value to stay %q (no rotation), got %q", existingToken, cookie.Value)
	}
	if !cookie.Expires.Equal(newExpiry) {
		t.Fatalf("cookie Expires = %v, want %v", cookie.Expires, newExpiry)
	}
}

func TestRefreshCSRFCookieNoopsWhenCookieMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/retention-sweep", nil)
	rec := httptest.NewRecorder()

	RefreshCSRFCookie(rec, req, true, time.Now().Add(time.Hour))

	for _, c := range rec.Result().Cookies() {
		if c.Name == CSRFCookieName {
			t.Fatalf("expected no CSRF cookie to be set, got one: %+v", c)
		}
	}
}

func findCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %q not set", name)
	return nil
}
