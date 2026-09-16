package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backapeando-backup-manager/internal/domain"
)

type ServerRepo struct {
	pool *pgxpool.Pool
}

const serverColumns = `
	id, name, host, port, ssh_user, db_engine, deployment_mode, container_name, db_name, db_user,
	pg_dump_extra_args, mysql_dump_extra_args, sqlcmd_extra_args, db_password_encrypted,
	ssh_private_key_encrypted, ssh_public_key, ssh_key_fingerprint, ssh_host_key_fingerprint,
	storage_target_id, cron_expression, enabled, status,
	last_test_connection_at, last_test_connection_ok, last_test_connection_error, next_run_at, last_scheduled_at, created_at, updated_at
`

func scanServer(row pgx.Row) (domain.Server, error) {
	var s domain.Server
	err := row.Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.SSHUser, &s.DBEngine, &s.DeploymentMode, &s.ContainerName, &s.DBName, &s.DBUser,
		&s.PgDumpExtraArgs, &s.MySQLDumpExtraArgs, &s.SqlCmdExtraArgs, &s.DBPasswordEncrypted,
		&s.SSHPrivateKeyEncrypted, &s.SSHPublicKey, &s.SSHKeyFingerprint, &s.SSHHostKeyFingerprint,
		&s.StorageTargetID, &s.CronExpression, &s.Enabled, &s.Status,
		&s.LastTestConnectionAt, &s.LastTestConnectionOK, &s.LastTestConnectionError, &s.NextRunAt, &s.LastScheduledAt, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

// Create inserts a server, including DBPasswordEncrypted when the caller
// already has it (it must be included in this same INSERT: the
// servers_password_required_by_engine CHECK constraint rejects a
// non-postgres row with a NULL password at insert time, so a later
// SetDBPassword UPDATE — which requires an ID that only exists after
// INSERT — is too late for non-postgres engines). The ID is generated here
// (not left to the column's DB-side default) so the caller can encrypt the
// password against the AAD (server ID) before this call. SSH key material
// has no equivalent CHECK constraint, so it is still populated later via
// SetSSHKeyPair once generated.
func (r *ServerRepo) Create(ctx context.Context, s domain.Server) (domain.Server, error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO servers (id, name, host, port, ssh_user, db_engine, deployment_mode, container_name, db_name, db_user,
		                      pg_dump_extra_args, mysql_dump_extra_args, sqlcmd_extra_args, db_password_encrypted,
		                      storage_target_id, cron_expression)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING `+serverColumns, s.ID, s.Name, s.Host, s.Port, s.SSHUser, s.DBEngine, s.DeploymentMode, s.ContainerName, s.DBName, s.DBUser,
		s.PgDumpExtraArgs, s.MySQLDumpExtraArgs, s.SqlCmdExtraArgs, s.DBPasswordEncrypted, s.StorageTargetID, s.CronExpression)
	created, err := scanServer(row)
	if err != nil {
		return domain.Server{}, fmt.Errorf("create server: %w", err)
	}
	return created, nil
}

func (r *ServerRepo) List(ctx context.Context) ([]domain.Server, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serverColumns+` FROM servers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()

	var out []domain.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ServerRepo) Get(ctx context.Context, id string) (domain.Server, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+serverColumns+` FROM servers WHERE id = $1`, id)
	s, err := scanServer(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Server{}, ErrNotFound
	}
	if err != nil {
		return domain.Server{}, fmt.Errorf("get server: %w", err)
	}
	return s, nil
}

// Update modifies the non-secret, user-editable fields of a server. It does
// not touch SSH key material, status, or scheduling state — those are
// mutated by their own dedicated methods so each has a single writer.
func (r *ServerRepo) Update(ctx context.Context, s domain.Server) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE servers
		SET name = $2, host = $3, port = $4, ssh_user = $5, db_engine = $6, deployment_mode = $7,
		    container_name = $8, db_name = $9, db_user = $10, pg_dump_extra_args = $11,
		    mysql_dump_extra_args = $12, sqlcmd_extra_args = $13,
		    storage_target_id = $14, cron_expression = $15, updated_at = now()
		WHERE id = $1
	`, s.ID, s.Name, s.Host, s.Port, s.SSHUser, s.DBEngine, s.DeploymentMode,
		s.ContainerName, s.DBName, s.DBUser, s.PgDumpExtraArgs,
		s.MySQLDumpExtraArgs, s.SqlCmdExtraArgs, s.StorageTargetID, s.CronExpression)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	return nil
}

func (r *ServerRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM servers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	return nil
}

// SetSSHKeyPair stores newly generated SSH key material and moves the
// server into awaiting_authorization. Used by Phase 2 (key generation) and
// by "regenerate key".
func (r *ServerRepo) SetSSHKeyPair(ctx context.Context, id string, encryptedPrivateKey []byte, publicKey, fingerprint string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE servers
		SET ssh_private_key_encrypted = $2, ssh_public_key = $3, ssh_key_fingerprint = $4,
		    status = 'awaiting_authorization', enabled = false, updated_at = now()
		WHERE id = $1
	`, id, encryptedPrivateKey, publicKey, fingerprint)
	if err != nil {
		return fmt.Errorf("set ssh key pair: %w", err)
	}
	return nil
}

// SetEnabled toggles the server's manual on/off switch (RN-BACKUP-024),
// independent of Status — disabling only stops the automatic scheduler from
// claiming the server (RN-BACKUP-002); manual backups (run-now/backup-now)
// remain available either way.
func (r *ServerRepo) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE servers SET enabled = $2, updated_at = now() WHERE id = $1
	`, id, enabled)
	if err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}
	return nil
}

// RecordTestConnectionResult records the outcome of a test-connection attempt,
// including status transition (e.g., awaiting_authorization → ready),
// auto-enable on first success, and any error message.
func (r *ServerRepo) RecordTestConnectionResult(ctx context.Context, id string, status domain.ServerStatus, enabled bool, nextRunAt *time.Time, testError *string) error {
	// $5 is cast explicitly (::text) because it's reused in two different
	// expression contexts below (a plain assignment and an `IS NULL`
	// check) — without the cast, Postgres/pgx cannot always determine a
	// type for the placeholder from context alone and rejects the query
	// with "could not determine data type of parameter $5" (SQLSTATE
	// 42P08), regardless of whether the Go-side value is nil or set.
	const query = `
		UPDATE servers SET status = $2, enabled = $3, next_run_at = $4, last_test_connection_error = $5::text,
		                   last_test_connection_at = now(), last_test_connection_ok = ($5::text IS NULL),
		                   updated_at = now()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, status, enabled, nextRunAt, testError)
	if err != nil {
		return fmt.Errorf("record test connection result: %w", err)
	}
	return nil
}

// SetDBPassword stores the encrypted DB password (RN-BACKUP-019: only ever
// called when the operator submitted a non-empty password — an empty
// password in a PUT request means "keep the current one", so this method is
// simply not called in that case, leaving the existing value untouched).
func (r *ServerRepo) SetDBPassword(ctx context.Context, id string, encryptedPassword []byte) error {
	const query = `
		UPDATE servers SET db_password_encrypted = $2, updated_at = now() WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, encryptedPassword)
	if err != nil {
		return fmt.Errorf("set db password: %w", err)
	}
	return nil
}

// SetHostKeyFingerprint stores the SSH host key fingerprint (TOFU pinning).
func (r *ServerRepo) SetHostKeyFingerprint(ctx context.Context, id string, fingerprint string) error {
	const query = `
		UPDATE servers SET ssh_host_key_fingerprint = $2, updated_at = now() WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, fingerprint)
	if err != nil {
		return fmt.Errorf("set host key fingerprint: %w", err)
	}
	return nil
}

// ResetHostKeyFingerprint clears the pinned host key fingerprint (e.g., for re-authorization).
func (r *ServerRepo) ResetHostKeyFingerprint(ctx context.Context, id string) error {
	const query = `
		UPDATE servers SET ssh_host_key_fingerprint = NULL, updated_at = now() WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("reset host key fingerprint: %w", err)
	}
	return nil
}

// GetReadyServersForScheduling returns servers that are eligible for scheduling.
// Criteria: enabled=true, status=ready, next_run_at <= now(), and the server
// has no backup_runs row currently in 'running'/'queued' (RN-BACKUP-028): a
// prior run for the same server that is still in flight — legitimately or
// stuck behind a hung worker — must finish (or be failed by the watchdog/
// task timeout) before another one is claimed, otherwise a stuck run gets a
// new one stacked behind it on every cron tick, forever.
// Uses FOR UPDATE SKIP LOCKED to prevent concurrent scheduler workers from
// claiming the same server simultaneously. The caller is responsible for
// parsing the cron expression and computing the next run time.
func (r *ServerRepo) GetReadyServersForScheduling(ctx context.Context, limit int) ([]*domain.Server, error) {
	const query = `
		SELECT ` + serverColumns + `
		FROM servers
		WHERE enabled = true AND status = 'ready' AND next_run_at IS NOT NULL AND next_run_at <= now()
		  AND NOT EXISTS (
		    SELECT 1 FROM backup_runs br
		    WHERE br.server_id = servers.id AND br.status IN ('running', 'queued')
		  )
		ORDER BY next_run_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get ready servers for scheduling: %w", err)
	}
	defer rows.Close()

	var out []*domain.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get ready servers for scheduling rows error: %w", err)
	}
	return out, nil
}

// CountByStatus returns the number of servers in each status, for the
// dashboard summary. Statuses with zero servers are simply absent from the
// map rather than present with a zero value.
func (r *ServerRepo) CountByStatus(ctx context.Context) (map[domain.ServerStatus]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT status, COUNT(*) FROM servers GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("count servers by status: %w", err)
	}
	defer rows.Close()

	out := make(map[domain.ServerStatus]int)
	for rows.Next() {
		var status domain.ServerStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan server status count: %w", err)
		}
		out[status] = count
	}
	return out, rows.Err()
}

// ListByStatus returns up to limit servers with the given status, ordered
// by name. Used by the dashboard summary to surface servers stuck in
// awaiting_authorization as an attention point — the exact total per status
// comes from CountByStatus, not from the length of this (capped) list.
func (r *ServerRepo) ListByStatus(ctx context.Context, status domain.ServerStatus, limit int) ([]domain.Server, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serverColumns+` FROM servers WHERE status = $1 ORDER BY name LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list servers by status: %w", err)
	}
	defer rows.Close()

	var out []domain.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CountWithConnectionError returns the exact number of servers whose last
// test-connection attempt failed. Used for the dashboard summary's KPI —
// kept separate from ListWithConnectionError (which is capped for the
// attention-points list) so the KPI is never silently truncated.
func (r *ServerRepo) CountWithConnectionError(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM servers WHERE last_test_connection_ok = false`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count servers with connection error: %w", err)
	}
	return count, nil
}

// ListWithConnectionError returns up to limit servers whose last
// test-connection attempt failed, ordered by name. Used by the dashboard
// summary's attention points — the exact total comes from
// CountWithConnectionError, not from the length of this (capped) list.
func (r *ServerRepo) ListWithConnectionError(ctx context.Context, limit int) ([]domain.Server, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+serverColumns+` FROM servers WHERE last_test_connection_ok = false ORDER BY name LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list servers with connection error: %w", err)
	}
	defer rows.Close()

	var out []domain.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateNextRun updates next_run_at and last_scheduled_at atomically.
// Called after a backup run completes (successfully or not) to schedule
// the next execution time. The scheduler computes nextTime from the cron
// expression and this method persists it.
func (r *ServerRepo) UpdateNextRun(ctx context.Context, serverID string, nextTime time.Time) error {
	const query = `
		UPDATE servers
		SET next_run_at = $2, last_scheduled_at = now(), updated_at = now()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, serverID, nextTime)
	if err != nil {
		return fmt.Errorf("update next run: %w", err)
	}
	return nil
}

// RescheduleNextRunAt updates only next_run_at, leaving last_scheduled_at
// untouched. Used when an operator edits a server's cron_expression
// (RN-BACKUP-030): unlike UpdateNextRun, this is not a real scheduler claim,
// so it must not make last_scheduled_at lie about when the scheduler last
// actually touched this server.
func (r *ServerRepo) RescheduleNextRunAt(ctx context.Context, serverID string, nextTime time.Time) error {
	const query = `
		UPDATE servers
		SET next_run_at = $2, updated_at = now()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, serverID, nextTime)
	if err != nil {
		return fmt.Errorf("reschedule next run at: %w", err)
	}
	return nil
}
