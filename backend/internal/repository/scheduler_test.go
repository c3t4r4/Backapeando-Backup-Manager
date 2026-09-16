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

// TestSchedulerMethods tests Phase 4 scheduler-specific repository methods.
// Requires DATABASE_URL to be set; skipped otherwise.
func TestSchedulerMethods(t *testing.T) {
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

	// Create a storage target (required for server creation)
	accountName, containerName := "testaccount", "testcontainer"
	storageTarget := domain.StorageTarget{
		ID:                     uuid.NewString(),
		Name:                   "test-azure-" + uuid.New().String()[:8],
		Type:                   domain.StorageTargetTypeAzure,
		AzureAccountName:       &accountName,
		AzureContainerName:     &containerName,
		AzureSASTokenEncrypted: []byte("dummy-encrypted-sas-token"),
	}
	createdTarget, err := repos.StorageTargets.Create(ctx, storageTarget)
	if err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	// Create a server
	serverInput := domain.Server{
		Name:            "scheduler-test-server-" + uuid.New().String()[:8],
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
	}
	server, err := repos.Servers.Create(ctx, serverInput)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() {
		_ = repos.Servers.Delete(context.Background(), server.ID)
		_ = repos.StorageTargets.Delete(context.Background(), createdTarget.ID)
	})

	// Simulate SSH key setup and authorization (transition to ready)
	privKey := []byte("dummy-encrypted-private-key")
	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGvjc8PnBW3K..."
	fingerprint := "SHA256:abc123"
	if err := repos.Servers.SetSSHKeyPair(ctx, server.ID, privKey, pubKey, fingerprint); err != nil {
		t.Fatalf("set ssh key pair: %v", err)
	}

	// Simulate successful test connection (transition to ready, auto-enable)
	nextRun := time.Now().Add(3 * time.Hour)
	if err := repos.Servers.RecordTestConnectionResult(ctx, server.ID, domain.ServerStatusReady, true, &nextRun, nil); err != nil {
		t.Fatalf("record test connection result: %v", err)
	}

	// Verify server is ready
	readyServer, err := repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if readyServer.Status != domain.ServerStatusReady {
		t.Errorf("expected status ready, got %s", readyServer.Status)
	}
	if !readyServer.Enabled {
		t.Error("expected enabled=true after test connection success")
	}

	// Test GetReadyServersForScheduling with next_run_at in past
	pastTime := time.Now().Add(-1 * time.Hour)
	if err := repos.Servers.UpdateNextRun(ctx, server.ID, pastTime); err != nil {
		t.Fatalf("update next run (past): %v", err)
	}

	readyServers, err := repos.Servers.GetReadyServersForScheduling(ctx, 10)
	if err != nil {
		t.Fatalf("get ready servers for scheduling: %v", err)
	}
	if len(readyServers) == 0 {
		t.Fatal("expected to find ready servers with past next_run_at")
	}
	found := false
	for _, s := range readyServers {
		if s.ID == server.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find our test server in ready list")
	}

	// Test UpdateNextRun with future time
	futureTime := time.Now().Add(24 * time.Hour)
	if err := repos.Servers.UpdateNextRun(ctx, server.ID, futureTime); err != nil {
		t.Fatalf("update next run (future): %v", err)
	}

	updatedServer, err := repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("get server after update next run: %v", err)
	}
	if updatedServer.NextRunAt == nil {
		t.Fatal("expected NextRunAt to be set")
	}
	// Allow 5-second tolerance for timing
	if updatedServer.NextRunAt.Sub(futureTime).Abs() > 5*time.Second {
		t.Errorf("expected NextRunAt ~%v, got %v", futureTime, updatedServer.NextRunAt)
	}
	if updatedServer.LastScheduledAt == nil {
		t.Fatal("expected LastScheduledAt to be set after UpdateNextRun")
	}

	// Verify server is no longer in ready list (next_run_at is in future)
	readyServersAgain, err := repos.Servers.GetReadyServersForScheduling(ctx, 10)
	if err != nil {
		t.Fatalf("get ready servers after update: %v", err)
	}
	for _, s := range readyServersAgain {
		if s.ID == server.ID {
			t.Fatal("expected server to be excluded after updating NextRunAt to future")
		}
	}

	// Test backup run status and blob metadata updates
	backup, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create backup run: %v", err)
	}

	// Test UpdateStatus
	errMsg := "test error message"
	if err := repos.BackupRuns.UpdateStatus(ctx, backup.ID, string(domain.BackupRunStatusFailed), &errMsg); err != nil {
		t.Fatalf("update status: %v", err)
	}

	updatedBackup, err := repos.BackupRuns.Get(ctx, backup.ID)
	if err != nil {
		t.Fatalf("get backup after status update: %v", err)
	}
	if updatedBackup.Status != domain.BackupRunStatusFailed {
		t.Errorf("expected status failed, got %s", updatedBackup.Status)
	}
	if updatedBackup.ErrorMessage == nil || *updatedBackup.ErrorMessage != errMsg {
		t.Errorf("expected error message %q, got %v", errMsg, updatedBackup.ErrorMessage)
	}

	// Create another backup for blob metadata test
	backup2, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create second backup run: %v", err)
	}

	// Test UpdateBlobMetadata
	blobName := "backup-2024-01-15.sql.gz"
	blobSize := int64(1024 * 1024 * 500) // 500 MB
	if err := repos.BackupRuns.UpdateBlobMetadata(ctx, backup2.ID, blobName, blobSize); err != nil {
		t.Fatalf("update blob metadata: %v", err)
	}

	updatedBackup2, err := repos.BackupRuns.Get(ctx, backup2.ID)
	if err != nil {
		t.Fatalf("get backup after blob update: %v", err)
	}
	if updatedBackup2.BlobName == nil || *updatedBackup2.BlobName != blobName {
		t.Errorf("expected blob name %q, got %v", blobName, updatedBackup2.BlobName)
	}
	if updatedBackup2.BlobSizeBytes == nil || *updatedBackup2.BlobSizeBytes != blobSize {
		t.Errorf("expected blob size %d, got %v", blobSize, updatedBackup2.BlobSizeBytes)
	}

	t.Logf("All Phase 4 scheduler tests passed")
}
