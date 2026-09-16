// Package repository contains hand-written SQL access for every aggregate.
// No ORM is used deliberately: these tables hold encrypted secrets and
// security-sensitive state, so every query should be easy to audit by
// reading it directly.
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct {
	AdminUsers         *AdminUserRepo
	AdminSessions      *AdminSessionRepo
	StorageTargets     *StorageTargetRepo
	Servers            *ServerRepo
	RetentionPolicies  *RetentionPolicyRepo
	BackupRuns         *BackupRunRepo
	RetentionDeletions *RetentionDeletionRepo
}

func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		AdminUsers:         &AdminUserRepo{pool: pool},
		AdminSessions:      &AdminSessionRepo{pool: pool},
		StorageTargets:     &StorageTargetRepo{pool: pool},
		Servers:            &ServerRepo{pool: pool},
		RetentionPolicies:  &RetentionPolicyRepo{pool: pool},
		BackupRuns:         &BackupRunRepo{pool: pool},
		RetentionDeletions: &RetentionDeletionRepo{pool: pool},
	}
}

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}
