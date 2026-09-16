package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"backapeando-backup-manager/internal/domain"
)

type RetentionDeletionRepo struct {
	pool *pgxpool.Pool
}

// Create records an audit entry for a blob that was deleted by the
// retention sweep. This is the durable trail proving an exclusion actually
// happened; it must be written for every real (non-dry-run) deletion.
func (r *RetentionDeletionRepo) Create(ctx context.Context, serverID string, backupRunID *string, blobName string, reason string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO retention_deletions (server_id, backup_run_id, blob_name, reason)
		VALUES ($1, $2, $3, $4)
	`, serverID, backupRunID, blobName, reason)
	if err != nil {
		return fmt.Errorf("create retention deletion: %w", err)
	}
	return nil
}

// ListByServer returns the deletion audit trail for a server, most recent first.
func (r *RetentionDeletionRepo) ListByServer(ctx context.Context, serverID string) ([]domain.RetentionDeletion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, server_id, backup_run_id, blob_name, deleted_at, reason
		FROM retention_deletions WHERE server_id = $1 ORDER BY deleted_at DESC
	`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list retention deletions: %w", err)
	}
	defer rows.Close()

	var out []domain.RetentionDeletion
	for rows.Next() {
		var d domain.RetentionDeletion
		if err := rows.Scan(&d.ID, &d.ServerID, &d.BackupRunID, &d.BlobName, &d.DeletedAt, &d.Reason); err != nil {
			return nil, fmt.Errorf("scan retention deletion: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
