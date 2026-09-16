package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backapeando-backup-manager/internal/domain"
)

type RetentionPolicyRepo struct {
	pool *pgxpool.Pool
}

func scanRetentionPolicy(row pgx.Row) (domain.RetentionPolicy, error) {
	var p domain.RetentionPolicy
	err := row.Scan(&p.ID, &p.ServerID, &p.RecentCount, &p.MonthlyCount, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// GetGlobalDefault returns the single server_id IS NULL policy row that
// every server without its own override falls back to.
func (r *RetentionPolicyRepo) GetGlobalDefault(ctx context.Context) (domain.RetentionPolicy, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, server_id, recent_count, monthly_count, created_at, updated_at
		FROM retention_policies WHERE server_id IS NULL
	`)
	return scanRetentionPolicy(row)
}

func (r *RetentionPolicyRepo) UpdateGlobalDefault(ctx context.Context, recentCount, monthlyCount int) (domain.RetentionPolicy, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE retention_policies SET recent_count = $1, monthly_count = $2, updated_at = now()
		WHERE server_id IS NULL
		RETURNING id, server_id, recent_count, monthly_count, created_at, updated_at
	`, recentCount, monthlyCount)
	return scanRetentionPolicy(row)
}

// GetForServer returns the server-specific override if one exists, or
// ErrNotFound if the server relies on the global default.
func (r *RetentionPolicyRepo) GetForServer(ctx context.Context, serverID string) (domain.RetentionPolicy, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, server_id, recent_count, monthly_count, created_at, updated_at
		FROM retention_policies WHERE server_id = $1
	`, serverID)
	p, err := scanRetentionPolicy(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RetentionPolicy{}, ErrNotFound
	}
	if err != nil {
		return domain.RetentionPolicy{}, fmt.Errorf("get retention policy for server: %w", err)
	}
	return p, nil
}

// EffectiveForServer returns the server's override if present, else the
// global default — this is what the retention sweep (Phase 3) should call.
func (r *RetentionPolicyRepo) EffectiveForServer(ctx context.Context, serverID string) (domain.RetentionPolicy, error) {
	p, err := r.GetForServer(ctx, serverID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return domain.RetentionPolicy{}, err
	}
	return r.GetGlobalDefault(ctx)
}

func (r *RetentionPolicyRepo) UpsertForServer(ctx context.Context, serverID string, recentCount, monthlyCount int) (domain.RetentionPolicy, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO retention_policies (server_id, recent_count, monthly_count)
		VALUES ($1, $2, $3)
		ON CONFLICT (server_id) DO UPDATE
		SET recent_count = EXCLUDED.recent_count, monthly_count = EXCLUDED.monthly_count, updated_at = now()
		RETURNING id, server_id, recent_count, monthly_count, created_at, updated_at
	`, serverID, recentCount, monthlyCount)
	p, err := scanRetentionPolicy(row)
	if err != nil {
		return domain.RetentionPolicy{}, fmt.Errorf("upsert retention policy for server: %w", err)
	}
	return p, nil
}

func (r *RetentionPolicyRepo) DeleteForServer(ctx context.Context, serverID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM retention_policies WHERE server_id = $1`, serverID)
	if err != nil {
		return fmt.Errorf("delete retention policy override: %w", err)
	}
	return nil
}
