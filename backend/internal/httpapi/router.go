// Package httpapi wires the HTTP routes for the API process: session-based
// admin auth, CRUD over servers / storage targets / retention policies, SSH
// key generation + test-connection (Fase 2), and the synchronous
// backup-now + backup history endpoints (Fase 3). The automatic scheduler
// (cmd/worker) is added in Fase 4 on top of the same packages.
package httpapi

import (
	"log/slog"
	"net/http"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/httpapi/handlers"
	"backapeando-backup-manager/internal/httpapi/middleware"
	"backapeando-backup-manager/internal/repository"
)

type Deps struct {
	Repos             *repository.Repositories
	Sessions          *auth.SessionManager
	Sealer            *crypto.Sealer
	RateLimiter       auth.RateLimiter
	SecureCookies     bool
	CORSAllowedOrigin string
}

func NewRouter(deps Deps) http.Handler {
	authHandlers := &handlers.AuthHandlers{
		Users: deps.Repos.AdminUsers, Sessions: deps.Sessions,
		RateLimiter: deps.RateLimiter, SecureCookies: deps.SecureCookies,
	}
	serverHandlers := &handlers.ServerHandlers{Servers: deps.Repos.Servers, StorageTargets: deps.Repos.StorageTargets, Sealer: deps.Sealer, Logger: slog.Default()}
	storageTargetHandlers := &handlers.StorageTargetHandlers{Targets: deps.Repos.StorageTargets, Sealer: deps.Sealer}
	retentionHandlers := &handlers.RetentionPolicyHandlers{Policies: deps.Repos.RetentionPolicies}
	backupHandlers := &handlers.BackupHandlers{
		Servers:            deps.Repos.Servers,
		StorageTargets:     deps.Repos.StorageTargets,
		RetentionPolicies:  deps.Repos.RetentionPolicies,
		BackupRuns:         deps.Repos.BackupRuns,
		RetentionDeletions: deps.Repos.RetentionDeletions,
		Sealer:             deps.Sealer,
		Logger:             slog.Default(),
	}
	runNowHandlers := &handlers.RunNowHandlers{
		BackupRuns: deps.Repos.BackupRuns,
		Servers:    deps.Repos.Servers,
		Logger:     slog.Default(),
	}
	dashboardHandlers := &handlers.DashboardHandlers{Servers: deps.Repos.Servers, BackupRuns: deps.Repos.BackupRuns, StorageTargets: deps.Repos.StorageTargets}
	adminUserHandlers := &handlers.AdminUserHandlers{Users: deps.Repos.AdminUsers}

	mux := http.NewServeMux()

	// Health check endpoints (both for Docker health and Traefik liveness probes)
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /api/health", healthHandler)

	mux.HandleFunc("POST /api/auth/login", authHandlers.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandlers.Logout)
	mux.HandleFunc("GET /api/auth/me", authHandlers.Me)

	mux.HandleFunc("GET /api/servers", serverHandlers.List)
	mux.HandleFunc("POST /api/servers", serverHandlers.Create)
	mux.HandleFunc("GET /api/servers/{id}", serverHandlers.Get)
	mux.HandleFunc("PUT /api/servers/{id}", serverHandlers.Update)
	mux.HandleFunc("DELETE /api/servers/{id}", serverHandlers.Delete)
	mux.HandleFunc("POST /api/servers/{id}/test-connection", serverHandlers.TestConnection)
	mux.HandleFunc("POST /api/servers/{id}/regenerate-key", serverHandlers.RegenerateKey)
	mux.HandleFunc("POST /api/servers/{id}/reset-host-key", serverHandlers.ResetHostKey)
	mux.HandleFunc("POST /api/servers/{id}/enable", serverHandlers.Enable)
	mux.HandleFunc("POST /api/servers/{id}/disable", serverHandlers.Disable)
	mux.HandleFunc("GET /api/servers/{id}/retention-policy", retentionHandlers.GetForServer)
	mux.HandleFunc("PUT /api/servers/{id}/retention-policy", retentionHandlers.UpsertForServer)
	mux.HandleFunc("DELETE /api/servers/{id}/retention-policy", retentionHandlers.DeleteForServer)
	mux.HandleFunc("POST /api/servers/{id}/backup-now", backupHandlers.BackupNow)
	mux.HandleFunc("POST /api/servers/{id}/run-now", runNowHandlers.RunServerBackupNow)
	mux.HandleFunc("GET /api/servers/{id}/backup-runs", backupHandlers.ListBackupRuns)
	mux.HandleFunc("GET /api/backup-runs", backupHandlers.ListAllBackupRuns)

	mux.HandleFunc("GET /api/storage-targets", storageTargetHandlers.List)
	mux.HandleFunc("POST /api/storage-targets", storageTargetHandlers.Create)
	mux.HandleFunc("GET /api/storage-targets/{id}", storageTargetHandlers.Get)
	mux.HandleFunc("PUT /api/storage-targets/{id}", storageTargetHandlers.Update)
	mux.HandleFunc("DELETE /api/storage-targets/{id}", storageTargetHandlers.Delete)

	mux.HandleFunc("GET /api/retention-policy/default", retentionHandlers.GetGlobal)
	mux.HandleFunc("PUT /api/retention-policy/default", retentionHandlers.UpdateGlobal)

	mux.HandleFunc("GET /api/dashboard/summary", dashboardHandlers.Summary)
	mux.HandleFunc("GET /api/dashboard/backup-stats", dashboardHandlers.BackupStats)

	mux.HandleFunc("GET /api/admin-users", adminUserHandlers.List)
	mux.HandleFunc("POST /api/admin-users", adminUserHandlers.Create)
	mux.HandleFunc("GET /api/admin-users/{id}", adminUserHandlers.Get)
	mux.HandleFunc("PUT /api/admin-users/{id}", adminUserHandlers.Update)
	mux.HandleFunc("DELETE /api/admin-users/{id}", adminUserHandlers.Delete)

	// Every /api/ route except auth/login and healthz requires a valid
	// session; mutating requests additionally require a matching CSRF
	// token. Login itself is exempt from the CSRF check — the CSRF cookie
	// doesn't exist yet until a session is issued, so there is nothing to
	// double-submit at that point (the login credentials themselves are
	// the proof of intent).
	protected := middleware.Chain(mux,
		skipForPublicPaths(middleware.RequireCSRF),
		skipForPublicPaths(middleware.RequireAuth(deps.Sessions)),
	)

	// CORS runs outermost so an OPTIONS preflight (which carries no
	// cookies/CSRF token) is answered before it ever reaches
	// RequireAuth/RequireCSRF below.
	return middleware.Chain(protected,
		middleware.CORS(deps.CORSAllowedOrigin),
		middleware.Recover,
		middleware.Logging,
		middleware.SecurityHeaders,
	)
}

var publicPaths = map[string]bool{
	"/healthz":        true,
	"/api/health":     true,
	"/api/auth/login": true,
}

// skipForPublicPaths lets health checks and login bypass whatever
// middleware it wraps (auth, CSRF) — both would otherwise be impossible to
// satisfy before a session exists.
func skipForPublicPaths(mw func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		wrapped := mw(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}
