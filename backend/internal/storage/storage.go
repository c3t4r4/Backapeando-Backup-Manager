// Package storage defines the pluggable backend abstraction backups are
// uploaded to, listed in, and deleted from. A Backend is always bound to an
// already-configured target (account/bucket/directory + credentials) at
// construction time, so call sites never thread a raw target struct through
// every method call — they just call UploadStream/ListBlobs/CheckAccess/
// DeleteBlob against the Backend value returned by NewBackend (dispatch.go).
//
// Three concrete implementations exist:
//   - internal/azureblob — Azure Blob Storage (SAS token), the original and
//     only backend before this package existed.
//   - internal/s3storage — any S3-compatible object store (AWS S3, MinIO,
//     Backblaze, Wasabi, Cloudflare R2, ...) via a configurable endpoint.
//   - internal/fsstorage — a local filesystem directory. This is also what
//     backs "NFS storage": an NFS export mounted as a directory looks like
//     any other folder to this process, so no network NFS protocol client is
//     implemented or needed — mounting is an infra/ops concern outside Go.
package storage

import (
	"context"
	"io"
	"time"
)

// BlobInfo describes a single stored backup object as needed by the
// retention algorithm (internal/retention) and the backup-runs API. The name
// mirrors azureblob.BlobInfo, which predates this package.
type BlobInfo struct {
	Name         string
	LastModified time.Time
	SizeBytes    int64
}

// Backend is the storage-agnostic contract every backup destination
// implements. All methods are bound to whatever target the Backend was
// constructed for (see NewBackend in dispatch.go) — none of them take a
// target/credentials argument themselves.
type Backend interface {
	// UploadStream uploads r as a blob/object/file named blobName, streaming
	// without buffering the whole content in memory. Returns the number of
	// bytes uploaded.
	UploadStream(ctx context.Context, blobName string, r io.Reader) (int64, error)

	// ListBlobs lists all blobs/objects/files whose name starts with prefix.
	ListBlobs(ctx context.Context, prefix string) ([]BlobInfo, error)

	// CheckAccess validates that the target is reachable and usable
	// (credentials/permissions/existence) without uploading or deleting
	// anything.
	CheckAccess(ctx context.Context) error

	// DeleteBlob permanently deletes a single blob/object/file by name.
	// Irreversible — callers performing a retention sweep must have already
	// recorded (or be about to record) an audit row before/alongside calling
	// this (see internal/backupcore).
	DeleteBlob(ctx context.Context, blobName string) error
}
