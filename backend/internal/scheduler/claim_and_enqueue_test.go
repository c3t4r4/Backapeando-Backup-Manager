package scheduler

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

// TestClaimAndEnqueueProcessesQueuedRunsEvenWithNoCronEligibleServers is a
// regression test for a bug where claimAndEnqueue returned early whenever
// GetReadyServersForScheduling found zero cron-eligible servers, skipping
// the processQueuedRuns() call entirely. That meant a backup manually
// queued via POST /run-now was only ever picked up if some OTHER server
// happened to be due for automatic cron scheduling in the very same poll
// cycle — otherwise it stayed status='queued' forever, even with the
// worker running and healthy.
func TestClaimAndEnqueueProcessesQueuedRunsEvenWithNoCronEligibleServers(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	dbPool, err := repository.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(dbPool.Close)

	repos := repository.New(dbPool)

	accountName, containerName := "testaccount", "testcontainer"
	storageTarget := domain.StorageTarget{
		ID:                     uuid.NewString(),
		Name:                   "claim-test-azure-" + uuid.New().String()[:8],
		Type:                   domain.StorageTargetTypeAzure,
		AzureAccountName:       &accountName,
		AzureContainerName:     &containerName,
		AzureSASTokenEncrypted: []byte("dummy-encrypted-sas-token"),
	}
	createdTarget, err := repos.StorageTargets.Create(ctx, storageTarget)
	if err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	// A freshly created server has status='pending_key', enabled=false and
	// next_run_at=NULL — it will never appear in GetReadyServersForScheduling,
	// reproducing the exact "no cron-eligible servers this cycle" case that
	// used to make claimAndEnqueue skip processQueuedRuns entirely.
	server, err := repos.Servers.Create(ctx, domain.Server{
		Name:            "claim-test-server-" + uuid.New().String()[:8],
		Host:            "example.com",
		Port:            22,
		SSHUser:         "postgres",
		DBEngine:        domain.DBEnginePostgres,
		DeploymentMode:  domain.DeploymentModeDocker,
		ContainerName:   &containerName,
		DBName:          "testdb",
		DBUser:          "testuser",
		CronExpression:  "0 3 * * *",
		StorageTargetID: &createdTarget.ID,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() {
		_ = repos.Servers.Delete(context.Background(), server.ID)
		_ = repos.StorageTargets.Delete(context.Background(), createdTarget.ID)
	})

	queuedRun, err := repos.BackupRuns.CreateQueued(ctx, server.ID)
	if err != nil {
		t.Fatalf("create queued backup run: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(
		noopWriter{},
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))
	mockPool := &MockWorkerPool{availableSlots: 3}
	executor := NewBackupExecutor(dbPool, repos, nil, logger)
	sched := NewScheduler(mockPool, repos, executor, time.Minute, logger)

	sched.claimAndEnqueue(ctx, 3)

	if mockPool.submitCalls == 0 {
		t.Fatal("expected the queued run to be submitted to the pool, but claimAndEnqueue returned early without ever calling processQueuedRuns")
	}

	found := false
	for _, task := range mockPool.submitTasks {
		if task.ID == queuedRun.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a task for queued run %s to be submitted, got %d task(s) submitted", queuedRun.ID, len(mockPool.submitTasks))
	}
}
