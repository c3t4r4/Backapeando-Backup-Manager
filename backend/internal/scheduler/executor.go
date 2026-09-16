package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"backapeando-backup-manager/internal/backupcore"
	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
	"backapeando-backup-manager/internal/retention"
	"backapeando-backup-manager/internal/sshclient"
	"backapeando-backup-manager/internal/storage"
)

// BackupExecutor executes a backup run: decrypt keys, SSH dial, pg_dump stream,
// storage upload, and retention sweep.
type BackupExecutor struct {
	db     *pgxpool.Pool
	repos  *repository.Repositories
	sealer *crypto.Sealer
	logger *slog.Logger
}

// NewBackupExecutor creates a new executor.
func NewBackupExecutor(db *pgxpool.Pool, repos *repository.Repositories, sealer *crypto.Sealer, logger *slog.Logger) *BackupExecutor {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackupExecutor{
		db:     db,
		repos:  repos,
		sealer: sealer,
		logger: logger,
	}
}

// ExecuteBackup runs the complete backup pipeline: decrypt, SSH, pg_dump stream,
// storage upload, retenção sweep. It updates backup_runs status at each stage.
//
// Semantics:
//   - If any error before upload completes, backup is marked "failed"
//   - If upload completes but retenção read fails, backup is marked "success" anyway
//   - If blob is uploaded but cleanup fails, the error is logged and propagated,
//     but backup is still marked "failed" (not left in running state)
func (e *BackupExecutor) ExecuteBackup(ctx context.Context, backupRun *domain.BackupRun, server *domain.Server, storageTarget *domain.StorageTarget) error {
	// Validate inputs: none of these should be nil
	if backupRun == nil {
		e.logger.ErrorContext(ctx, "invalid input: backupRun is nil")
		return fmt.Errorf("backup run cannot be nil")
	}
	if server == nil {
		e.logger.ErrorContext(ctx, "invalid input: server is nil")
		return fmt.Errorf("server cannot be nil")
	}
	if storageTarget == nil {
		e.logger.ErrorContext(ctx, "invalid input: storageTarget is nil")
		return fmt.Errorf("storage target cannot be nil")
	}

	e.logger.InfoContext(ctx, "backup execution started",
		slog.String("backupRunId", backupRun.ID),
		slog.String("serverId", server.ID),
	)

	// Mark as running
	if err := e.repos.BackupRuns.UpdateStatus(ctx, backupRun.ID, "running", nil); err != nil {
		e.logger.ErrorContext(ctx, "failed to mark backup running",
			slog.String("backupRunId", backupRun.ID),
			slog.String("error", err.Error()),
		)
		return err
	}

	// Decrypt SSH private key
	privateKeyPEM, err := e.sealer.Decrypt(server.ID, "ssh_private_key_encrypted", server.SSHPrivateKeyEncrypted)
	if err != nil {
		errMsg := backupcore.TruncateText(fmt.Sprintf("falha ao decriptar chave SSH: %s", err.Error()), backupcore.MaxStoredErrorLen)
		e.logger.ErrorContext(ctx, "failed to decrypt SSH key",
			slog.String("serverId", server.ID),
			slog.String("error", errMsg),
		)
		_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, nil)
		return fmt.Errorf("decrypt ssh private key: %w", err)
	}

	// Build the storage backend for this target (decrypts whichever secret
	// the target type needs — see internal/storage.NewBackend).
	backend, err := storage.NewBackend(*storageTarget, e.sealer)
	if err != nil {
		errMsg := backupcore.TruncateText(fmt.Sprintf("falha ao decriptar credencial de armazenamento: %s", err.Error()), backupcore.MaxStoredErrorLen)
		e.logger.ErrorContext(ctx, "failed to build storage backend",
			slog.String("targetId", storageTarget.ID),
			slog.String("error", errMsg),
		)
		_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, nil)
		return fmt.Errorf("build storage backend: %w", err)
	}

	// SSH dial with TOFU validation
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	host := fmt.Sprintf("%s:%d", server.Host, server.Port)
	sshClient, _, err := sshclient.Connect(connectCtx, sshclient.ConnectOptions{
		User:              server.SSHUser,
		Host:              host,
		PrivateKeyPEM:     []byte(privateKeyPEM),
		Timeout:           30 * time.Second,
		ExpectedHostKeyFP: server.SSHHostKeyFingerprint,
	})
	if err != nil {
		errMsg := backupcore.TruncateText(fmt.Sprintf("falha na conexão SSH: %s", err.Error()), backupcore.MaxStoredErrorLen)
		e.logger.ErrorContext(ctx, "SSH connection failed",
			slog.String("serverId", server.ID),
			slog.String("host", host),
			slog.String("error", errMsg),
		)
		_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, nil)
		return fmt.Errorf("ssh dial: %w", err)
	}
	defer sshClient.Close()

	// Decrypt the DB password, if one is configured (RN-BACKUP-017: required
	// for mysql/sqlserver, optional for postgres).
	var dbPassword string
	if server.DBPasswordEncrypted != nil {
		dbPassword, err = e.sealer.Decrypt(server.ID, domain.DBPasswordAAD, server.DBPasswordEncrypted)
		if err != nil {
			errMsg := backupcore.TruncateText(fmt.Sprintf("falha ao decriptar senha do banco: %s", err.Error()), backupcore.MaxStoredErrorLen)
			e.logger.ErrorContext(ctx, "failed to decrypt DB password",
				slog.String("serverId", server.ID),
				slog.String("error", errMsg),
			)
			_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, nil)
			return fmt.Errorf("decrypt db password: %w", err)
		}
	}

	// Resolve the configured (possibly partial) container name against the
	// remote Docker daemon before building the dump command (RN-BACKUP-023).
	dumpServer := *server
	if server.DeploymentMode == domain.DeploymentModeDocker {
		resolvedName, resolveErr := ResolveContainerName(ctx, sshClient, *server.ContainerName)
		if resolveErr != nil {
			errMsg := backupcore.TruncateText(backupcore.RedactSecret(fmt.Sprintf("falha ao resolver nome do container: %s", resolveErr.Error()), dbPassword), backupcore.MaxStoredErrorLen)
			e.logger.ErrorContext(ctx, "failed to resolve container name",
				slog.String("serverId", server.ID),
				slog.String("error", errMsg),
			)
			_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, nil)
			return fmt.Errorf("resolve container name: %w", resolveErr)
		}
		dumpServer.ContainerName = &resolvedName
	}

	// Build the dump command(s) with shell quoting (RN-BACKUP-008)
	plan, err := BuildDumpPlan(dumpServer, dbPassword)
	if err != nil {
		errMsg := backupcore.TruncateText(backupcore.RedactSecret(fmt.Sprintf("falha ao preparar comando de dump: %s", err.Error()), dbPassword), backupcore.MaxStoredErrorLen)
		e.logger.ErrorContext(ctx, "failed to build dump plan",
			slog.String("serverId", server.ID),
			slog.String("error", errMsg),
		)
		_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, nil)
		return fmt.Errorf("build dump plan: %w", err)
	}

	// Register the SQL Server temp-file cleanup defer BEFORE the PreCmd
	// failure check below (which can return early): cleanup must be
	// attempted whether BACKUP DATABASE itself fails partway (e.g. disk
	// full), the stream fails, or everything succeeds (RN-BACKUP-020).
	// Registering it after that check would skip cleanup entirely on a
	// PreCmd failure, leaving a partial/complete plaintext dump behind.
	if plan.CleanupCmd != "" {
		defer func() {
			// Independent, bounded timeout: cleanup must still be attempted
			// even if the original ctx was already canceled/expired by the
			// time the dump/upload finished.
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			if _, _, exitCode, cleanupErr := sshClient.RunCommand(cleanupCtx, plan.CleanupCmd); cleanupErr != nil || exitCode != 0 {
				e.logger.ErrorContext(ctx, "failed to clean up SQL Server temp backup file (non-fatal)",
					slog.String("serverId", server.ID),
					slog.Int("exitCode", exitCode),
					slog.String("error", fmt.Sprintf("%v", cleanupErr)),
				)
			}
		}()
	}

	// SQL Server only: run BACKUP DATABASE synchronously before the file it
	// wrote can be streamed. Cleanup of the temp file it produced is always
	// attempted (see the deferred cleanup above), whether this step, the
	// stream, or the upload succeeds or fails.
	if plan.PreCmd != "" {
		stdout, stderr, exitCode, runErr := sshClient.RunCommand(ctx, plan.PreCmd)
		if runErr != nil || exitCode != 0 {
			errMsg := backupcore.TruncateText(backupcore.RedactSecret(fmt.Sprintf("falha ao executar BACKUP DATABASE (exit code %d)", exitCode), dbPassword), backupcore.MaxStoredErrorLen)
			combinedLog := backupcore.TruncateText(backupcore.RedactSecret(strings.TrimSpace("stdout: "+stdout+"\nstderr: "+stderr), dbPassword), backupcore.MaxStoredLogLen)
			e.logger.ErrorContext(ctx, "BACKUP DATABASE command failed",
				slog.String("serverId", server.ID),
				slog.Int("exitCode", exitCode),
				slog.String("log", combinedLog),
			)
			_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, &combinedLog)
			if runErr != nil {
				return fmt.Errorf("run backup database command: %w", runErr)
			}
			return fmt.Errorf("run backup database command: exit code %d", exitCode)
		}
	}

	// Start the dump stream
	dumpStart := time.Now()
	stdout, wait, err := sshClient.StreamCommand(ctx, plan.StreamCmd)
	if err != nil {
		errMsg := backupcore.TruncateText(backupcore.RedactSecret(fmt.Sprintf("falha ao iniciar o dump: %s", err.Error()), dbPassword), backupcore.MaxStoredErrorLen)
		e.logger.ErrorContext(ctx, "failed to start dump stream",
			slog.String("serverId", server.ID),
			slog.String("error", errMsg),
		)
		_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, nil)
		return fmt.Errorf("stream command: %w", err)
	}

	// Build blob name: <slugified-server>/<dbname>_<timestamp>.dump
	blobName := fmt.Sprintf("%s/%s_%s.dump", slugify(server.Name), server.DBName, time.Now().Format(time.RFC3339))

	// Upload stream to the storage backend
	uploadStart := time.Now()
	uploadedBytes, err := backend.UploadStream(ctx, blobName, stdout)
	dumpDurationMS := int(time.Since(dumpStart).Milliseconds())
	uploadDurationMS := int(time.Since(uploadStart).Milliseconds())

	// Wait for SSH command to complete
	waitErr := wait()
	if err != nil || waitErr != nil {
		var rawMsg string
		switch {
		case waitErr != nil && err != nil:
			rawMsg = fmt.Sprintf("falha no dump e no upload: dump=%s upload=%s", waitErr.Error(), err.Error())
		case waitErr != nil:
			rawMsg = fmt.Sprintf("falha no dump: %s", waitErr.Error())
		default:
			rawMsg = fmt.Sprintf("falha no upload: %s", err.Error())
		}
		errMsg := backupcore.TruncateText(backupcore.RedactSecret(rawMsg, dbPassword), backupcore.MaxStoredErrorLen)
		var logOutput *string
		if waitErr != nil {
			lo := backupcore.TruncateText(backupcore.RedactSecret(waitErr.Error(), dbPassword), backupcore.MaxStoredLogLen)
			logOutput = &lo
		}
		e.logger.ErrorContext(ctx, "backup upload or dump failed",
			slog.String("backupRunId", backupRun.ID),
			slog.String("error", errMsg),
		)
		// Attempt cleanup of partial blob
		cleanupErr := backend.DeleteBlob(ctx, blobName)
		if cleanupErr != nil {
			e.logger.ErrorContext(ctx, "failed to cleanup partial blob",
				slog.String("blobName", blobName),
				slog.String("error", cleanupErr.Error()),
			)
		}
		_ = e.repos.BackupRuns.MarkFailedWithDetails(ctx, backupRun.ID, errMsg, logOutput)
		if err != nil {
			return fmt.Errorf("upload stream: %w", err)
		}
		return fmt.Errorf("wait command: %w", waitErr)
	}

	// Mark backup as successful with blob metadata and timing (RN-BACKUP-022:
	// MarkSuccess is the method that persists dump_duration_ms/upload_duration_ms;
	// the previous UpdateBlobMetadata+UpdateStatus pair never wrote them, so
	// every scheduler-driven run kept those columns NULL even though the
	// values were computed above).
	if err := e.repos.BackupRuns.MarkSuccess(ctx, backupRun.ID, blobName, uploadedBytes, dumpDurationMS, uploadDurationMS); err != nil {
		e.logger.ErrorContext(ctx, "failed to mark backup success",
			slog.String("backupRunId", backupRun.ID),
			slog.String("error", err.Error()),
		)
		return err
	}

	e.logger.InfoContext(ctx, "backup uploaded successfully",
		slog.String("backupRunId", backupRun.ID),
		slog.String("blobName", blobName),
		slog.Int64("uploadedBytes", uploadedBytes),
		slog.Int("dumpDurationMs", dumpDurationMS),
		slog.Int("uploadDurationMs", uploadDurationMS),
	)

	// Run retention sweep post-backup (non-fatal on error)
	if err := e.sweepRetention(ctx, backend, server, backupRun.ID); err != nil {
		e.logger.ErrorContext(ctx, "retention sweep failed (non-fatal)",
			slog.String("serverId", server.ID),
			slog.String("backupRunId", backupRun.ID),
			slog.String("error", err.Error()),
		)
		// Don't fail the backup — retention failure is post-hoc cleanup
	}

	e.logger.InfoContext(ctx, "backup completed successfully",
		slog.String("backupRunId", backupRun.ID),
		slog.String("blobName", blobName),
		slog.Int64("uploadedBytes", uploadedBytes),
	)
	return nil
}

// sweepRetention runs the GFS retention policy post-backup (non-fatal),
// delegating the list/decide/delete/audit sequence to the shared
// backupcore.SweepRetention helper also used by
// httpapi/handlers/backup.go's sweepRetention — see docs/Arquitetura.md for
// the duplication this replaced.
func (e *BackupExecutor) sweepRetention(ctx context.Context, backend storage.Backend, server *domain.Server, backupRunID string) error {
	dbPolicy, err := e.repos.RetentionPolicies.EffectiveForServer(ctx, server.ID)
	if err != nil {
		return fmt.Errorf("get retention policy: %w", err)
	}

	policy := retention.Policy{RecentCount: dbPolicy.RecentCount, MonthlyCount: dbPolicy.MonthlyCount}

	onAuditFailure := func(ctx context.Context, serverID, runID, blobName string, auditErr error) error {
		e.logger.ErrorContext(ctx, "retention deletion audit write failed after delete succeeded",
			slog.String("stage", "audit_write_after_delete_succeeded"),
			slog.String("serverId", serverID),
			slog.String("backupRunId", runID),
			slog.String("blobName", blobName),
			slog.String("error", auditErr.Error()),
		)
		return fmt.Errorf("record retention deletion audit for blob %q (blob already deleted): %w", blobName, auditErr)
	}

	result, err := backupcore.SweepRetention(ctx, backend, slugify(server.Name)+"/", policy, server.ID, backupRunID, false, e.repos.RetentionDeletions, onAuditFailure)
	if err != nil {
		return err
	}

	e.logger.InfoContext(ctx, "retention sweep completed",
		slog.String("serverId", server.ID),
		slog.Int("deletedCount", len(result.Affected)),
	)
	return nil
}

// slugify converts a server name into a safe, single-path-segment folder name
// for blob storage: lowercased, non-alphanumeric runs collapsed to a single
// hyphen, leading/trailing hyphens trimmed. This is a security boundary — the
// result is concatenated directly into a blob name.
func slugify(name string) string {
	var b strings.Builder
	lastWasHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasHyphen = false
		default:
			if !lastWasHyphen && b.Len() > 0 {
				b.WriteRune('-')
				lastWasHyphen = true
			}
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		slug = "server"
	}
	return slug
}
