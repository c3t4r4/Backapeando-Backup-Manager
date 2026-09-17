// Command worker runs the backup scheduler worker in a separate process.
// It polls the database for servers eligible for backup, claims them with
// FOR UPDATE SKIP LOCKED, and enqueues backup tasks to the worker pool for
// execution. Graceful shutdown via SIGTERM/SIGINT.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"backapeando-backup-manager/internal/config"
	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/repository"
	"backapeando-backup-manager/internal/scheduler"
)

func main() {
	exitOnErr(runWorker())
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runWorker initializes and runs the worker scheduler loop.
// It loads config, migrates database, initializes crypto, creates the worker pool
// and scheduler, and runs until receiving SIGTERM or SIGINT.
//
// Returns an error if any initialization step fails.
func runWorker() error {
	// Load configuration with fail-fast on missing required env vars
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Migrate database schema
	if err := db.Migrate(cfg.DSN); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Connect to database
	ctx := context.Background()
	pool, err := repository.Connect(ctx, cfg.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	// Initialize repositories
	repos := repository.New(pool)

	// Initialize crypto sealer for decrypting SSH keys and SAS tokens
	sealer, err := crypto.NewSealer(cfg.MasterKey, cfg.MasterKeyPrevious)
	if err != nil {
		return fmt.Errorf("init sealer: %w", err)
	}

	// Parse scheduler configuration from environment
	maxConcurrentBackups := getEnvIntDefault("MAX_CONCURRENT_BACKUPS", 3)
	schedulerPollIntervalSecs := getEnvIntDefault("SCHEDULER_POLL_INTERVAL", 30)
	gracefulShutdownTimeoutSecs := getEnvIntDefault("GRACEFUL_SHUTDOWN_TIMEOUT", 300)
	// BACKUP_TASK_TIMEOUT bounds a single backup's execution (SSH handshake
	// already has its own 30s timeout; this covers everything after it —
	// dump streaming and upload — which previously had none. A run that
	// exceeds this is marked failed instead of occupying its worker forever
	// (see docs/RegrasNegocio.md RN-BACKUP-028). Default 6h.
	backupTaskTimeoutSecs := getEnvIntDefault("BACKUP_TASK_TIMEOUT", 6*60*60)
	// WATCHDOG_POLL_INTERVAL controls how often the watchdog sweeps for
	// backup_runs stuck in 'running' past BACKUP_TASK_TIMEOUT (safety net
	// for runs stuck before this fix, or through a path the per-task timeout
	// doesn't cover). Default 5 min.
	watchdogPollIntervalSecs := getEnvIntDefault("WATCHDOG_POLL_INTERVAL", 300)

	pollInterval := time.Duration(schedulerPollIntervalSecs) * time.Second
	gracefulTimeout := time.Duration(gracefulShutdownTimeoutSecs) * time.Second
	backupTaskTimeout := time.Duration(backupTaskTimeoutSecs) * time.Second
	watchdogPollInterval := time.Duration(watchdogPollIntervalSecs) * time.Second

	// Create and start worker pool
	workerPool := scheduler.NewWorkerPool(maxConcurrentBackups)
	workerPool.SetTaskTimeout(backupTaskTimeout)
	workerPool.Start(maxConcurrentBackups)

	// Create backup executor
	executor := scheduler.NewBackupExecutor(pool, repos, sealer, slog.Default())

	// Create and start scheduler
	sched := scheduler.NewScheduler(workerPool, repos, executor, pollInterval, slog.Default())
	sched.Start()

	// Create and start the watchdog (staleAfter uses the same duration as
	// the per-task timeout: a run still "running" past that point is, by
	// definition, one the timeout should already have failed).
	watchdog := scheduler.NewWatchdog(repos, backupTaskTimeout, watchdogPollInterval, slog.Default())
	watchdog.Start()

	slog.Info("worker_started",
		"maxConcurrentBackups", maxConcurrentBackups,
		"schedulerPollInterval", pollInterval,
		"gracefulShutdownTimeout", gracefulTimeout,
		"backupTaskTimeout", backupTaskTimeout,
		"watchdogPollInterval", watchdogPollInterval,
	)

	// Setup signal handler for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	setupSignalHandler(sigCh)

	// Block until signal received
	<-sigCh

	slog.Info("shutdown_started")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer cancel()

	if err := sched.Stop(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, "scheduler stop failed",
			slog.String("error", err.Error()),
		)
	}

	if err := watchdog.Stop(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, "watchdog stop failed",
			slog.String("error", err.Error()),
		)
	}

	if err := workerPool.Stop(shutdownCtx); err != nil {
		slog.ErrorContext(shutdownCtx, "worker pool stop failed",
			slog.String("error", err.Error()),
		)
	}

	slog.Info("worker_stopped")
	return nil
}

// setupSignalHandler registers SIGTERM and SIGINT handlers to the provided channel.
func setupSignalHandler(sigCh chan<- os.Signal) {
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
}

// getEnvIntDefault reads an integer environment variable with a default fallback.
// If the variable is unset or invalid, it logs a warning and returns the default.
func getEnvIntDefault(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		slog.Warn("invalid integer env var, using default",
			slog.String("key", key),
			slog.String("value", raw),
			slog.Int("default", def),
		)
		return def
	}
	return val
}
