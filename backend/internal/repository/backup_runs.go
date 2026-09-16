package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backapeando-backup-manager/internal/domain"
)

type BackupRunRepo struct {
	pool *pgxpool.Pool
}

const backupRunColumns = `
	id, server_id, status, started_at, finished_at, blob_name,
	blob_size_bytes, dump_duration_ms, upload_duration_ms,
	error_message, log_output, created_at
`

func scanBackupRun(row pgx.Row) (domain.BackupRun, error) {
	var b domain.BackupRun
	err := row.Scan(
		&b.ID, &b.ServerID, &b.Status, &b.StartedAt, &b.FinishedAt, &b.BlobName,
		&b.BlobSizeBytes, &b.DumpDurationMS, &b.UploadDurationMS,
		&b.ErrorMessage, &b.LogOutput, &b.CreatedAt,
	)
	return b, err
}

// Create inserts a new backup_runs row with status=running and
// started_at=now — the backup-now handler creates the run immediately
// before dialing SSH, so even a connection failure is captured as a
// failed run rather than never appearing in history.
func (r *BackupRunRepo) Create(ctx context.Context, serverID string) (domain.BackupRun, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO backup_runs (server_id, status, started_at)
		VALUES ($1, 'running', now())
		RETURNING `+backupRunColumns, serverID)
	created, err := scanBackupRun(row)
	if err != nil {
		return domain.BackupRun{}, fmt.Errorf("create backup run: %w", err)
	}
	return created, nil
}

// CreateQueued inserts a new backup_runs row with status=queued and
// no started_at timestamp. Used by /api/servers/{id}/run-now endpoint
// to enqueue a backup for execution by the scheduler (Tarefa 6).
func (r *BackupRunRepo) CreateQueued(ctx context.Context, serverID string) (domain.BackupRun, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO backup_runs (server_id, status)
		VALUES ($1, 'queued')
		RETURNING `+backupRunColumns, serverID)
	created, err := scanBackupRun(row)
	if err != nil {
		return domain.BackupRun{}, fmt.Errorf("create queued backup run: %w", err)
	}
	return created, nil
}

// MarkSuccess finalizes a run as successful.
func (r *BackupRunRepo) MarkSuccess(ctx context.Context, id string, blobName string, blobSizeBytes int64, dumpDurationMS, uploadDurationMS int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE backup_runs
		SET status = 'success', finished_at = now(), blob_name = $2,
		    blob_size_bytes = $3, dump_duration_ms = $4, upload_duration_ms = $5
		WHERE id = $1
	`, id, blobName, blobSizeBytes, dumpDurationMS, uploadDurationMS)
	if err != nil {
		return fmt.Errorf("mark backup run success: %w", err)
	}
	return nil
}

// MarkFailed finalizes a run as failed with a sanitized error message. The
// caller must ensure errMsg never contains secrets (SAS token, SSH private
// key) or raw SDK/SSH internals — see MarkFailedWithDetails for the variant
// used by the scheduler and backup-now handler, which also redacts the
// known database-password secret and persists log_output.
func (r *BackupRunRepo) MarkFailed(ctx context.Context, id string, errMsg string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE backup_runs SET status = 'failed', finished_at = now(), error_message = $2
		WHERE id = $1
	`, id, errMsg)
	if err != nil {
		return fmt.Errorf("mark backup run failed: %w", err)
	}
	return nil
}

// MarkFailedWithDetails finalizes a run as failed with an error message and,
// when available, the captured stdout/stderr of the remote dump command
// (log_output). Unlike MarkFailed/UpdateStatus, the caller is expected to
// pass a real, stage-specific error message (RN-BACKUP-027) — both errMsg
// and logOutput must already be redacted of secrets and bounded in size by
// the caller (see the redactSecret/truncateText helpers in the scheduler and
// httpapi/handlers packages).
func (r *BackupRunRepo) MarkFailedWithDetails(ctx context.Context, id string, errMsg string, logOutput *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE backup_runs
		SET status = 'failed', finished_at = now(), error_message = $2, log_output = $3
		WHERE id = $1
	`, id, errMsg, logOutput)
	if err != nil {
		return fmt.Errorf("mark backup run failed with details: %w", err)
	}
	return nil
}

// FailStaleRunning marks as failed any backup_runs row still in status
// 'running' whose started_at is older than olderThan (watchdog for runs
// stuck forever behind a hung worker — see docs/RegrasNegocio.md
// RN-BACKUP-028). Returns the number of rows affected.
func (r *BackupRunRepo) FailStaleRunning(ctx context.Context, olderThan time.Duration, errMsg string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE backup_runs
		SET status = 'failed', finished_at = now(), error_message = $1
		WHERE status = 'running' AND started_at IS NOT NULL
		  AND started_at < now() - make_interval(secs => $2::double precision)
	`, errMsg, olderThan.Seconds())
	if err != nil {
		return 0, fmt.Errorf("fail stale running backup runs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// List returns a page of backup runs for a server, most recent first
// (RN-BACKUP-025). status filters to a single BackupRunStatus value when
// non-nil. total is the number of matching rows across all pages, not just
// len(runs) — queried separately from the page itself (rather than via
// COUNT(*) OVER() alongside LIMIT/OFFSET) because a page beyond the last one
// legitimately returns zero rows, and a window function has no row left to
// carry the total on in that case.
func (r *BackupRunRepo) List(ctx context.Context, serverID string, status *string, page, pageSize int) (runs []domain.BackupRun, total int, err error) {
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM backup_runs
		WHERE server_id = $1 AND ($2::text IS NULL OR status = $2)
	`, serverID, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count backup runs: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx, `
		SELECT `+backupRunColumns+`
		FROM backup_runs
		WHERE server_id = $1 AND ($2::text IS NULL OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, serverID, status, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list backup runs: %w", err)
	}
	defer rows.Close()

	var out []domain.BackupRun
	for rows.Next() {
		b, err := scanBackupRun(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan backup run: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListAll returns a page of backup runs across every server, most recent
// first, optionally filtered by serverID and/or status (both nil-able,
// following the same "$N::type IS NULL OR column = $N" pattern List already
// uses for status). Backs the "no server selected" default view of the
// History screen (T-06) — see docs/RegrasNegocio.md.
func (r *BackupRunRepo) ListAll(ctx context.Context, serverID *string, status *string, page, pageSize int) (runs []domain.BackupRun, total int, err error) {
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM backup_runs
		WHERE ($1::uuid IS NULL OR server_id = $1) AND ($2::text IS NULL OR status = $2)
	`, serverID, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count backup runs: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx, `
		SELECT `+backupRunColumns+`
		FROM backup_runs
		WHERE ($1::uuid IS NULL OR server_id = $1) AND ($2::text IS NULL OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, serverID, status, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all backup runs: %w", err)
	}
	defer rows.Close()

	var out []domain.BackupRun
	for rows.Next() {
		b, err := scanBackupRun(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan backup run: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *BackupRunRepo) Get(ctx context.Context, id string) (domain.BackupRun, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+backupRunColumns+` FROM backup_runs WHERE id = $1`, id)
	b, err := scanBackupRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BackupRun{}, ErrNotFound
	}
	if err != nil {
		return domain.BackupRun{}, fmt.Errorf("get backup run: %w", err)
	}
	return b, nil
}

// UpdateStatus updates the status of a backup run and optionally sets an error message.
// Used by scheduler and manual backup handlers to track state transitions (queued → running → success/failed).
// errorMsg must never contain secrets (SAS token, SSH key material, DB password); the caller must redact them first.
//
// Timestamps are set based on the target status: "running" sets started_at
// via COALESCE (preserving the value Create already set for cron-scheduled
// runs, filling it in for manual runs created via CreateQueued, which have
// no started_at yet); "success"/"failed" set finished_at. Without this, every
// run transitioned through this method (i.e. everything the scheduler
// processes) would keep started_at/finished_at NULL forever — see
// docs/Memoria.md for the incident this fixed.
func (r *BackupRunRepo) UpdateStatus(ctx context.Context, backupRunID string, status string, errorMsg *string) error {
	var query string
	switch status {
	case "running":
		query = `UPDATE backup_runs SET status = $2, error_message = $3, started_at = COALESCE(started_at, now()) WHERE id = $1`
	case "success", "failed":
		query = `UPDATE backup_runs SET status = $2, error_message = $3, finished_at = now() WHERE id = $1`
	default:
		query = `UPDATE backup_runs SET status = $2, error_message = $3 WHERE id = $1`
	}
	_, err := r.pool.Exec(ctx, query, backupRunID, status, errorMsg)
	if err != nil {
		return fmt.Errorf("update backup run status: %w", err)
	}
	return nil
}

// UpdateBlobMetadata updates blob_name and blob_size_bytes after a successful backup upload.
// Called after the backup blob has been written to Azure Blob Storage.
// Caller is responsible for setting finished_at and status = 'success' via MarkSuccess or UpdateStatus.
func (r *BackupRunRepo) UpdateBlobMetadata(ctx context.Context, backupRunID string, blobName string, sizeBytes int64) error {
	const query = `
		UPDATE backup_runs
		SET blob_name = $2, blob_size_bytes = $3
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, backupRunID, blobName, sizeBytes)
	if err != nil {
		return fmt.Errorf("update blob metadata: %w", err)
	}
	return nil
}

// CountByStatusSince returns the number of backup runs created at or after
// since, grouped by status. Used by the dashboard summary to compute a
// recent-window success rate. Statuses with zero runs in the window are
// simply absent from the map rather than present with a zero value.
func (r *BackupRunRepo) CountByStatusSince(ctx context.Context, since time.Time) (map[domain.BackupRunStatus]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*) FROM backup_runs WHERE created_at >= $1 GROUP BY status
	`, since)
	if err != nil {
		return nil, fmt.Errorf("count backup runs by status since: %w", err)
	}
	defer rows.Close()

	out := make(map[domain.BackupRunStatus]int)
	for rows.Next() {
		var status domain.BackupRunStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan backup run status count: %w", err)
		}
		out[status] = count
	}
	return out, rows.Err()
}

// CountAndBytesByDestination returns, for the window [since, until), one row
// per (time bucket, destination) with the run count and byte sum in that
// bucket. unit is always a compile-time constant from this package ("day" or
// "month"), never user input.
//
// Destination is resolved from the server's CURRENT storage_target_id via a
// join, not the target in effect when the run happened — see RN-BACKUP-031.
// A server with no storage_target_id configured yields
// StorageTargetID/StorageTargetName = nil.
//
// created_at is converted to UTC via "AT TIME ZONE 'UTC'" before truncation:
// plain date_trunc(unit, timestamptz) truncates in the session's timezone,
// which would silently shift day/month boundaries relative to the UTC
// windows the caller passes in since/until.
func (r *BackupRunRepo) CountAndBytesByDestination(ctx context.Context, unit string, since, until time.Time) ([]domain.BackupRunDestinationBucket, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc($1, br.created_at AT TIME ZONE 'UTC') AS bucket,
		       s.storage_target_id,
		       st.name,
		       COUNT(*) AS run_count,
		       COALESCE(SUM(br.blob_size_bytes), 0) AS total_bytes
		FROM backup_runs br
		JOIN servers s ON s.id = br.server_id
		LEFT JOIN storage_targets st ON st.id = s.storage_target_id
		WHERE br.created_at >= $2 AND br.created_at < $3
		GROUP BY bucket, s.storage_target_id, st.name
		ORDER BY bucket
	`, unit, since, until)
	if err != nil {
		return nil, fmt.Errorf("count and bytes by destination: %w", err)
	}
	defer rows.Close()

	var out []domain.BackupRunDestinationBucket
	for rows.Next() {
		var b domain.BackupRunDestinationBucket
		if err := rows.Scan(&b.Bucket, &b.StorageTargetID, &b.StorageTargetName, &b.RunCount, &b.TotalBytes); err != nil {
			return nil, fmt.Errorf("scan backup run destination bucket: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// RecentFailures returns the most recent failed backup runs across every
// server, most recent first. Used by the dashboard summary's attention
// points.
func (r *BackupRunRepo) RecentFailures(ctx context.Context, limit int) ([]domain.BackupRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+backupRunColumns+`
		FROM backup_runs WHERE status = 'failed' ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent failed backup runs: %w", err)
	}
	defer rows.Close()

	var out []domain.BackupRun
	for rows.Next() {
		b, err := scanBackupRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan backup run: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetQueuedRuns returns all backup runs with status='queued' that are waiting
// for execution. Used by the scheduler to discover manually-triggered backups
// via /api/servers/{id}/run-now endpoint (Tarefa 6).
func (r *BackupRunRepo) GetQueuedRuns(ctx context.Context) ([]domain.BackupRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+backupRunColumns+`
		FROM backup_runs WHERE status = 'queued' ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("get queued runs: %w", err)
	}
	defer rows.Close()

	var out []domain.BackupRun
	for rows.Next() {
		b, err := scanBackupRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan queued run: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
