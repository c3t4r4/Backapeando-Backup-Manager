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
	DryRun       bool
	Affected     []string // names deleted (dryRun=false) or that would be deleted (dryRun=true)
	FailedDelete []string // names whose DeleteBlob failed (never includes audit-after-delete failures)
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

const (
	// ReasonPostBackupSweep is the audit reason for retention after a backup run.
	ReasonPostBackupSweep = "post-backup sweep"
	// ReasonGlobalRetentionSweep is the audit reason for the async global purge.
	ReasonGlobalRetentionSweep = "global retention sweep"
)

// SweepRetention lists a server's existing blobs under prefix, computes the
// GFS retention decision for policy, and deletes everything Decide() did
// not keep (unless dryRun), recording a retention_deletions audit row for
// each real deletion. If dryRun is true, nothing is deleted and no audit
// rows are written — the result only reports what would be deleted
// (RN-BACKUP-007).
//
// backupRunID may be empty for global sweeps that are not tied to a specific
// run; in that case the audit row is written with backup_run_id NULL (the
// column is nullable). Passing "" as a non-nil pointer would fail the UUID
// cast in Postgres after the blob was already deleted.
//
// reason is stored on each audit row; if empty, ReasonPostBackupSweep is used.
//
// If onAuditFailure is nil, an audit-write failure after a successful delete
// is silently ignored beyond being folded into the returned error — callers
// that want the "log it no matter what" guarantee must pass a non-nil
// onAuditFailure.
//
// FailedDelete lists only blobs where DeleteBlob itself failed — not audit
// write failures after a successful delete.
func SweepRetention(
	ctx context.Context,
	backend storage.Backend,
	prefix string,
	policy retention.Policy,
	serverID string,
	backupRunID string,
	dryRun bool,
	reason string,
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

	var runIDPtr *string
	if backupRunID != "" {
		runIDPtr = &backupRunID
	}
	if reason == "" {
		reason = ReasonPostBackupSweep
	}

	var failedDelete []string
	del := func(delCtx context.Context, blobName string) error {
		if err := backend.DeleteBlob(delCtx, blobName); err != nil {
			failedDelete = append(failedDelete, blobName)
			return err
		}
		// The blob is now irrevocably gone. If the audit row fails to
		// write, the blob name must not be lost — see
		// AuditWriteFailureAfterDeleteFunc and retention.Sweep's contract.
		if err := deletions.Create(delCtx, serverID, runIDPtr, blobName, reason); err != nil {
			if onAuditFailure != nil {
				return onAuditFailure(delCtx, serverID, backupRunID, blobName, err)
			}
			return fmt.Errorf("record retention deletion audit for blob %q (blob already deleted): %w", blobName, err)
		}
		return nil
	}

	affected, err := retention.Sweep(ctx, retentionBlobs, policy, time.Now(), dryRun, del)

	// Even if sweep has errors, return the result with successful deletions
	// already recorded in affected. Callers can inspect the error separately
	// to learn which blobs failed to delete.
	result := &SweepResult{DryRun: dryRun, Affected: affected, FailedDelete: failedDelete}
	if err != nil {
		return result, fmt.Errorf("sweep: %w", err)
	}

	return result, nil
}
