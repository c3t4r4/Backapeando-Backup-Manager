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

var ErrNotFound = errors.New("repository: not found")

type AdminUserRepo struct {
	pool *pgxpool.Pool
}

func (r *AdminUserRepo) Create(ctx context.Context, email, cpf, passwordHash, role string) (domain.AdminUser, error) {
	var u domain.AdminUser
	err := r.pool.QueryRow(ctx, `
		INSERT INTO admin_users (email, cpf, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, cpf, password_hash, role, created_at, updated_at, last_login_at
	`, email, cpf, passwordHash, role).Scan(&u.ID, &u.Email, &u.CPF, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("create admin user: %w", err)
	}
	return u, nil
}

func (r *AdminUserRepo) GetByEmail(ctx context.Context, email string) (domain.AdminUser, error) {
	var u domain.AdminUser
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, cpf, password_hash, role, created_at, updated_at, last_login_at
		FROM admin_users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.CPF, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminUser{}, ErrNotFound
	}
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("get admin user by email: %w", err)
	}
	return u, nil
}

func (r *AdminUserRepo) GetByID(ctx context.Context, id string) (domain.AdminUser, error) {
	var u domain.AdminUser
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, cpf, password_hash, role, created_at, updated_at, last_login_at
		FROM admin_users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.CPF, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminUser{}, ErrNotFound
	}
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("get admin user by id: %w", err)
	}
	return u, nil
}

func (r *AdminUserRepo) TouchLastLogin(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin_users SET last_login_at = $2, updated_at = now() WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("touch last login: %w", err)
	}
	return nil
}

func (r *AdminUserRepo) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin_users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (r *AdminUserRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM admin_users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count admin users: %w", err)
	}
	return n, nil
}

func (r *AdminUserRepo) List(ctx context.Context) ([]domain.AdminUser, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, cpf, password_hash, role, created_at, updated_at, last_login_at
		FROM admin_users ORDER BY email
	`)
	if err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}
	defer rows.Close()

	var out []domain.AdminUser
	for rows.Next() {
		var u domain.AdminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.CPF, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt); err != nil {
			return nil, fmt.Errorf("scan admin user: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Update changes email/cpf only. Password changes go through the existing
// UpdatePassword (reused as-is, not duplicated here) — the handler calls
// both when a new password is supplied.
func (r *AdminUserRepo) Update(ctx context.Context, id, email, cpf string) (domain.AdminUser, error) {
	var u domain.AdminUser
	err := r.pool.QueryRow(ctx, `
		UPDATE admin_users SET email = $2, cpf = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, email, cpf, password_hash, role, created_at, updated_at, last_login_at
	`, id, email, cpf).Scan(&u.ID, &u.Email, &u.CPF, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminUser{}, ErrNotFound
	}
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("update admin user: %w", err)
	}
	return u, nil
}

func (r *AdminUserRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM admin_users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete admin user: %w", err)
	}
	return nil
}
