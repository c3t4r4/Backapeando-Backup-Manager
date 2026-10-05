package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/httpapi"
	"backapeando-backup-manager/internal/repository"
)

// TestRouterHealthIncludesVersion and TestRouterRetentionSweepNot404 guard
// against shipping a binary/router that omits deploy signals used in prod
// diagnostics (see docs/Infraestrutura.md: volume /app shadowing, health.version).
func TestRouterHealthIncludesVersion(t *testing.T) {
	router := newRouteSmokeRouter(t, "v1.0.7")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d, want 200", rr.Code)
	}
	body, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json: %v; body=%s", err, body)
	}
	if payload["status"] != "ok" {
		t.Fatalf("status = %q, want ok", payload["status"])
	}
	if payload["version"] != "v1.0.7" {
		t.Fatalf("version = %q, want v1.0.7 (body must always include version)", payload["version"])
	}
	if !strings.Contains(string(body), `"version"`) {
		t.Fatalf("body missing version key: %s", body)
	}
}

func TestRouterRetentionSweepNot404(t *testing.T) {
	router := newRouteSmokeRouter(t, "test")

	// CSRF is enforced before auth on mutating routes; matching cookie+header
	// lets the request reach RequireAuth, which must return 401 (route exists).
	token := "smoke-csrf-token"
	req := httptest.NewRequest(http.MethodPost, "/api/retention-sweep", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeaderName, token)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: token})

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Fatalf("POST /api/retention-sweep returned 404 — route missing from NewRouter")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/retention-sweep status = %d, want 401 (unauthenticated)", rr.Code)
	}
}

func newRouteSmokeRouter(t *testing.T, version string) http.Handler {
	t.Helper()
	return httpapi.NewRouter(httpapi.Deps{
		Repos: &repository.Repositories{},
		Sessions: &auth.SessionManager{
			CookieName:  "test_session",
			IdleTTL:     time.Hour,
			AbsoluteTTL: 24 * time.Hour,
			Secure:      false,
		},
		RateLimiter:   auth.NewLoginRateLimiter(100, time.Minute),
		SecureCookies: false,
		Version:       version,
	})
}
