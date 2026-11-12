package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

func TestCreate_LocalServerPersistsPendingLocalWithoutSSHKey(t *testing.T) {
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
	containerName := "pg"
	created, err := repos.Servers.Create(ctx, domain.Server{
		ID:             uuid.New().String(),
		Name:           "local-" + uuid.New().String()[:8],
		Host:           "localhost",
		Port:           0,
		SSHUser:        "",
		ConnectionMode: domain.ConnectionModeLocal,
		DBEngine:       domain.DBEnginePostgres,
		DeploymentMode: domain.DeploymentModeDocker,
		ContainerName:  &containerName,
		DBName:         "app",
		DBUser:         "backup",
		CronExpression: "0 3 * * *",
		Status:         domain.ServerStatusPendingLocal,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ConnectionMode != domain.ConnectionModeLocal {
		t.Fatalf("connection mode = %q", created.ConnectionMode)
	}
	if created.Status != domain.ServerStatusPendingLocal {
		t.Fatalf("status = %q, want pending_local", created.Status)
	}
	if created.SSHPublicKey != nil || created.SSHPrivateKeyEncrypted != nil {
		t.Fatal("local server must be stored without an SSH key")
	}
}
