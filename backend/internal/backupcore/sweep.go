// Package backupcore holds backup-pipeline logic shared between the two
// places a backup is executed: the synchronous /backup-now HTTP handler
// (httpapi/handlers/backup.go) and the asynchronous scheduler/worker
// (scheduler/executor.go). Before this package existed, both call sites
// duplicated the same "list existing blobs, apply the GFS retention policy,
// delete what's no longer kept, write an audit row per deletion" sequence —
// documented as accepted tech debt in docs/Arquitetura.md. SweepRetention
// resolves that duplication.
package backupcore

import (
	"context"
	"fmt"
	"time"

	"backapeando-backup-manager/internal/retention"
	"backapeando-backup-manager/internal/storage"
)

// RetentionDeletionRecorder is the minimal surface SweepRetention needs from
// repository.RetentionDeletionRepo, kept as an interface so this package
// doesn't import the repository package (which would be a needless coupling
// for a pure orchestration helper) and so it stays trivially testable.
type RetentionDeletionRecorder interface {
	Create(ctx context.Context, serverID string, backupRunID *string, blobName string, reason string) error
}

// SweepResult reports what SweepRetention did, in the same shape both call
// sites expose over their respective APIs (BackupHandlers.sweepRetention's
// retentionResultDTO and the worker's log line).
type SweepResult struct {
	DryRun   bool
	Affected []string // names deleted (dryRun=false) or that would be deleted (dryRun=true)
}

// AuditWriteFailureAfterDeleteFunc is called by SweepRetention when a blob
// was successfully deleted but recording the audit row failed, and must
// return a non-nil error that still mentions blobName (wrapping auditErr) —
// the blob is irrevocably gone by this point, so the only remaining
// question is whether its name survives somewhere. Both existing call sites
// had their own version of this guarantee before backupcore existed;
// callers pass their own implementation (typically wrapping a *slog.Logger)
// so the log line keeps each call site's existing fields/shape.
type AuditWriteFailureAfterDeleteFunc func(ctx context.Context, serverID, backupRunID, blobName string, auditErr error) error

// SweepRetention lists a server's existing blobs under prefix, computes the
// GFS retention decision for policy, and deletes everything Decide() did
// not keep (unless dryRun), recording a retention_deletions audit row for
// each real deletion. If dryRun is true, nothing is deleted and no audit
// rows are written — the result only reports what would be deleted
// (RN-BACKUP-007).
//
// If onAuditFailure is nil, an audit-write failure after a successful delete
// is silently ignored beyond being folded into the returned error — callers
// that want the "log it no matter what" guarantee must pass a non-nil
// onAuditFailure.
func SweepRetention(
	ctx context.Context,
	backend storage.Backend,
	prefix string,
	policy retention.Policy,
	serverID string,
	backupRunID string,
	dryRun bool,
	deletions RetentionDeletionRecorder,
	onAuditFailure AuditWriteFailureAfterDeleteFunc,
) (*SweepResult, error) {
	blobs, err := backend.ListBlobs(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list blobs: %w", err)
	}

	retentionBlobs := make([]retention.BlobInfo, len(blobs))
	for i, b := range blobs {
		retentionBlobs[i] = retention.BlobInfo{Name: b.Name, LastModified: b.LastModified}
	}

	runID := backupRunID
	del := func(delCtx context.Context, blobName string) error {
		if err := backend.DeleteBlob(delCtx, blobName); err != nil {
			return err
		}
		// The blob is now irrevocably gone. If the audit row fails to
		// write, the blob name must not be lost — see
		// AuditWriteFailureAfterDeleteFunc and retention.Sweep's contract.
		if err := deletions.Create(delCtx, serverID, &runID, blobName, "post-backup sweep"); err != nil {
			if onAuditFailure != nil {
				return onAuditFailure(delCtx, serverID, runID, blobName, err)
			}
			return fmt.Errorf("record retention deletion audit for blob %q (blob already deleted): %w", blobName, err)
		}
		return nil
	}

	affected, err := retention.Sweep(ctx, retentionBlobs, policy, time.Now(), dryRun, del)
	if err != nil {
		return nil, fmt.Errorf("sweep: %w", err)
	}

	return &SweepResult{DryRun: dryRun, Affected: affected}, nil
}
