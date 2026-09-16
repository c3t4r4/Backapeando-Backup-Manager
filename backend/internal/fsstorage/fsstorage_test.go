package fsstorage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadListDeleteRoundTrip(t *testing.T) {
	root := t.TempDir()
	backend := NewBackend(Target{RootPath: root})
	ctx := context.Background()

	content := []byte("fake pg_dump bytes")
	written, err := backend.UploadStream(ctx, "my-server/mydb_20260101T000000Z.dump", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("UploadStream: %v", err)
	}
	if written != int64(len(content)) {
		t.Errorf("UploadStream returned %d bytes, want %d", written, len(content))
	}

	// The file should actually exist on disk under root, in the expected
	// subdirectory.
	onDisk, err := os.ReadFile(filepath.Join(root, "my-server", "mydb_20260101T000000Z.dump"))
	if err != nil {
		t.Fatalf("read uploaded file directly: %v", err)
	}
	if !bytes.Equal(onDisk, content) {
		t.Errorf("on-disk content = %q, want %q", onDisk, content)
	}

	blobs, err := backend.ListBlobs(ctx, "my-server/")
	if err != nil {
		t.Fatalf("ListBlobs: %v", err)
	}
	if len(blobs) != 1 {
		t.Fatalf("expected 1 blob, got %d: %+v", len(blobs), blobs)
	}
	if blobs[0].Name != "my-server/mydb_20260101T000000Z.dump" {
		t.Errorf("blob name = %q, want %q", blobs[0].Name, "my-server/mydb_20260101T000000Z.dump")
	}
	if blobs[0].SizeBytes != int64(len(content)) {
		t.Errorf("blob size = %d, want %d", blobs[0].SizeBytes, len(content))
	}

	// A prefix that doesn't match anything returns an empty (not error) list.
	none, err := backend.ListBlobs(ctx, "other-server/")
	if err != nil {
		t.Fatalf("ListBlobs with non-matching prefix: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 blobs for non-matching prefix, got %d", len(none))
	}

	if err := backend.DeleteBlob(ctx, "my-server/mydb_20260101T000000Z.dump"); err != nil {
		t.Fatalf("DeleteBlob: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "my-server", "mydb_20260101T000000Z.dump")); !os.IsNotExist(err) {
		t.Errorf("expected file to be gone after DeleteBlob, stat err = %v", err)
	}

	blobsAfterDelete, err := backend.ListBlobs(ctx, "my-server/")
	if err != nil {
		t.Fatalf("ListBlobs after delete: %v", err)
	}
	if len(blobsAfterDelete) != 0 {
		t.Errorf("expected 0 blobs after delete, got %d", len(blobsAfterDelete))
	}
}

func TestCheckAccess(t *testing.T) {
	root := t.TempDir()
	backend := NewBackend(Target{RootPath: root})

	if err := backend.CheckAccess(context.Background()); err != nil {
		t.Fatalf("CheckAccess on a fresh writable directory: %v", err)
	}

	// Directory doesn't exist yet: CheckAccess must create it (mirrors the
	// operator experience of pointing a new target at a not-yet-created
	// path) rather than failing.
	missing := filepath.Join(root, "does", "not", "exist", "yet")
	backend2 := NewBackend(Target{RootPath: missing})
	if err := backend2.CheckAccess(context.Background()); err != nil {
		t.Fatalf("CheckAccess should create a missing root path: %v", err)
	}
	if info, err := os.Stat(missing); err != nil || !info.IsDir() {
		t.Errorf("expected root path to have been created as a directory")
	}
}

// TestPathTraversalRejected is the security-critical test: blobName is
// operator/application-controlled (server slug + timestamped dump filename),
// never raw end-user input, but resolve() must still reject any name that
// would escape RootPath — via "..", an absolute path, or any other trick —
// as a hard boundary against a future bug or caller.
func TestPathTraversalRejected(t *testing.T) {
	root := t.TempDir()
	backend := NewBackend(Target{RootPath: root})
	ctx := context.Background()

	dangerous := []string{
		"../escape.txt",
		"../../etc/passwd",
		"a/../../escape.txt",
		"a/../../../escape.txt",
	}
	if !filepath.IsAbs("/etc/passwd") {
		t.Fatal("test assumption broken: /etc/passwd is not absolute on this platform")
	}
	dangerous = append(dangerous, "/etc/passwd")

	for _, name := range dangerous {
		t.Run(name, func(t *testing.T) {
			if _, err := backend.UploadStream(ctx, name, strings.NewReader("x")); err == nil {
				t.Errorf("UploadStream(%q) succeeded, want path-traversal rejection", name)
			}
			if err := backend.DeleteBlob(ctx, name); err == nil {
				t.Errorf("DeleteBlob(%q) succeeded, want path-traversal rejection", name)
			}
		})
	}

	// Confirm nothing escaped: no file was created outside root.
	parent := filepath.Dir(root)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("read parent dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == "escape.txt" {
			t.Fatalf("path traversal succeeded: found escape.txt in %s", parent)
		}
	}
}

func TestUploadStream_EmptyBlobNameRejected(t *testing.T) {
	root := t.TempDir()
	backend := NewBackend(Target{RootPath: root})
	if _, err := backend.UploadStream(context.Background(), "", strings.NewReader("x")); err == nil {
		t.Error("expected error for empty blobName, got nil")
	}
}

func TestUploadStream_CreatesParentDirectories(t *testing.T) {
	root := t.TempDir()
	backend := NewBackend(Target{RootPath: root})

	_, err := backend.UploadStream(context.Background(), "nested/deeper/still/file.dump", strings.NewReader("content"))
	if err != nil {
		t.Fatalf("UploadStream with nested path: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "nested", "deeper", "still", "file.dump"))
	if err != nil {
		t.Fatalf("read nested file: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("content = %q, want %q", data, "content")
	}
}

func TestListBlobs_NonExistentRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	backend := NewBackend(Target{RootPath: root})

	blobs, err := backend.ListBlobs(context.Background(), "")
	if err != nil {
		t.Fatalf("ListBlobs on a non-existent root should not error, got: %v", err)
	}
	if len(blobs) != 0 {
		t.Errorf("expected 0 blobs, got %d", len(blobs))
	}
}

var _ io.Reader = (*bytes.Reader)(nil) // sanity: bytes.Reader satisfies io.Reader used above
