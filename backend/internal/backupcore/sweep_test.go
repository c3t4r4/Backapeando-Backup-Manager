package backupcore

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"backapeando-backup-manager/internal/retention"
	"backapeando-backup-manager/internal/storage"
)

type fakeBackend struct {
	blobs   []storage.BlobInfo
	deleted []string
	failOn  map[string]error
}

func (f *fakeBackend) UploadStream(context.Context, string, io.Reader) (int64, error) {
	return 0, nil
}
func (f *fakeBackend) ListBlobs(context.Context, string) ([]storage.BlobInfo, error) {
	return f.blobs, nil
}
func (f *fakeBackend) CheckAccess(context.Context) error { return nil }
func (f *fakeBackend) DeleteBlob(_ context.Context, blobName string) error {
	if f.failOn != nil {
		if err, ok := f.failOn[blobName]; ok {
			return err
		}
	}
	f.deleted = append(f.deleted, blobName)
	return nil
}
func (f *fakeBackend) ProbeDelete(context.Context) error { return nil }

type recordingDeletions struct {
	calls []struct {
		serverID    string
		backupRunID *string
		blobName    string
		reason      string
	}
}

func (r *recordingDeletions) Create(_ context.Context, serverID string, backupRunID *string, blobName string, reason string) error {
	r.calls = append(r.calls, struct {
		serverID    string
		backupRunID *string
		blobName    string
		reason      string
	}{serverID, backupRunID, blobName, reason})
	return nil
}

func TestSweepRetention_EmptyBackupRunIDPassesNilToAudit(t *testing.T) {
	now := time.Now().UTC()
	backend := &fakeBackend{
		blobs: []storage.BlobInfo{
			{Name: "srv/a.dump", LastModified: now},
			{Name: "srv/b.dump", LastModified: now.Add(-time.Hour)},
			{Name: "srv/c.dump", LastModified: now.Add(-2 * time.Hour)},
			{Name: "srv/d.dump", LastModified: now.Add(-3 * time.Hour)},
		},
	}
	deletions := &recordingDeletions{}

	result, err := SweepRetention(
		context.Background(),
		backend,
		"srv/",
		retention.Policy{RecentCount: 3, MonthlyCount: 0},
		"server-1",
		"", // global sweep — must not pass &""
		false,
		ReasonGlobalRetentionSweep,
		deletions,
		nil,
	)
	if err != nil {
		t.Fatalf("SweepRetention: %v", err)
	}
	if result == nil || len(result.Affected) != 1 {
		t.Fatalf("expected 1 deletion, got %#v", result)
	}
	if len(deletions.calls) != 1 {
		t.Fatalf("expected 1 audit call, got %d", len(deletions.calls))
	}
	call := deletions.calls[0]
	if call.backupRunID != nil {
		t.Fatalf("expected nil backupRunID for global sweep, got %q", *call.backupRunID)
	}
	if call.reason != ReasonGlobalRetentionSweep {
		t.Fatalf("reason = %q, want %q", call.reason, ReasonGlobalRetentionSweep)
	}
}

func TestSweepRetention_WithBackupRunIDPassesPointer(t *testing.T) {
	now := time.Now().UTC()
	backend := &fakeBackend{
		blobs: []storage.BlobInfo{
			{Name: "srv/a.dump", LastModified: now},
			{Name: "srv/b.dump", LastModified: now.Add(-time.Hour)},
			{Name: "srv/c.dump", LastModified: now.Add(-2 * time.Hour)},
			{Name: "srv/d.dump", LastModified: now.Add(-3 * time.Hour)},
		},
	}
	deletions := &recordingDeletions{}
	runID := "11111111-1111-1111-1111-111111111111"

	_, err := SweepRetention(
		context.Background(),
		backend,
		"srv/",
		retention.Policy{RecentCount: 3, MonthlyCount: 0},
		"server-1",
		runID,
		false,
		ReasonPostBackupSweep,
		deletions,
		nil,
	)
	if err != nil {
		t.Fatalf("SweepRetention: %v", err)
	}
	if len(deletions.calls) != 1 {
		t.Fatalf("expected 1 audit call, got %d", len(deletions.calls))
	}
	call := deletions.calls[0]
	if call.backupRunID == nil || *call.backupRunID != runID {
		t.Fatalf("backupRunID = %#v, want %q", call.backupRunID, runID)
	}
	if call.reason != ReasonPostBackupSweep {
		t.Fatalf("reason = %q, want %q", call.reason, ReasonPostBackupSweep)
	}
}

func TestSweepRetention_FailedDeleteTracksDeleteBlobErrors(t *testing.T) {
	now := time.Now().UTC()
	failName := "srv/d.dump"
	backend := &fakeBackend{
		blobs: []storage.BlobInfo{
			{Name: "srv/a.dump", LastModified: now},
			{Name: "srv/b.dump", LastModified: now.Add(-time.Hour)},
			{Name: "srv/c.dump", LastModified: now.Add(-2 * time.Hour)},
			{Name: failName, LastModified: now.Add(-3 * time.Hour)},
		},
		failOn: map[string]error{failName: errors.New("permission denied")},
	}
	deletions := &recordingDeletions{}

	result, err := SweepRetention(
		context.Background(),
		backend,
		"srv/",
		retention.Policy{RecentCount: 2, MonthlyCount: 0},
		"server-1",
		"11111111-1111-1111-1111-111111111111",
		false,
		ReasonPostBackupSweep,
		deletions,
		nil,
	)
	if err == nil {
		t.Fatal("expected error from partial delete failure")
	}
	if result == nil {
		t.Fatal("expected non-nil result with partial success")
	}
	if len(result.FailedDelete) != 1 || result.FailedDelete[0] != failName {
		t.Fatalf("FailedDelete = %#v, want [%q]", result.FailedDelete, failName)
	}
	// RecentCount=2 keeps a,b; c and d are candidates — c succeeds, d fails.
	if len(result.Affected) != 1 || result.Affected[0] != "srv/c.dump" {
		t.Fatalf("Affected = %#v, want [srv/c.dump]", result.Affected)
	}
}
