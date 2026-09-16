// Package middleware provides the small, hand-rolled HTTP middleware chain
// used by the API: request logging, panic recovery, session auth, and CSRF
// enforcement. At ~5 middlewares total, a framework's middleware system
// would add more ceremony than it saves.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/domain"
)

type ctxKey int

const sessionCtxKey ctxKey = iota

func WithSession(ctx context.Context, s domain.AdminSession) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
}

// SessionFromContext returns the authenticated session, if RequireAuth ran
// for this request. The bool mirrors the map "comma ok" idiom.
func SessionFromContext(ctx context.Context) (domain.AdminSession, bool) {
	s, ok := ctx.Value(sessionCtxKey).(domain.AdminSession)
	return s, ok
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic_recovered", "error", rec, "path", r.URL.Path)
				// Never leak the panic value/stack to the client — it may
				// contain internal details (query fragments, file paths).
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequireAuth rejects the request unless it carries a valid session cookie,
// and stashes the session in the request context for handlers to read.
func RequireAuth(sessions *auth.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := sessions.Authenticate(r)
			if err != nil {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithSession(r.Context(), session)))
		})
	}
}

// RequireCSRF enforces the double-submit CSRF token on state-changing
// methods. Safe methods (GET/HEAD/OPTIONS) are exempt since they must not
// mutate state per HTTP semantics.
func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if err := auth.ValidateCSRF(r); err != nil {
			http.Error(w, `{"error":"csrf token missing or invalid"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sets a few defensive headers on every response. This API
// only ever serves JSON today, so the risk these mitigate is currently low,
// but the system will hold SSH private keys and Azure SAS tokens — cheap
// hardening now costs nothing and reduces blast radius if the separate SPA
// frontend (served from a different container) ever has an XSS bug.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		// Browsers ignore this header entirely when the response wasn't
		// received over HTTPS, so it's safe to send unconditionally even
		// in a plain-HTTP local dev setup.
		h.Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// CORS enables cross-origin requests from exactly one configured origin,
// with credentials (cookies). It is opt-in: when allowedOrigin is empty
// (the default), this middleware is a pure no-op — the correct setting
// whenever the frontend and API are served same-origin, which is how
// every docker-compose setup in this project works (Nginx/Traefik proxy
// `/api/` to the backend). Only set CORS_ALLOWED_ORIGIN for a genuinely
// cross-origin deployment (e.g. `npm run dev` against a remote API).
//
// A wildcard origin ("*") is deliberately not supported: browsers reject
// `Access-Control-Allow-Origin: *` combined with
// `Access-Control-Allow-Credentials: true`, and this API always needs
// credentials (the session + CSRF cookies) — so the only valid
// configuration is one exact origin, echoed back verbatim.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if allowedOrigin == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", allowedOrigin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Vary", "Origin")

			if r.Method == http.MethodOptions {
				// Preflight requests carry no cookies/CSRF token — they
				// must be answered here, before RequireAuth/RequireCSRF
				// would otherwise reject them.
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Chain applies middlewares in the given order (first listed runs outermost).
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
