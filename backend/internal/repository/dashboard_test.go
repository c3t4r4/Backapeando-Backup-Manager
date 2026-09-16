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

// TestServerRepo_CountByStatusAndListByStatus exercises the aggregate
// queries backing the dashboard summary's server KPIs. It uses
// floor/contains-style assertions rather than exact totals because the
// servers table is shared with other tests and any pre-existing data in a
// long-lived local database.
func TestServerRepo_CountByStatusAndListByStatus(t *testing.T) {
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
	created, err := repos.Servers.Create(ctx, domain.Server{
		Name: "dashboard-count-test-" + uuid.New().String()[:8], Host: "example.com", Port: 22,
		SSHUser: "postgres", DBEngine: domain.DBEnginePostgres, DeploymentMode: domain.DeploymentModeDocker,
		ContainerName: &containerName, DBName: "testdb", DBUser: "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, created.ID)
	})

	counts, err := repos.Servers.CountByStatus(ctx)
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts[created.Status] < 1 {
		t.Errorf("CountByStatus[%s] = %d, want at least 1", created.Status, counts[created.Status])
	}

	byStatus, err := repos.Servers.ListByStatus(ctx, created.Status, 50)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	found := false
	for _, s := range byStatus {
		if s.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListByStatus(%s) did not include created server %s", created.Status, created.ID)
	}
}

// TestServerRepo_ListWithConnectionError exercises the query backing the
// dashboard summary's "connection error" attention point.
func TestServerRepo_ListWithConnectionError(t *testing.T) {
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
	created, err := repos.Servers.Create(ctx, domain.Server{
		Name: "dashboard-conn-error-test-" + uuid.New().String()[:8], Host: "example.com", Port: 22,
		SSHUser: "postgres", DBEngine: domain.DBEnginePostgres, DeploymentMode: domain.DeploymentModeDocker,
		ContainerName: &containerName, DBName: "testdb", DBUser: "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, created.ID)
	})

	errMsg := "ssh connection test failed"
	if err := repos.Servers.RecordTestConnectionResult(ctx, created.ID, domain.ServerStatusAwaitingAuthorization, false, nil, &errMsg); err != nil {
		t.Fatalf("RecordTestConnectionResult: %v", err)
	}

	withError, err := repos.Servers.ListWithConnectionError(ctx, 50)
	if err != nil {
		t.Fatalf("ListWithConnectionError: %v", err)
	}
	found := false
	for _, s := range withError {
		if s.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListWithConnectionError did not include server %s", created.ID)
	}

	count, err := repos.Servers.CountWithConnectionError(ctx)
	if err != nil {
		t.Fatalf("CountWithConnectionError: %v", err)
	}
	if count < 1 {
		t.Errorf("CountWithConnectionError = %d, want at least 1", count)
	}
}

// TestBackupRunRepo_CountByStatusSinceAndRecentFailures exercises the
// aggregate queries backing the dashboard summary's backup-runs KPI window
// and its "recent failures" attention point.
func TestBackupRunRepo_CountByStatusSinceAndRecentFailures(t *testing.T) {
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
		Name: "dashboard-backup-run-test-" + uuid.New().String()[:8], Host: "example.com", Port: 22,
		SSHUser: "postgres", DBEngine: domain.DBEnginePostgres, DeploymentMode: domain.DeploymentModeDocker,
		ContainerName: &containerName, DBName: "testdb", DBUser: "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() {
		// backup_runs.server_id has ON DELETE CASCADE, so deleting the
		// server also removes the run created below.
		_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, server.ID)
	})

	since := time.Now().Add(-1 * time.Minute)

	run, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create backup run: %v", err)
	}
	if err := repos.BackupRuns.MarkFailed(ctx, run.ID, "boom"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	counts, err := repos.BackupRuns.CountByStatusSince(ctx, since)
	if err != nil {
		t.Fatalf("CountByStatusSince: %v", err)
	}
	if counts[domain.BackupRunStatusFailed] < 1 {
		t.Errorf("CountByStatusSince[failed] = %d, want at least 1", counts[domain.BackupRunStatusFailed])
	}

	failures, err := repos.BackupRuns.RecentFailures(ctx, 50)
	if err != nil {
		t.Fatalf("RecentFailures: %v", err)
	}
	found := false
	for _, f := range failures {
		if f.ID == run.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("RecentFailures did not include run %s", run.ID)
	}

}

// TestBackupRunRepo_CountAndBytesByDestination exercises the aggregate query
// backing the dashboard's 4 per-destination charts (RN-BACKUP-031): it must
// group by the server's CURRENT storage target, treat a server with no
// storage_target_id as its own "nil destination" group, sum bytes/counts
// across multiple runs in the same bucket, and support both "day" and
// "month" granularities.
func TestBackupRunRepo_CountAndBytesByDestination(t *testing.T) {
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

	rootPath := "/tmp/backapeando-test"
	target, err := repos.StorageTargets.Create(ctx, domain.StorageTarget{
		ID:   uuid.New().String(),
		Name: "dashboard-destination-test-" + uuid.New().String()[:8],
		Type: domain.StorageTargetTypeFilesystem, FSRootPath: &rootPath,
	})
	if err != nil {
		t.Fatalf("create storage target: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM storage_targets WHERE id = $1`, target.ID)
	})

	containerName := "testcontainer"
	serverWithTarget, err := repos.Servers.Create(ctx, domain.Server{
		Name: "dashboard-dest-with-target-" + uuid.New().String()[:8], Host: "example.com", Port: 22,
		SSHUser: "postgres", DBEngine: domain.DBEnginePostgres, DeploymentMode: domain.DeploymentModeDocker,
		ContainerName: &containerName, DBName: "testdb", DBUser: "testuser",
		CronExpression: "0 3 * * *", StorageTargetID: &target.ID,
	})
	if err != nil {
		t.Fatalf("create server with target: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, serverWithTarget.ID)
	})

	serverWithoutTarget, err := repos.Servers.Create(ctx, domain.Server{
		Name: "dashboard-dest-no-target-" + uuid.New().String()[:8], Host: "example.com", Port: 22,
		SSHUser: "postgres", DBEngine: domain.DBEnginePostgres, DeploymentMode: domain.DeploymentModeDocker,
		ContainerName: &containerName, DBName: "testdb", DBUser: "testuser",
		CronExpression: "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server without target: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM servers WHERE id = $1`, serverWithoutTarget.ID)
	})

	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	until := since.AddDate(0, 0, 1)

	// Two runs on the same server/day must be summed into a single group.
	for i := 0; i < 2; i++ {
		run, err := repos.BackupRuns.Create(ctx, serverWithTarget.ID)
		if err != nil {
			t.Fatalf("create backup run (with target): %v", err)
		}
		if err := repos.BackupRuns.MarkSuccess(ctx, run.ID, "backup.dump", 1000, 100, 100); err != nil {
			t.Fatalf("mark success (with target): %v", err)
		}
	}

	runNoTarget, err := repos.BackupRuns.Create(ctx, serverWithoutTarget.ID)
	if err != nil {
		t.Fatalf("create backup run (no target): %v", err)
	}
	if err := repos.BackupRuns.MarkSuccess(ctx, runNoTarget.ID, "backup.dump", 500, 50, 50); err != nil {
		t.Fatalf("mark success (no target): %v", err)
	}

	t.Run("day granularity groups by current destination and sums bytes/counts", func(t *testing.T) {
		buckets, err := repos.BackupRuns.CountAndBytesByDestination(ctx, "day", since, until)
		if err != nil {
			t.Fatalf("CountAndBytesByDestination: %v", err)
		}

		var withTarget, withoutTarget *domain.BackupRunDestinationBucket
		for i := range buckets {
			b := &buckets[i]
			if b.StorageTargetID != nil && *b.StorageTargetID == target.ID {
				withTarget = b
			}
			if b.StorageTargetID == nil {
				withoutTarget = b
			}
		}

		if withTarget == nil {
			t.Fatalf("no bucket found for storage target %s", target.ID)
		}
		if withTarget.RunCount < 2 {
			t.Errorf("RunCount = %d, want at least 2 (two runs summed)", withTarget.RunCount)
		}
		if withTarget.TotalBytes < 2000 {
			t.Errorf("TotalBytes = %d, want at least 2000 (two runs of 1000 bytes summed)", withTarget.TotalBytes)
		}
		if withTarget.StorageTargetName == nil || *withTarget.StorageTargetName != target.Name {
			t.Errorf("StorageTargetName = %v, want %q", withTarget.StorageTargetName, target.Name)
		}

		if withoutTarget == nil {
			t.Fatalf("no bucket found for nil destination (server without storage target)")
		}
		if withoutTarget.RunCount < 1 {
			t.Errorf("RunCount = %d, want at least 1", withoutTarget.RunCount)
		}
		if withoutTarget.TotalBytes < 500 {
			t.Errorf("TotalBytes = %d, want at least 500", withoutTarget.TotalBytes)
		}
	})

	t.Run("month granularity buckets at the first day of the month", func(t *testing.T) {
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		monthEnd := monthStart.AddDate(0, 1, 0)

		buckets, err := repos.BackupRuns.CountAndBytesByDestination(ctx, "month", monthStart, monthEnd)
		if err != nil {
			t.Fatalf("CountAndBytesByDestination (month): %v", err)
		}
		found := false
		for _, b := range buckets {
			if b.StorageTargetID != nil && *b.StorageTargetID == target.ID {
				found = true
				if !b.Bucket.Equal(monthStart) {
					t.Errorf("Bucket = %v, want %v (first day of month)", b.Bucket, monthStart)
				}
			}
		}
		if !found {
			t.Errorf("no monthly bucket found for storage target %s", target.ID)
		}
	})

	t.Run("empty window returns no rows and no error", func(t *testing.T) {
		farPast := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		farPastEnd := farPast.AddDate(0, 0, 1)
		buckets, err := repos.BackupRuns.CountAndBytesByDestination(ctx, "day", farPast, farPastEnd)
		if err != nil {
			t.Fatalf("CountAndBytesByDestination (empty window): %v", err)
		}
		if len(buckets) != 0 {
			t.Errorf("len(buckets) = %d, want 0 for a window with no runs", len(buckets))
		}
	})
}
