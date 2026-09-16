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

// TestGetReadyServersForScheduling_SkipsServerWithInFlightRun is a
// regression test for RN-BACKUP-028: before this fix, the scheduler could
// claim a server for a new run even while a previous run for that same
// server was still 'running' (legitimately, or stuck behind a hung worker)
// or 'queued' — stacking a new backup_runs row behind the stuck one on every
// cron tick, forever. GetReadyServersForScheduling must now exclude any
// server that already has an in-flight run.
func TestGetReadyServersForScheduling_SkipsServerWithInFlightRun(t *testing.T) {
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

	fsPath := "/tmp/backapeando-test"
	target, err := repos.StorageTargets.Create(ctx, domain.StorageTarget{
		ID:         uuid.New().String(),
		Name:       "dedup-test-target-" + uuid.New().String()[:8],
		Type:       domain.StorageTargetTypeFilesystem,
		FSRootPath: &fsPath,
	})
	if err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	containerName := "testcontainer"
	makeReadyServer := func(name string) domain.Server {
		s, err := repos.Servers.Create(ctx, domain.Server{
			Name:            name + "-" + uuid.New().String()[:8],
			Host:            "example.com",
			Port:            22,
			SSHUser:         "postgres",
			DBEngine:        domain.DBEnginePostgres,
			DeploymentMode:  domain.DeploymentModeDocker,
			ContainerName:   &containerName,
			DBName:          "testdb",
			DBUser:          "testuser",
			CronExpression:  "0 3 * * *",
			StorageTargetID: &target.ID,
		})
		if err != nil {
			t.Fatalf("create server %q: %v", name, err)
		}
		// Force the server into the scheduler-eligible state directly (no
		// public repository method sets status/enabled/next_run_at all at
		// once outside the real test-connection/enable flows).
		if _, err := pool.Exec(ctx, `
			UPDATE servers SET enabled = true, status = 'ready', next_run_at = now() - interval '1 minute'
			WHERE id = $1
		`, s.ID); err != nil {
			t.Fatalf("force server %q ready: %v", name, err)
		}
		return s
	}

	blockedServer := makeReadyServer("dedup-blocked")
	freeServer := makeReadyServer("dedup-free")

	if _, err := repos.BackupRuns.Create(ctx, blockedServer.ID); err != nil {
		t.Fatalf("create in-flight run for blocked server: %v", err)
	}

	claimed, err := repos.Servers.GetReadyServersForScheduling(ctx, 10)
	if err != nil {
		t.Fatalf("get ready servers: %v", err)
	}

	var sawBlocked, sawFree bool
	for _, s := range claimed {
		if s.ID == blockedServer.ID {
			sawBlocked = true
		}
		if s.ID == freeServer.ID {
			sawFree = true
		}
	}
	if sawBlocked {
		t.Error("expected the server with an in-flight run to be excluded from scheduling, but it was claimed")
	}
	if !sawFree {
		t.Error("expected the server with no in-flight run to still be claimed")
	}
}

// TestRecordTestConnectionResult_NilAndNonNilError is a regression test for
// a bug found via live E2E testing of RN-BACKUP-013 (combined test-connection):
// the query underlying RecordTestConnectionResult reused the same
// placeholder ($5) in two different expression contexts (a plain
// assignment and an `IS NULL` check), which Postgres/pgx could not always
// resolve a type for — failing with "could not determine data type of
// parameter $5" (SQLSTATE 42P08), for BOTH a nil and a non-nil testError.
// This had never been caught because this exact call shape (a real
// Postgres, not a mock) was only ever exercised by an integration test
// gated behind DATABASE_URL, which nobody had run against a live database
// until this session. Requires DATABASE_URL to be set; skipped otherwise.
func TestRecordTestConnectionResult_NilAndNonNilError(t *testing.T) {
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
		Name:           "record-test-conn-" + uuid.New().String()[:8],
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

	t.Run("nil testError (success path)", func(t *testing.T) {
		nextRun := time.Now().Add(3 * time.Hour)
		err := repos.Servers.RecordTestConnectionResult(ctx, server.ID, domain.ServerStatusReady, true, &nextRun, nil)
		if err != nil {
			t.Fatalf("RecordTestConnectionResult with nil testError: %v", err)
		}

		got, err := repos.Servers.Get(ctx, server.ID)
		if err != nil {
			t.Fatalf("get server: %v", err)
		}
		if got.Status != domain.ServerStatusReady {
			t.Errorf("status = %s, want ready", got.Status)
		}
		if got.LastTestConnectionError != nil {
			t.Errorf("expected nil LastTestConnectionError, got %v", *got.LastTestConnectionError)
		}
		if got.LastTestConnectionOK == nil || !*got.LastTestConnectionOK {
			t.Error("expected LastTestConnectionOK=true")
		}
	})

	t.Run("non-nil testError (failure path)", func(t *testing.T) {
		composed := "SSH: ssh connection test failed; see server logs for details; Docker: not attempted: SSH connection failed"
		err := repos.Servers.RecordTestConnectionResult(ctx, server.ID, domain.ServerStatusAwaitingAuthorization, false, nil, &composed)
		if err != nil {
			t.Fatalf("RecordTestConnectionResult with non-nil testError: %v", err)
		}

		got, err := repos.Servers.Get(ctx, server.ID)
		if err != nil {
			t.Fatalf("get server: %v", err)
		}
		if got.LastTestConnectionError == nil || *got.LastTestConnectionError != composed {
			t.Errorf("LastTestConnectionError = %v, want %q", got.LastTestConnectionError, composed)
		}
		if got.LastTestConnectionOK == nil || *got.LastTestConnectionOK {
			t.Error("expected LastTestConnectionOK=false")
		}
	})
}

// TestSetDBPassword confirms the update is scoped to db_password_encrypted
// only — it must not touch status/enabled (unlike SetSSHKeyPair, which
// deliberately does) and must be readable back byte-for-byte via Get.
func TestSetDBPassword(t *testing.T) {
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

	// MySQL/SQL Server rows must carry a non-NULL db_password_encrypted from
	// the moment they're inserted (servers_password_required_by_engine CHECK,
	// migration 000006) — Create is the only place that can satisfy it,
	// since it's the only write that happens before the row exists.
	initialPassword := []byte("initial-encrypted-db-password-bytes")
	containerName := "testcontainer"
	server, err := repos.Servers.Create(ctx, domain.Server{
		Name:                "set-db-password-test-" + uuid.New().String()[:8],
		Host:                "example.com",
		Port:                22,
		SSHUser:             "postgres",
		DBEngine:            domain.DBEngineMySQL,
		DeploymentMode:      domain.DeploymentModeDocker,
		ContainerName:       &containerName,
		DBName:              "testdb",
		DBUser:              "testuser",
		DBPasswordEncrypted: initialPassword,
		CronExpression:      "0 3 * * *",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	if string(server.DBPasswordEncrypted) != string(initialPassword) {
		t.Fatalf("expected DBPasswordEncrypted set right after Create, got %v", server.DBPasswordEncrypted)
	}

	// SetDBPassword is exercised here as the Update-path re-keying operation
	// (RN-BACKUP-019), not as the only way to set a password for the first time.
	encrypted := []byte("fake-encrypted-db-password-bytes")
	if err := repos.Servers.SetDBPassword(ctx, server.ID, encrypted); err != nil {
		t.Fatalf("SetDBPassword: %v", err)
	}

	got, err := repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if string(got.DBPasswordEncrypted) != string(encrypted) {
		t.Errorf("DBPasswordEncrypted = %v, want %v", got.DBPasswordEncrypted, encrypted)
	}
	// SetDBPassword must not have side effects on unrelated fields.
	if got.Status != server.Status || got.Enabled != server.Enabled {
		t.Errorf("SetDBPassword unexpectedly changed status/enabled: got status=%s enabled=%v, want status=%s enabled=%v",
			got.Status, got.Enabled, server.Status, server.Enabled)
	}
}

// TestSetEnabled confirms the manual on/off toggle (RN-BACKUP-024) flips
// only Enabled — Status must be untouched either way, unlike SetSSHKeyPair
// (which deliberately resets both).
func TestSetEnabled(t *testing.T) {
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
		Name:           "set-enabled-test-" + uuid.New().String()[:8],
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
	if server.Enabled {
		t.Fatalf("expected newly created server to start disabled, got enabled=true")
	}
	initialStatus := server.Status

	if err := repos.Servers.SetEnabled(ctx, server.ID, true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	got, err := repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if !got.Enabled {
		t.Error("expected Enabled=true after SetEnabled(true)")
	}
	if got.Status != initialStatus {
		t.Errorf("SetEnabled unexpectedly changed status: got %s, want %s", got.Status, initialStatus)
	}

	if err := repos.Servers.SetEnabled(ctx, server.ID, false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	got, err = repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.Enabled {
		t.Error("expected Enabled=false after SetEnabled(false)")
	}
	if got.Status != initialStatus {
		t.Errorf("SetEnabled unexpectedly changed status: got %s, want %s", got.Status, initialStatus)
	}
}

// TestRescheduleNextRunAt_DoesNotTouchLastScheduledAt is a regression test
// for a Validator finding on RN-BACKUP-030: UpdateNextRun also bumps
// last_scheduled_at, which is correct for its real caller (claimAndEnqueue,
// after an actual scheduler claim) but would be a lie if reused for a manual
// cron edit via PUT /api/servers/{id} — no scheduler claim happened, so
// last_scheduled_at must stay whatever it already was.
func TestRescheduleNextRunAt_DoesNotTouchLastScheduledAt(t *testing.T) {
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
		Name:           "reschedule-next-run-test-" + uuid.New().String()[:8],
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
	if server.LastScheduledAt != nil {
		t.Fatalf("expected newly created server to start with LastScheduledAt=nil, got %v", server.LastScheduledAt)
	}

	// Simulate a real scheduler claim first, so LastScheduledAt is non-nil —
	// the interesting assertion is that RescheduleNextRunAt leaves this
	// pre-existing value untouched, not that it stays nil.
	firstRun := time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)
	if err := repos.Servers.UpdateNextRun(ctx, server.ID, firstRun); err != nil {
		t.Fatalf("UpdateNextRun: %v", err)
	}
	afterClaim, err := repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if afterClaim.LastScheduledAt == nil {
		t.Fatal("expected LastScheduledAt to be set after UpdateNextRun (a real scheduler claim)")
	}
	lastScheduledAtAfterClaim := *afterClaim.LastScheduledAt

	// Now simulate an operator editing the cron via PUT — this must update
	// next_run_at without touching last_scheduled_at.
	newNextRun := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	if err := repos.Servers.RescheduleNextRunAt(ctx, server.ID, newNextRun); err != nil {
		t.Fatalf("RescheduleNextRunAt: %v", err)
	}

	got, err := repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.NextRunAt == nil || !got.NextRunAt.Equal(newNextRun) {
		t.Errorf("expected NextRunAt=%v, got %v", newNextRun, got.NextRunAt)
	}
	if got.LastScheduledAt == nil || !got.LastScheduledAt.Equal(lastScheduledAtAfterClaim) {
		t.Errorf("expected LastScheduledAt to remain %v (untouched by RescheduleNextRunAt), got %v", lastScheduledAtAfterClaim, got.LastScheduledAt)
	}
}
