package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"backapeando-backup-manager/internal/repository"
)

// Watchdog periodically fails any backup_runs row stuck in status='running'
// longer than staleAfter. It exists as a safety net alongside
// WorkerPool.SetTaskTimeout: the per-task timeout is what should normally
// resolve a hung backup (network stall, unresponsive remote), but a row that
// got stuck before that timeout existed — or through some path the timeout
// doesn't cover — would otherwise stay "running" forever with no further
// signal in the History screen or the logs (see docs/RegrasNegocio.md
// RN-BACKUP-028, docs/Memoria.md).
type Watchdog struct {
	repos        *repository.Repositories
	staleAfter   time.Duration
	pollInterval time.Duration
	logger       *slog.Logger

	ctx       context.Context
	cancel    context.CancelFunc
	stoppedCh chan struct{}
}

// NewWatchdog creates a watchdog that sweeps every pollInterval, failing any
// run whose started_at is older than staleAfter.
func NewWatchdog(repos *repository.Repositories, staleAfter, pollInterval time.Duration, logger *slog.Logger) *Watchdog {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Watchdog{
		repos:        repos,
		staleAfter:   staleAfter,
		pollInterval: pollInterval,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		stoppedCh:    make(chan struct{}),
	}
}

// Start begins the sweep loop in a new goroutine. Safe to call once; it is
// not idempotent (mirrors Scheduler.Start's contract in this package).
func (w *Watchdog) Start() {
	go w.run()
}

func (w *Watchdog) run() {
	defer close(w.stoppedCh)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	w.logger.Info("watchdog started", "staleAfter", w.staleAfter, "pollInterval", w.pollInterval)

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("watchdog stopped")
			return
		case <-ticker.C:
			w.sweep()
		}
	}
}

func (w *Watchdog) sweep() {
	errMsg := fmt.Sprintf("execução marcada como falha pelo watchdog: preso em 'running' por mais de %s sem concluir", w.staleAfter)
	count, err := w.repos.BackupRuns.FailStaleRunning(w.ctx, w.staleAfter, errMsg)
	if err != nil {
		w.logger.ErrorContext(w.ctx, "watchdog: failed to sweep stale running backup runs",
			slog.String("error", err.Error()),
		)
		return
	}
	if count > 0 {
		w.logger.WarnContext(w.ctx, "watchdog: marked stale running backup runs as failed",
			slog.Int64("count", count),
		)
	}
}

// Stop signals the watchdog to stop and waits for the loop to exit,
// respecting ctx's deadline.
func (w *Watchdog) Stop(ctx context.Context) error {
	w.cancel()
	select {
	case <-w.stoppedCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop: %w", ctx.Err())
	}
}
