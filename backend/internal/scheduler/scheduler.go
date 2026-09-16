package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"backapeando-backup-manager/internal/repository"
)

// Pool defines the interface for enqueuing tasks and checking capacity.
// This abstraction allows for testing with mocks without depending on
// the concrete WorkerPool implementation.
type Pool interface {
	// AvailableSlots returns the number of free slots in the pool.
	AvailableSlots() int

	// Submit enqueues a task to the pool.
	Submit(task WorkerTask) error

	// Stop gracefully shuts down the pool, respecting the provided context's deadline.
	Stop(ctx context.Context) error

	// Start initializes the pool with the specified number of workers.
	Start(numWorkers int)
}

// Scheduler polls for backup-eligible servers, claims them with SKIP LOCKED,
// creates backup_runs, and enqueues tasks to the pool for execution.
type Scheduler struct {
	pool         Pool
	repos        *repository.Repositories
	executor     *BackupExecutor
	pollInterval time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *slog.Logger
	stoppedCh    chan struct{}
	mu           sync.Mutex
	started      bool
	stopped      bool
}

// NewScheduler creates a new scheduler that will poll the database at the
// specified interval and enqueue backup tasks to the pool.
//
// Args:
//   - pool: Pool implementation for executing backup tasks
//   - repos: Repositories for database access
//   - executor: BackupExecutor for executing backup tasks
//   - pollInterval: how often to poll for ready servers (e.g., 30s)
//   - logger: structured logger (can be nil, will be set to noop if needed)
//
// Note: Scheduler does not start automatically. Call Start() to begin polling.
func NewScheduler(pool Pool, repos *repository.Repositories, executor *BackupExecutor, pollInterval time.Duration, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		pool:         pool,
		repos:        repos,
		executor:     executor,
		pollInterval: pollInterval,
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
		stoppedCh:    make(chan struct{}),
	}
}

// Start begins the polling loop in a new goroutine. It is safe to call
// multiple times (subsequent calls are no-ops).
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.poll()
}

// Stop signals the scheduler to stop polling and waits for the loop to exit.
// It respects the provided context's deadline; if deadline expires before the scheduler
// exits, returns ctx.Err(). It is safe to call multiple times and safe to call without Start().
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	wasStarted := s.started
	s.mu.Unlock()

	// Cancel the context to signal the polling loop to exit.
	s.cancel()

	// If the loop was started, wait for it to exit.
	if wasStarted {
		select {
		case <-s.stoppedCh:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("stop: %w", ctx.Err())
		}
	} else {
		// Loop was never started; close stoppedCh to satisfy any waiters.
		close(s.stoppedCh)
	}

	return nil
}

// poll runs the main scheduling loop, executing at each pollInterval tick.
// It polls for servers eligible for backup, respects pool capacity, and
// enqueues tasks. The loop exits when ctx is canceled.
func (s *Scheduler) poll() {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	s.logger.Info("scheduler started", "pollInterval", s.pollInterval)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("scheduler stopped")
			return

		case <-ticker.C:
			// Check how many slots are available in the worker pool
			availableSlots := s.pool.AvailableSlots()
			if availableSlots <= 0 {
				// Pool is at capacity; skip this poll cycle
				s.logger.Debug("worker pool at capacity", "availableSlots", availableSlots)
				continue
			}

			// In Tarefa 4: claimAndEnqueue will call repo.Servers.GetReadyServersForScheduling
			// and create backup_runs atomically, then enqueue tasks to the pool.
			s.claimAndEnqueue(s.ctx, availableSlots)
		}
	}
}

// claimAndEnqueue is called each poll cycle to claim servers ready for
// execution and enqueue them to the worker pool. It uses FOR UPDATE SKIP LOCKED
// to claim servers atomically, creates backup_runs, and submits tasks to the pool.
//
// Args:
//   - ctx: context for database operations
//   - vagas: number of available slots in the worker pool
//
// Note: This method handles concurrent schedulers correctly; the database query
// uses FOR UPDATE SKIP LOCKED to ensure each server is claimed by exactly one scheduler.
func (s *Scheduler) claimAndEnqueue(ctx context.Context, vagas int) {
	// Guard against nil repos (can happen in tests with incomplete mocks)
	if s.repos == nil || s.repos.Servers == nil {
		s.logger.Debug("repos or repos.Servers is nil, skipping claim")
		return
	}

	servers, err := s.repos.Servers.GetReadyServersForScheduling(ctx, vagas)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to claim servers for scheduling",
			slog.String("error", err.Error()),
		)
		return
	}

	if len(servers) > 0 {
		s.logger.InfoContext(ctx, "claimed servers for scheduling",
			slog.Int("count", len(servers)),
		)
	} else {
		s.logger.Debug("no servers eligible for scheduling")
	}

	for _, server := range servers {
		// Parse cron expression and compute next run time
		nextRun, err := NextRunTime(server.CronExpression, time.Now())
		if err != nil {
			s.logger.ErrorContext(ctx, "invalid cron expression",
				slog.String("serverId", server.ID),
				slog.String("cronExpression", server.CronExpression),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Update server's next_run_at for future claims
		if err := s.repos.Servers.UpdateNextRun(ctx, server.ID, nextRun); err != nil {
			s.logger.ErrorContext(ctx, "failed to update next_run_at",
				slog.String("serverId", server.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Fetch storage target to pass to executor
		if server.StorageTargetID == nil {
			s.logger.WarnContext(ctx, "server has no storage target configured",
				slog.String("serverId", server.ID),
			)
			continue
		}

		storageTarget, err := s.repos.StorageTargets.Get(ctx, *server.StorageTargetID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to fetch storage target",
				slog.String("serverId", server.ID),
				slog.String("targetId", *server.StorageTargetID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Create BackupRun with status='running' (created immediately upon claim)
		backupRun, err := s.repos.BackupRuns.Create(ctx, server.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to create backup run",
				slog.String("serverId", server.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Create executor closure that captures all necessary context
		executor := s.executor
		runToExecute := &backupRun
		serverToExecute := server
		targetToExecute := &storageTarget

		// Submit task to worker pool
		task := WorkerTask{
			ID:        backupRun.ID,
			ServerID:  server.ID,
			BackupRun: runToExecute,
			Execute: func(ctx context.Context) error {
				return executor.ExecuteBackup(ctx, runToExecute, serverToExecute, targetToExecute)
			},
		}

		if err := s.pool.Submit(task); err != nil {
			s.logger.ErrorContext(ctx, "failed to submit backup task to pool",
				slog.String("backupRunId", backupRun.ID),
				slog.String("serverId", server.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		s.logger.InfoContext(ctx, "backup task enqueued",
			slog.String("backupRunId", backupRun.ID),
			slog.String("serverId", server.ID),
			slog.Time("nextRunAt", nextRun),
		)
	}

	// Tarefa 6: Process manually-triggered backups (status='queued') from /api/servers/{id}/run-now endpoint
	remainingSlots := s.pool.AvailableSlots()
	if remainingSlots > 0 {
		s.processQueuedRuns(ctx, remainingSlots)
	}
}

// processQueuedRuns discovers and processes backup runs with status='queued' that were
// manually triggered via the /api/servers/{id}/run-now endpoint (Tarefa 6).
// It transitions each queued run to 'running' and enqueues it to the worker pool.
func (s *Scheduler) processQueuedRuns(ctx context.Context, maxRuns int) {
	if s.repos == nil || s.repos.BackupRuns == nil || s.repos.Servers == nil {
		s.logger.Debug("repos or required repo is nil, skipping queued runs processing")
		return
	}

	queuedRuns, err := s.repos.BackupRuns.GetQueuedRuns(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch queued runs",
			slog.String("error", err.Error()),
		)
		return
	}

	if len(queuedRuns) == 0 {
		return
	}

	// Process up to maxRuns queued runs
	limit := maxRuns
	if len(queuedRuns) < limit {
		limit = len(queuedRuns)
	}

	s.logger.InfoContext(ctx, "processing queued backup runs",
		slog.Int("count", limit),
		slog.Int("total_queued", len(queuedRuns)),
	)

	for i := 0; i < limit; i++ {
		queuedRun := queuedRuns[i]

		// Transition run from 'queued' to 'running' with current timestamp
		if err := s.repos.BackupRuns.UpdateStatus(ctx, queuedRun.ID, "running", nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to transition queued run to running",
				slog.String("backupRunId", queuedRun.ID),
				slog.String("serverId", queuedRun.ServerID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Fetch the server
		server, err := s.repos.Servers.Get(ctx, queuedRun.ServerID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to fetch server for queued run",
				slog.String("backupRunId", queuedRun.ID),
				slog.String("serverId", queuedRun.ServerID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Validate storage target is configured
		if server.StorageTargetID == nil {
			s.logger.WarnContext(ctx, "server for queued run has no storage target configured",
				slog.String("backupRunId", queuedRun.ID),
				slog.String("serverId", queuedRun.ServerID),
			)
			continue
		}

		// Fetch storage target
		storageTarget, err := s.repos.StorageTargets.Get(ctx, *server.StorageTargetID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to fetch storage target for queued run",
				slog.String("backupRunId", queuedRun.ID),
				slog.String("serverId", queuedRun.ServerID),
				slog.String("targetId", *server.StorageTargetID),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Enqueue the task to the worker pool
		runToExecute := &queuedRun
		serverToExecute := &server
		targetToExecute := &storageTarget

		task := WorkerTask{
			ID:        queuedRun.ID,
			ServerID:  queuedRun.ServerID,
			BackupRun: runToExecute,
			Execute: func(ctx context.Context) error {
				return s.executor.ExecuteBackup(ctx, runToExecute, serverToExecute, targetToExecute)
			},
		}

		if err := s.pool.Submit(task); err != nil {
			s.logger.ErrorContext(ctx, "failed to submit queued backup task to pool",
				slog.String("backupRunId", queuedRun.ID),
				slog.String("serverId", queuedRun.ServerID),
				slog.String("error", err.Error()),
			)
			continue
		}

		s.logger.InfoContext(ctx, "queued backup task enqueued",
			slog.String("backupRunId", queuedRun.ID),
			slog.String("serverId", queuedRun.ServerID),
		)
	}
}
