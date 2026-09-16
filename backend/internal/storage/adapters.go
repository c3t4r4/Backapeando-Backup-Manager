package storage

import (
	"context"
	"io"

	"backapeando-backup-manager/internal/azureblob"
	"backapeando-backup-manager/internal/fsstorage"
	"backapeando-backup-manager/internal/s3storage"
)

// This file holds the thin adapters that make each concrete backend package
// satisfy the Backend interface above. They live here — in package storage —
// rather than inside azureblob/fsstorage/s3storage themselves, because this
// package's dispatch.go needs to import all three concrete packages to
// construct them; if any of those packages in turn imported this package
// (to reference Backend/BlobInfo in their own adapter's method signatures),
// the two packages would import each other, which Go disallows. Each
// concrete package instead defines its own equivalent BlobInfo type and
// stays entirely unaware that internal/storage exists; the small conversion
// between the two shapes happens once, right here.

// azureAdapter satisfies Backend for an Azure Blob Storage target, without
// requiring any change to the tested azureblob package.
type azureAdapter struct {
	target azureblob.Target
}

func (a *azureAdapter) UploadStream(ctx context.Context, blobName string, r io.Reader) (int64, error) {
	return azureblob.UploadStream(ctx, a.target, blobName, r)
}

func (a *azureAdapter) ListBlobs(ctx context.Context, prefix string) ([]BlobInfo, error) {
	blobs, err := azureblob.ListBlobs(ctx, a.target, prefix)
	if err != nil {
		return nil, err
	}
	return convertBlobInfo(blobs), nil
}

func (a *azureAdapter) CheckAccess(ctx context.Context) error {
	return azureblob.CheckAccess(ctx, a.target)
}

func (a *azureAdapter) DeleteBlob(ctx context.Context, blobName string) error {
	return azureblob.DeleteBlob(ctx, a.target, blobName)
}

// convertBlobInfo adapts any []T with the same (Name, LastModified,
// SizeBytes) shape as BlobInfo. Go generics can't express "structurally
// identical struct" directly, so each concrete package's own BlobInfo slice
// gets its own tiny conversion helper below instead of one generic function.
func convertBlobInfo(blobs []azureblob.BlobInfo) []BlobInfo {
	out := make([]BlobInfo, len(blobs))
	for i, b := range blobs {
		out[i] = BlobInfo{Name: b.Name, LastModified: b.LastModified, SizeBytes: b.SizeBytes}
	}
	return out
}

// s3Adapter satisfies Backend by delegating to an *s3storage.Backend,
// converting s3storage.BlobInfo to BlobInfo.
type s3Adapter struct {
	backend *s3storage.Backend
}

func (a *s3Adapter) UploadStream(ctx context.Context, blobName string, r io.Reader) (int64, error) {
	return a.backend.UploadStream(ctx, blobName, r)
}

func (a *s3Adapter) ListBlobs(ctx context.Context, prefix string) ([]BlobInfo, error) {
	blobs, err := a.backend.ListBlobs(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]BlobInfo, len(blobs))
	for i, b := range blobs {
		out[i] = BlobInfo{Name: b.Name, LastModified: b.LastModified, SizeBytes: b.SizeBytes}
	}
	return out, nil
}

func (a *s3Adapter) CheckAccess(ctx context.Context) error {
	return a.backend.CheckAccess(ctx)
}

func (a *s3Adapter) DeleteBlob(ctx context.Context, blobName string) error {
	return a.backend.DeleteBlob(ctx, blobName)
}

// fsAdapter satisfies Backend by delegating to an *fsstorage.Backend,
// converting fsstorage.BlobInfo to BlobInfo.
type fsAdapter struct {
	backend *fsstorage.Backend
}

func (a *fsAdapter) UploadStream(ctx context.Context, blobName string, r io.Reader) (int64, error) {
	return a.backend.UploadStream(ctx, blobName, r)
}

func (a *fsAdapter) ListBlobs(ctx context.Context, prefix string) ([]BlobInfo, error) {
	blobs, err := a.backend.ListBlobs(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]BlobInfo, len(blobs))
	for i, b := range blobs {
		out[i] = BlobInfo{Name: b.Name, LastModified: b.LastModified, SizeBytes: b.SizeBytes}
	}
	return out, nil
}

func (a *fsAdapter) CheckAccess(ctx context.Context) error {
	return a.backend.CheckAccess(ctx)
}

func (a *fsAdapter) DeleteBlob(ctx context.Context, blobName string) error {
	return a.backend.DeleteBlob(ctx, blobName)
}
