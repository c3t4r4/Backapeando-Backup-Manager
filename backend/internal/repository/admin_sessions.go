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

type AdminSessionRepo struct {
	pool *pgxpool.Pool
}

func (r *AdminSessionRepo) Create(ctx context.Context, userID string, expiresAt time.Time, userAgent, ip string) (domain.AdminSession, error) {
	var s domain.AdminSession
	err := r.pool.QueryRow(ctx, `
		INSERT INTO admin_sessions (user_id, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, created_at, expires_at, user_agent, ip
	`, userID, expiresAt, userAgent, ip).Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.ExpiresAt, &s.UserAgent, &s.IP)
	if err != nil {
		return domain.AdminSession{}, fmt.Errorf("create session: %w", err)
	}
	return s, nil
}

// GetValid returns the session only if it exists and has not expired.
func (r *AdminSessionRepo) GetValid(ctx context.Context, id string) (domain.AdminSession, error) {
	var s domain.AdminSession
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, created_at, expires_at, user_agent, ip
		FROM admin_sessions WHERE id = $1 AND expires_at > now()
	`, id).Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.ExpiresAt, &s.UserAgent, &s.IP)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminSession{}, ErrNotFound
	}
	if err != nil {
		return domain.AdminSession{}, fmt.Errorf("get session: %w", err)
	}
	return s, nil
}

func (r *AdminSessionRepo) Extend(ctx context.Context, id string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin_sessions SET expires_at = $2 WHERE id = $1`, id, expiresAt)
	if err != nil {
		return fmt.Errorf("extend session: %w", err)
	}
	return nil
}

func (r *AdminSessionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *AdminSessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
