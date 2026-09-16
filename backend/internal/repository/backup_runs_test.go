package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

// TestUpdateStatus_SetsTimestamps is a regression test for the bug where
// every backup run processed by the scheduler (executor.go and
// scheduler.go's queued→running promotion) went through UpdateStatus, whose
// query never touched started_at/finished_at — leaving both NULL forever
// for any run not created via the synchronous BackupNow path (Create +
// MarkSuccess/MarkFailed). See docs/Memoria.md for the full incident.
func TestUpdateStatus_SetsTimestamps(t *testing.T) {
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

	containerName := "testcontainer"
	serverInput := domain.Server{
		Name:           "backup-runs-test-" + uuid.New().String()[:8],
		Host:           "example.com",
		Port:           22,
		SSHUser:        "postgres",
		DBEngine:       domain.DBEnginePostgres,
		DeploymentMode: domain.DeploymentModeDocker,
		ContainerName:  &containerName,
		DBName:         "testdb",
		DBUser:         "testuser",
		CronExpression: "0 3 * * *",
	}
	server, err := repos.Servers.Create(ctx, serverInput)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	t.Run("running sets started_at when NULL (manual/queued path)", func(t *testing.T) {
		run, err := repos.BackupRuns.CreateQueued(ctx, server.ID)
		if err != nil {
			t.Fatalf("create queued: %v", err)
		}
		if run.StartedAt != nil {
			t.Fatalf("expected StartedAt nil right after CreateQueued, got %v", run.StartedAt)
		}

		if err := repos.BackupRuns.UpdateStatus(ctx, run.ID, "running", nil); err != nil {
			t.Fatalf("update status running: %v", err)
		}

		got, err := repos.BackupRuns.Get(ctx, run.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.StartedAt == nil {
			t.Fatal("expected StartedAt to be set after transitioning to running")
		}
	})

	t.Run("running preserves started_at when already set (scheduled path)", func(t *testing.T) {
		run, err := repos.BackupRuns.Create(ctx, server.ID)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		originalStartedAt := run.StartedAt
		if originalStartedAt == nil {
			t.Fatal("expected StartedAt to be set by Create")
		}

		if err := repos.BackupRuns.UpdateStatus(ctx, run.ID, "running", nil); err != nil {
			t.Fatalf("update status running: %v", err)
		}

		got, err := repos.BackupRuns.Get(ctx, run.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.StartedAt == nil || !got.StartedAt.Equal(*originalStartedAt) {
			t.Fatalf("expected StartedAt to be preserved (COALESCE), got %v want %v", got.StartedAt, originalStartedAt)
		}
	})

	t.Run("success sets finished_at", func(t *testing.T) {
		run, err := repos.BackupRuns.Create(ctx, server.ID)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		if err := repos.BackupRuns.UpdateStatus(ctx, run.ID, "success", nil); err != nil {
			t.Fatalf("update status success: %v", err)
		}

		got, err := repos.BackupRuns.Get(ctx, run.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.FinishedAt == nil {
			t.Fatal("expected FinishedAt to be set after transitioning to success")
		}
	})

	t.Run("failed sets finished_at", func(t *testing.T) {
		run, err := repos.BackupRuns.Create(ctx, server.ID)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		errMsg := "simulated failure"
		if err := repos.BackupRuns.UpdateStatus(ctx, run.ID, "failed", &errMsg); err != nil {
			t.Fatalf("update status failed: %v", err)
		}

		got, err := repos.BackupRuns.Get(ctx, run.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.FinishedAt == nil {
			t.Fatal("expected FinishedAt to be set after transitioning to failed")
		}
	})

	t.Run("full queued -> running -> success flow leaves both timestamps set", func(t *testing.T) {
		run, err := repos.BackupRuns.CreateQueued(ctx, server.ID)
		if err != nil {
			t.Fatalf("create queued: %v", err)
		}

		if err := repos.BackupRuns.UpdateStatus(ctx, run.ID, "running", nil); err != nil {
			t.Fatalf("update status running: %v", err)
		}
		if err := repos.BackupRuns.UpdateStatus(ctx, run.ID, "success", nil); err != nil {
			t.Fatalf("update status success: %v", err)
		}

		got, err := repos.BackupRuns.Get(ctx, run.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.StartedAt == nil {
			t.Error("expected StartedAt to be set")
		}
		if got.FinishedAt == nil {
			t.Error("expected FinishedAt to be set")
		}
	})
}

// TestMarkSuccess is a regression test for RN-BACKUP-022: the scheduler's
// executor.go used to call UpdateBlobMetadata+UpdateStatus on the success
// path, neither of which ever wrote dump_duration_ms/upload_duration_ms, so
// every scheduler-driven run kept those two columns NULL forever even though
// the values were already computed in memory. executor.go now calls
// MarkSuccess instead (the same method the manual backup-now path already
// used correctly) — this test asserts MarkSuccess itself persists every
// field it claims to, including both durations, directly against a real
// Postgres, independent of any SSH-dependent execution path.
func TestMarkSuccess(t *testing.T) {
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

	containerName := "testcontainer"
	server, err := repos.Servers.Create(ctx, domain.Server{
		Name:           "mark-success-test-" + uuid.New().String()[:8],
		Host:           "example.com",
		Port:           22,
		SSHUser:        "postgres",
		DBEngine:       domain.DBEnginePostgres,
		DeploymentMode: domain.DeploymentModeDocker,
		ContainerName:  &containerName,
		DBName:         "testdb",
		DBUser:         "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	run, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	const (
		blobName         = "prod/testdb_20260916T000000Z.dump"
		blobSizeBytes    = int64(123456789)
		dumpDurationMS   = 4321
		uploadDurationMS = 8765
	)
	if err := repos.BackupRuns.MarkSuccess(ctx, run.ID, blobName, blobSizeBytes, dumpDurationMS, uploadDurationMS); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	got, err := repos.BackupRuns.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.BackupRunStatusSuccess {
		t.Errorf("Status = %s, want success", got.Status)
	}
	if got.BlobName == nil || *got.BlobName != blobName {
		t.Errorf("BlobName = %v, want %q", got.BlobName, blobName)
	}
	if got.BlobSizeBytes == nil || *got.BlobSizeBytes != blobSizeBytes {
		t.Errorf("BlobSizeBytes = %v, want %d", got.BlobSizeBytes, blobSizeBytes)
	}
	if got.DumpDurationMS == nil || *got.DumpDurationMS != dumpDurationMS {
		t.Errorf("DumpDurationMS = %v, want %d", got.DumpDurationMS, dumpDurationMS)
	}
	if got.UploadDurationMS == nil || *got.UploadDurationMS != uploadDurationMS {
		t.Errorf("UploadDurationMS = %v, want %d", got.UploadDurationMS, uploadDurationMS)
	}
	if got.FinishedAt == nil {
		t.Error("expected FinishedAt to be set after MarkSuccess")
	}
}

// TestBackupRunRepo_List_PaginationAndStatusFilter covers RN-BACKUP-025:
// page/pageSize slicing, the separate COUNT(*) query staying accurate across
// pages (including a page beyond the last, which returns zero rows but must
// not report a zero total), an optional status filter, and the zero-match
// case.
func TestBackupRunRepo_List_PaginationAndStatusFilter(t *testing.T) {
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

	containerName := "testcontainer"
	server, err := repos.Servers.Create(ctx, domain.Server{
		Name:           "backup-runs-pagination-test-" + uuid.New().String()[:8],
		Host:           "example.com",
		Port:           22,
		SSHUser:        "postgres",
		DBEngine:       domain.DBEnginePostgres,
		DeploymentMode: domain.DeploymentModeDocker,
		ContainerName:  &containerName,
		DBName:         "testdb",
		DBUser:         "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	const totalRuns = 5
	for i := 0; i < totalRuns; i++ {
		run, err := repos.BackupRuns.Create(ctx, server.ID)
		if err != nil {
			t.Fatalf("create run %d: %v", i, err)
		}
		// Mark exactly one of them failed, to exercise the status filter.
		if i == 0 {
			if err := repos.BackupRuns.MarkFailed(ctx, run.ID, "simulated failure"); err != nil {
				t.Fatalf("mark failed: %v", err)
			}
		}
	}

	t.Run("first page respects pageSize and reports the full total", func(t *testing.T) {
		runs, total, err := repos.BackupRuns.List(ctx, server.ID, nil, 1, 2)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(runs) != 2 {
			t.Fatalf("expected 2 runs on page 1 (pageSize=2), got %d", len(runs))
		}
		if total != totalRuns {
			t.Fatalf("total = %d, want %d", total, totalRuns)
		}
	})

	t.Run("last page returns the remainder and the same total", func(t *testing.T) {
		// 5 runs, pageSize=2 -> page 3 has exactly 1 row.
		runs, total, err := repos.BackupRuns.List(ctx, server.ID, nil, 3, 2)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(runs) != 1 {
			t.Fatalf("expected 1 run on the last page, got %d", len(runs))
		}
		if total != totalRuns {
			t.Fatalf("total = %d, want %d", total, totalRuns)
		}
	})

	t.Run("page beyond the last returns zero rows and the real total, not an error", func(t *testing.T) {
		runs, total, err := repos.BackupRuns.List(ctx, server.ID, nil, 99, 2)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(runs) != 0 {
			t.Fatalf("expected 0 runs beyond the last page, got %d", len(runs))
		}
		if total != totalRuns {
			t.Fatalf("total = %d, want %d even when the page itself is empty", total, totalRuns)
		}
	})

	t.Run("status filter narrows both the rows and the total", func(t *testing.T) {
		failed := "failed"
		runs, total, err := repos.BackupRuns.List(ctx, server.ID, &failed, 1, 50)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 1 {
			t.Fatalf("total = %d, want 1 (only one run was marked failed)", total)
		}
		if len(runs) != 1 || runs[0].Status != domain.BackupRunStatusFailed {
			t.Fatalf("expected exactly one failed run, got %+v", runs)
		}
	})

	t.Run("status filter with no matches returns zero rows and zero total", func(t *testing.T) {
		queued := "queued"
		runs, total, err := repos.BackupRuns.List(ctx, server.ID, &queued, 1, 50)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 0 || len(runs) != 0 {
			t.Fatalf("expected no queued runs, got total=%d len=%d", total, len(runs))
		}
	})
}

// TestBackupRunRepo_ListAll covers RN-BACKUP-026: the History screen's
// default "no server selected" view, which lists across every server. It
// asserts that a nil serverID returns runs from every server (unlike List,
// whose serverID is mandatory), that passing a serverID narrows the same way
// List does, and that the status filter still applies in both cases.
func TestBackupRunRepo_ListAll(t *testing.T) {
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

	containerName := "testcontainer"
	newServer := func(name string) domain.Server {
		s, err := repos.Servers.Create(ctx, domain.Server{
			Name:           name + "-" + uuid.New().String()[:8],
			Host:           "example.com",
			Port:           22,
			SSHUser:        "postgres",
			DBEngine:       domain.DBEnginePostgres,
			DeploymentMode: domain.DeploymentModeDocker,
			ContainerName:  &containerName,
			DBName:         "testdb",
			DBUser:         "testuser",
			CronExpression: "0 3 * * *",
		})
		if err != nil {
			t.Fatalf("create server %q: %v", name, err)
		}
		return s
	}

	serverA := newServer("list-all-a")
	serverB := newServer("list-all-b")

	runA, err := repos.BackupRuns.Create(ctx, serverA.ID)
	if err != nil {
		t.Fatalf("create run for server A: %v", err)
	}
	runB, err := repos.BackupRuns.Create(ctx, serverB.ID)
	if err != nil {
		t.Fatalf("create run for server B: %v", err)
	}
	if err := repos.BackupRuns.MarkFailed(ctx, runB.ID, "simulated failure"); err != nil {
		t.Fatalf("mark run B failed: %v", err)
	}

	t.Run("nil serverID returns runs from every server", func(t *testing.T) {
		runs, _, err := repos.BackupRuns.ListAll(ctx, nil, nil, 1, 200)
		if err != nil {
			t.Fatalf("list all: %v", err)
		}
		var sawA, sawB bool
		for _, r := range runs {
			if r.ID == runA.ID {
				sawA = true
			}
			if r.ID == runB.ID {
				sawB = true
			}
		}
		if !sawA || !sawB {
			t.Fatalf("expected both server A's and server B's runs in the combined list, sawA=%v sawB=%v", sawA, sawB)
		}
	})

	t.Run("a serverID narrows to that server only, like List", func(t *testing.T) {
		runs, total, err := repos.BackupRuns.ListAll(ctx, &serverA.ID, nil, 1, 200)
		if err != nil {
			t.Fatalf("list all: %v", err)
		}
		if total != 1 || len(runs) != 1 || runs[0].ID != runA.ID {
			t.Fatalf("expected exactly server A's single run, got total=%d runs=%+v", total, runs)
		}
	})

	t.Run("status filter narrows across every server", func(t *testing.T) {
		failed := "failed"
		runs, total, err := repos.BackupRuns.ListAll(ctx, nil, &failed, 1, 200)
		if err != nil {
			t.Fatalf("list all: %v", err)
		}
		for _, r := range runs {
			if r.Status != domain.BackupRunStatusFailed {
				t.Fatalf("expected only failed runs, got %+v", r)
			}
		}
		if total < 1 {
			t.Fatalf("expected at least 1 failed run (server B's), got total=%d", total)
		}
	})
}

// TestMarkFailedWithDetails covers RN-BACKUP-027: unlike MarkFailed, it also
// persists log_output, and both fields are expected to already be
// redacted/truncated by the caller — this test only asserts persistence.
func TestMarkFailedWithDetails(t *testing.T) {
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

	containerName := "testcontainer"
	server, err := repos.Servers.Create(ctx, domain.Server{
		Name:           "mark-failed-details-test-" + uuid.New().String()[:8],
		Host:           "example.com",
		Port:           22,
		SSHUser:        "postgres",
		DBEngine:       domain.DBEnginePostgres,
		DeploymentMode: domain.DeploymentModeDocker,
		ContainerName:  &containerName,
		DBName:         "testdb",
		DBUser:         "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	run, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	logOutput := "stdout: (empty)\nstderr: pg_dump: error: connection refused"
	if err := repos.BackupRuns.MarkFailedWithDetails(ctx, run.ID, "falha na conexão SSH: connection refused", &logOutput); err != nil {
		t.Fatalf("MarkFailedWithDetails: %v", err)
	}

	got, err := repos.BackupRuns.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.BackupRunStatusFailed {
		t.Errorf("Status = %s, want failed", got.Status)
	}
	if got.ErrorMessage == nil || *got.ErrorMessage != "falha na conexão SSH: connection refused" {
		t.Errorf("ErrorMessage = %v, want the real stage-specific message", got.ErrorMessage)
	}
	if got.LogOutput == nil || *got.LogOutput != logOutput {
		t.Errorf("LogOutput = %v, want %q", got.LogOutput, logOutput)
	}
	if got.FinishedAt == nil {
		t.Error("expected FinishedAt to be set after MarkFailedWithDetails")
	}
}

// TestFailStaleRunning is a regression test for the watchdog safety net
// (RN-BACKUP-028): a backup_run stuck in 'running' past the configured
// staleness window must be marked failed, while one still within the window
// (or not running at all) must be left untouched.
func TestFailStaleRunning(t *testing.T) {
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

	containerName := "testcontainer"
	server, err := repos.Servers.Create(ctx, domain.Server{
		Name:           "fail-stale-running-test-" + uuid.New().String()[:8],
		Host:           "example.com",
		Port:           22,
		SSHUser:        "postgres",
		DBEngine:       domain.DBEnginePostgres,
		DeploymentMode: domain.DeploymentModeDocker,
		ContainerName:  &containerName,
		DBName:         "testdb",
		DBUser:         "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	staleRun, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create stale run: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE backup_runs SET started_at = now() - interval '1 hour' WHERE id = $1`, staleRun.ID); err != nil {
		t.Fatalf("backdate stale run: %v", err)
	}

	freshRun, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create fresh run: %v", err)
	}

	count, err := repos.BackupRuns.FailStaleRunning(ctx, 10*time.Minute, "backup marcado como falho pelo watchdog")
	if err != nil {
		t.Fatalf("FailStaleRunning: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 row affected, got %d", count)
	}

	gotStale, err := repos.BackupRuns.Get(ctx, staleRun.ID)
	if err != nil {
		t.Fatalf("get stale run: %v", err)
	}
	if gotStale.Status != domain.BackupRunStatusFailed {
		t.Errorf("stale run status = %s, want failed", gotStale.Status)
	}
	if gotStale.ErrorMessage == nil || *gotStale.ErrorMessage == "" {
		t.Error("expected stale run to have an error message set by the watchdog")
	}

	gotFresh, err := repos.BackupRuns.Get(ctx, freshRun.ID)
	if err != nil {
		t.Fatalf("get fresh run: %v", err)
	}
	if gotFresh.Status != domain.BackupRunStatusRunning {
		t.Errorf("fresh run status = %s, want running (untouched)", gotFresh.Status)
	}
}
