// Command api runs the HTTP API server, and also exposes a couple of small
// CLI subcommands (create-admin, reset-password) used for account
// bootstrap/recovery — there is deliberately no public registration
// endpoint or SMTP-based password reset (see docs/Auth.md).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"golang.org/x/term"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/config"
	"backapeando-backup-manager/internal/cpf"
	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/httpapi"
	"backapeando-backup-manager/internal/repository"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "create-admin":
			exitOnErr(runCreateAdmin())
			return
		case "reset-password":
			exitOnErr(runResetPassword())
			return
		}
	}
	exitOnErr(runServer())
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runServer() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := db.Migrate(cfg.DSN); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	ctx := context.Background()
	pool, err := repository.Connect(ctx, cfg.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	repos := repository.New(pool)
	sealer, err := crypto.NewSealer(cfg.MasterKey, cfg.MasterKeyPrevious)
	if err != nil {
		return fmt.Errorf("init sealer: %w", err)
	}

	sessions := &auth.SessionManager{
		Sessions:    repos.AdminSessions,
		CookieName:  cfg.SessionCookieName,
		IdleTTL:     cfg.SessionIdleTTL,
		AbsoluteTTL: cfg.SessionAbsoluteTTL,
		Secure:      cfg.SecureCookies,
	}

	// Initialize rate limiter (Redis or noop if not configured)
	var rateLimiter auth.RateLimiter
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		rl, err := auth.NewRedisRateLimiter(redisURL, 5)
		if err != nil {
			return fmt.Errorf("init redis rate limiter: %w", err)
		}
		rateLimiter = rl
		slog.Info("rate_limiter_initialized", "backend", "redis")
	} else {
		// Fallback to noop (development only)
		rateLimiter = auth.NewLoginRateLimiter(5, time.Minute)
		slog.Warn("rate_limiter_noop", "reason", "REDIS_URL not set")
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Repos:             repos,
		Sessions:          sessions,
		Sealer:            sealer,
		RateLimiter:       rateLimiter,
		SecureCookies:     cfg.SecureCookies,
		CORSAllowedOrigin: cfg.CORSAllowedOrigin,
	})

	slog.Info("api_starting", "addr", cfg.HTTPAddr)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}

// runCreateAdmin bootstraps the first (or an additional) admin account.
// There is no public registration endpoint by design — creating an admin
// requires access to the host running this binary.
func runCreateAdmin() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := db.Migrate(cfg.DSN); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	ctx := context.Background()
	pool, err := repository.Connect(ctx, cfg.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	repos := repository.New(pool)

	email, err := prompt("Admin email: ")
	if err != nil {
		return err
	}
	rawCPF, err := prompt("Admin CPF: ")
	if err != nil {
		return err
	}
	normalizedCPF, err := cpf.Validate(rawCPF)
	if err != nil {
		return fmt.Errorf("invalid CPF: %w", err)
	}
	password, err := promptPassword("Admin password: ")
	if err != nil {
		return err
	}
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user, err := repos.AdminUsers.Create(ctx, email, normalizedCPF, hash, "admin")
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}

	fmt.Printf("Admin created: %s (%s)\n", user.Email, user.ID)
	return nil
}

func runResetPassword() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	pool, err := repository.Connect(ctx, cfg.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()
	repos := repository.New(pool)

	email, err := prompt("Admin email: ")
	if err != nil {
		return err
	}
	user, err := repos.AdminUsers.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("no admin found with that email")
	}
	if err != nil {
		return err
	}

	password, err := promptPassword("New password: ")
	if err != nil {
		return err
	}
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := repos.AdminUsers.UpdatePassword(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	fmt.Printf("Password reset for %s\n", user.Email)
	return nil
}

func prompt(label string) (string, error) {
	fmt.Print(label)
	var s string
	if _, err := fmt.Scanln(&s); err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSpace(s), nil
}

// promptPassword reads a password without echoing it to the terminal.
func promptPassword(label string) (string, error) {
	fmt.Print(label)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimSpace(string(pw)), nil
}
