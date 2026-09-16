package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backapeando-backup-manager/internal/domain"
)

// StorageTargetRepo persists domain.StorageTarget rows (Azure Blob Storage,
// S3-compatible, or local/NFS filesystem targets — discriminated by the
// `type` column) against the storage_targets table.
type StorageTargetRepo struct {
	pool *pgxpool.Pool
}

const storageTargetColumns = `
	id, name, type,
	azure_account_name, azure_container_name, azure_sas_token_encrypted, azure_sas_token_expires_at,
	s3_endpoint, s3_region, s3_bucket, s3_access_key_id, s3_secret_access_key_encrypted, s3_use_path_style,
	fs_root_path,
	created_at, updated_at
`

func scanStorageTarget(row pgx.Row) (domain.StorageTarget, error) {
	var t domain.StorageTarget
	err := row.Scan(
		&t.ID, &t.Name, &t.Type,
		&t.AzureAccountName, &t.AzureContainerName, &t.AzureSASTokenEncrypted, &t.AzureSASTokenExpiresAt,
		&t.S3Endpoint, &t.S3Region, &t.S3Bucket, &t.S3AccessKeyID, &t.S3SecretAccessKeyEncrypted, &t.S3UsePathStyle,
		&t.FSRootPath,
		&t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

// Create requires t.ID to already be set by the caller (a fresh
// uuid.NewString()) when the target holds a secret (Azure/S3) — the
// encrypted secret's AEAD associated data is bound to this exact ID (see
// internal/crypto and internal/storage's AAD constants), so the row must be
// inserted with that same ID rather than letting the database generate its
// own default, or every future Decrypt would fail.
func (r *StorageTargetRepo) Create(ctx context.Context, t domain.StorageTarget) (domain.StorageTarget, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO storage_targets (
			id, name, type,
			azure_account_name, azure_container_name, azure_sas_token_encrypted, azure_sas_token_expires_at,
			s3_endpoint, s3_region, s3_bucket, s3_access_key_id, s3_secret_access_key_encrypted, s3_use_path_style,
			fs_root_path
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING `+storageTargetColumns,
		t.ID, t.Name, t.Type,
		t.AzureAccountName, t.AzureContainerName, t.AzureSASTokenEncrypted, t.AzureSASTokenExpiresAt,
		t.S3Endpoint, t.S3Region, t.S3Bucket, t.S3AccessKeyID, t.S3SecretAccessKeyEncrypted, t.S3UsePathStyle,
		t.FSRootPath,
	)
	created, err := scanStorageTarget(row)
	if err != nil {
		return domain.StorageTarget{}, fmt.Errorf("create storage target: %w", err)
	}
	return created, nil
}

func (r *StorageTargetRepo) List(ctx context.Context) ([]domain.StorageTarget, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+storageTargetColumns+` FROM storage_targets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list storage targets: %w", err)
	}
	defer rows.Close()

	var out []domain.StorageTarget
	for rows.Next() {
		t, err := scanStorageTarget(rows)
		if err != nil {
			return nil, fmt.Errorf("scan storage target: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *StorageTargetRepo) Get(ctx context.Context, id string) (domain.StorageTarget, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+storageTargetColumns+` FROM storage_targets WHERE id = $1`, id)
	t, err := scanStorageTarget(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StorageTarget{}, ErrNotFound
	}
	if err != nil {
		return domain.StorageTarget{}, fmt.Errorf("get storage target: %w", err)
	}
	return t, nil
}

func (r *StorageTargetRepo) Update(ctx context.Context, t domain.StorageTarget) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE storage_targets
		SET name = $2, type = $3,
		    azure_account_name = $4, azure_container_name = $5, azure_sas_token_encrypted = $6, azure_sas_token_expires_at = $7,
		    s3_endpoint = $8, s3_region = $9, s3_bucket = $10, s3_access_key_id = $11, s3_secret_access_key_encrypted = $12, s3_use_path_style = $13,
		    fs_root_path = $14,
		    updated_at = now()
		WHERE id = $1
	`, t.ID, t.Name, t.Type,
		t.AzureAccountName, t.AzureContainerName, t.AzureSASTokenEncrypted, t.AzureSASTokenExpiresAt,
		t.S3Endpoint, t.S3Region, t.S3Bucket, t.S3AccessKeyID, t.S3SecretAccessKeyEncrypted, t.S3UsePathStyle,
		t.FSRootPath,
	)
	if err != nil {
		return fmt.Errorf("update storage target: %w", err)
	}
	return nil
}

func (r *StorageTargetRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM storage_targets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete storage target: %w", err)
	}
	return nil
}
