package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backapeando-backup-manager/internal/domain"
)

type RetentionSweepRequestRepo struct {
	pool *pgxpool.Pool
}

func (r *RetentionSweepRequestRepo) Create(ctx context.Context) (domain.RetentionSweepRequest, error) {
	var req domain.RetentionSweepRequest
	err := r.pool.QueryRow(ctx, `
		INSERT INTO retention_sweep_requests (status, requested_at, created_at, updated_at)
		VALUES ('pending', NOW(), NOW(), NOW())
		RETURNING id, status, requested_at, created_at, updated_at
	`).Scan(&req.ID, &req.Status, &req.RequestedAt, &req.CreatedAt, &req.UpdatedAt)

	if err != nil {
		return domain.RetentionSweepRequest{}, fmt.Errorf("create retention sweep request: %w", err)
	}
	return req, nil
}

// ClaimPending atomically claims the oldest pending request by flipping its
// status to 'running' in the same statement that selects it (CTE +
// FOR UPDATE SKIP LOCKED). Returns nil if no pending request exists.
//
// Callers must not call MarkRunning afterwards — the row is already running.
func (r *RetentionSweepRequestRepo) ClaimPending(ctx context.Context) (*domain.RetentionSweepRequest, error) {
	var req domain.RetentionSweepRequest
	var summary json.RawMessage
	var errorMsg *string

	err := r.pool.QueryRow(ctx, `
		WITH next_req AS (
			SELECT id
			FROM retention_sweep_requests
			WHERE status = 'pending'
			ORDER BY requested_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE retention_sweep_requests r
		SET status = 'running', started_at = NOW(), updated_at = NOW()
		FROM next_req
		WHERE r.id = next_req.id
		RETURNING r.id, r.status, r.requested_at, r.started_at, r.finished_at, r.summary, r.error, r.created_at, r.updated_at
	`).Scan(&req.ID, &req.Status, &req.RequestedAt, &req.StartedAt, &req.FinishedAt, &summary, &errorMsg, &req.CreatedAt, &req.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("claim pending retention sweep request: %w", err)
	}

	if summary != nil {
		if err := json.Unmarshal(summary, &req.Summary); err != nil {
			return nil, fmt.Errorf("unmarshal summary: %w", err)
		}
	}
	req.Error = errorMsg

	return &req, nil
}

// MarkRunning sets status=running. Prefer ClaimPending for the worker path;
// kept for callers that already hold a request id (tests / recovery).
func (r *RetentionSweepRequestRepo) MarkRunning(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE retention_sweep_requests
		SET status = 'running', started_at = COALESCE(started_at, NOW()), updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark retention sweep request running: %w", err)
	}
	return nil
}

func (r *RetentionSweepRequestRepo) MarkCompleted(ctx context.Context, id string, summary map[string]interface{}) error {
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE retention_sweep_requests
		SET status = 'completed', finished_at = NOW(), summary = $2, updated_at = NOW()
		WHERE id = $1
	`, id, summaryJSON)
	if err != nil {
		return fmt.Errorf("mark retention sweep request completed: %w", err)
	}
	return nil
}

func (r *RetentionSweepRequestRepo) MarkFailed(ctx context.Context, id string, errorMsg string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE retention_sweep_requests
		SET status = 'failed', finished_at = NOW(), error = $2, updated_at = NOW()
		WHERE id = $1
	`, id, errorMsg)
	if err != nil {
		return fmt.Errorf("mark retention sweep request failed: %w", err)
	}
	return nil
}

// GetLatest returns the most recently created request (regardless of status).
func (r *RetentionSweepRequestRepo) GetLatest(ctx context.Context) (*domain.RetentionSweepRequest, error) {
	var req domain.RetentionSweepRequest
	var summary json.RawMessage
	var errorMsg *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, status, requested_at, started_at, finished_at, summary, error, created_at, updated_at
		FROM retention_sweep_requests
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&req.ID, &req.Status, &req.RequestedAt, &req.StartedAt, &req.FinishedAt, &summary, &errorMsg, &req.CreatedAt, &req.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest retention sweep request: %w", err)
	}

	if summary != nil {
		if err := json.Unmarshal(summary, &req.Summary); err != nil {
			return nil, fmt.Errorf("unmarshal summary: %w", err)
		}
	}
	req.Error = errorMsg

	return &req, nil
}
