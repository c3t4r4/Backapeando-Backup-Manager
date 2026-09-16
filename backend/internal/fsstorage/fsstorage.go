// Package fsstorage implements a storage backend over a plain local
// filesystem directory (adapted to storage.Backend by internal/storage's
// dispatch — see that package's adapters.go). This backend covers two use
// cases that are
// identical from this process's point of view:
//
//   - a local folder on the same host running the API/worker process;
//   - an NFS export mounted as a directory by the operating system/ops
//     tooling before this process starts.
//
// There is no network NFS protocol client here, and none is planned: once an
// NFS share is mounted, it is just a directory, and this package only ever
// deals in directories. Mounting (and its availability/retry/permission
// concerns) is entirely an infrastructure/ops responsibility, outside Go.
package fsstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BlobInfo describes a single stored file, mirroring azureblob.BlobInfo and
// storage.BlobInfo in shape. This package defines its own copy (rather than
// importing internal/storage) so that internal/storage can import this
// package to build its dispatch.NewBackend without creating an import
// cycle; internal/storage/adapters.go converts between the two shapes.
type BlobInfo struct {
	Name         string
	LastModified time.Time
	SizeBytes    int64
}

// Target configures a filesystem-backed Backend.
type Target struct {
	// RootPath is the directory backups are written under. It must already
	// exist (or be creatable) and be writable by the running process.
	RootPath string
}

// Backend implements upload/list/check/delete against a local directory (or
// an NFS export mounted as one — see the package doc comment).
type Backend struct {
	root string
}

// NewBackend returns a Backend that reads/writes under t.RootPath.
func NewBackend(t Target) *Backend {
	return &Backend{root: filepath.Clean(t.RootPath)}
}

// resolve joins the backend's root with blobName and verifies the result
// stays inside root. blobName is operator/application-controlled (server
// slug + timestamped dump filename), never raw end-user input, but this
// check is kept as a hard boundary regardless — a single ".." segment or an
// absolute path must never be able to escape RootPath, whether from a future
// caller, a bug elsewhere, or a crafted blob name.
//
// This deliberately REJECTS any blobName whose cleaned form still starts
// with ".." (rather than silently confining it to root, e.g. by cleaning an
// artificially-rooted "/"+blobName — that would make "../../secret" quietly
// resolve to root/secret instead of failing loudly, which is a worse
// outcome for a bug that should be visible immediately).
func (b *Backend) resolve(blobName string) (string, error) {
	if blobName == "" {
		return "", errors.New("fsstorage: blobName must not be empty")
	}
	if filepath.IsAbs(blobName) {
		return "", fmt.Errorf("fsstorage: blobName %q must not be an absolute path", blobName)
	}

	cleaned := filepath.Clean(blobName)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fsstorage: blobName %q escapes root path", blobName)
	}

	return filepath.Join(b.root, cleaned), nil
}

// UploadStream writes r to RootPath/blobName, creating any parent
// directories implied by blobName (e.g. "server-slug/db_2026...dump").
func (b *Backend) UploadStream(ctx context.Context, blobName string, r io.Reader) (int64, error) {
	path, err := b.resolve(blobName)
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, fmt.Errorf("fsstorage: create parent directories: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, fmt.Errorf("fsstorage: create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, r)
	if err != nil {
		return written, fmt.Errorf("fsstorage: write file: %w", err)
	}
	if err := f.Sync(); err != nil {
		return written, fmt.Errorf("fsstorage: sync file: %w", err)
	}
	return written, nil
}

// ListBlobs walks RootPath recursively and returns every regular file whose
// slash-separated relative path (relative to RootPath) starts with prefix.
func (b *Backend) ListBlobs(ctx context.Context, prefix string) ([]BlobInfo, error) {
	var out []BlobInfo

	err := filepath.WalkDir(b.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(b.root, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		if prefix != "" && !strings.HasPrefix(relSlash, prefix) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		out = append(out, BlobInfo{
			Name:         relSlash,
			LastModified: info.ModTime(),
			SizeBytes:    info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("fsstorage: walk root: %w", err)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// CheckAccess verifies RootPath exists (creating it if missing) and is
// writable, by writing and removing a small marker file.
func (b *Backend) CheckAccess(ctx context.Context) error {
	if err := os.MkdirAll(b.root, 0o700); err != nil {
		return fmt.Errorf("fsstorage: root path not usable: %w", err)
	}

	marker := filepath.Join(b.root, fmt.Sprintf(".fsstorage-check-%d", time.Now().UnixNano()))
	if err := os.WriteFile(marker, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("fsstorage: root path not writable: %w", err)
	}
	if err := os.Remove(marker); err != nil {
		return fmt.Errorf("fsstorage: failed to clean up access-check marker file: %w", err)
	}
	return nil
}

// DeleteBlob removes RootPath/blobName.
func (b *Backend) DeleteBlob(ctx context.Context, blobName string) error {
	path, err := b.resolve(blobName)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("fsstorage: delete file: %w", err)
	}
	return nil
}
