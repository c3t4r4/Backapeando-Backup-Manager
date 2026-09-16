package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"backapeando-backup-manager/internal/backupcore"
	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
	"backapeando-backup-manager/internal/retention"
	"backapeando-backup-manager/internal/scheduler"
	"backapeando-backup-manager/internal/sshclient"
	"backapeando-backup-manager/internal/storage"
)

// BackupHandlers implements the manual, synchronous backup-now endpoint
// (RN-BACKUP-006/007). The scheduler (Fase 4) also uses this same
// orchestration for automatic runs, via the shared internal/backupcore
// package for the retention-sweep step.
type BackupHandlers struct {
	Servers            *repository.ServerRepo
	StorageTargets     *repository.StorageTargetRepo
	RetentionPolicies  *repository.RetentionPolicyRepo
	BackupRuns         *repository.BackupRunRepo
	RetentionDeletions *repository.RetentionDeletionRepo
	Sealer             *crypto.Sealer
	Logger             *slog.Logger
}

type backupRunDTO struct {
	ID               string     `json:"id"`
	ServerID         string     `json:"serverId"`
	Status           string     `json:"status"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	BlobName         *string    `json:"blobName,omitempty"`
	BlobSizeBytes    *int64     `json:"blobSizeBytes,omitempty"`
	DumpDurationMS   *int       `json:"dumpDurationMs,omitempty"`
	UploadDurationMS *int       `json:"uploadDurationMs,omitempty"`
	ErrorMessage     *string    `json:"errorMessage,omitempty"`
	LogOutput        *string    `json:"logOutput,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

func toBackupRunDTO(b domain.BackupRun) backupRunDTO {
	return backupRunDTO{
		ID: b.ID, ServerID: b.ServerID, Status: string(b.Status),
		StartedAt: b.StartedAt, FinishedAt: b.FinishedAt, BlobName: b.BlobName,
		BlobSizeBytes: b.BlobSizeBytes, DumpDurationMS: b.DumpDurationMS,
		UploadDurationMS: b.UploadDurationMS, ErrorMessage: b.ErrorMessage,
		LogOutput: b.LogOutput, CreatedAt: b.CreatedAt,
	}
}

type backupNowRequest struct {
	DryRun bool `json:"dryRun"`
}

type retentionResultDTO struct {
	DryRun      bool     `json:"dryRun"`
	WouldDelete []string `json:"wouldDelete,omitempty"`
	Deleted     []string `json:"deleted,omitempty"`
}

type backupNowResponse struct {
	BackupRun backupRunDTO        `json:"backupRun"`
	Retention *retentionResultDTO `json:"retention"`
}

// slugify converts a server name into a safe, single-path-segment folder
// name for blob storage: lowercased, non-alphanumeric runs collapsed to a
// single hyphen, leading/trailing hyphens trimmed. This is a security
// boundary, not just cosmetics — the result is concatenated directly into a
// blob name, so it must never be able to produce "..", a leading "/", or an
// embedded "/" that would let a crafted server name escape its own prefix
// and collide with (or list/delete) another server's blobs.
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

// BackupNow runs a full backup synchronously: pg_dump over SSH, streamed
// directly into the server's configured storage target, followed by a
// retention sweep (unless dryRun). See RN-BACKUP-006 (prerequisites) and
// RN-BACKUP-007 (dryRun semantics: it only affects the retention step,
// never the backup itself).
func (h *BackupHandlers) BackupNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req backupNowRequest
	if r.ContentLength != 0 {
		if err := readJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	server, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// RN-BACKUP-006: backup-now requires status=ready AND enabled=true AND
	// a storage target configured.
	if server.Status != domain.ServerStatusReady || !server.Enabled || server.StorageTargetID == nil {
		writeError(w, http.StatusConflict, "server is not ready for backup: requires status=ready, enabled=true, and a storage target configured")
		return
	}

	storageTarget, err := h.StorageTargets.Get(r.Context(), *server.StorageTargetID)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusConflict, "server's storage target no longer exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	backend, err := storage.NewBackend(storageTarget, h.Sealer)
	if err != nil {
		h.logError(r.Context(), id, "", "build storage backend", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	run, err := h.BackupRuns.Create(r.Context(), server.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	blobName, blobSizeBytes, dumpDurationMS, uploadDurationMS, logOutput, runErr := h.performBackup(r.Context(), server, backend)
	if runErr != nil {
		h.logError(r.Context(), id, run.ID, "backup", runErr)
		var logOutputPtr *string
		if logOutput != "" {
			logOutputPtr = &logOutput
		}
		if err := h.BackupRuns.MarkFailedWithDetails(r.Context(), run.ID, runErr.Error(), logOutputPtr); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		failedRun, err := h.BackupRuns.Get(r.Context(), run.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		// A failed backup is a valid, recorded outcome — not an HTTP error.
		writeJSON(w, http.StatusOK, backupNowResponse{BackupRun: toBackupRunDTO(failedRun), Retention: nil})
		return
	}

	if err := h.BackupRuns.MarkSuccess(r.Context(), run.ID, blobName, blobSizeBytes, dumpDurationMS, uploadDurationMS); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	successRun, err := h.BackupRuns.Get(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	retentionResult, err := h.sweepRetention(r.Context(), server, backend, run.ID, req.DryRun)
	if err != nil {
		// The backup itself already succeeded and is durably recorded;
		// a retention-sweep failure is logged but does not turn a
		// successful backup into an HTTP error.
		h.logError(r.Context(), id, run.ID, "retention sweep", err)
		writeJSON(w, http.StatusOK, backupNowResponse{BackupRun: toBackupRunDTO(successRun), Retention: nil})
		return
	}

	writeJSON(w, http.StatusOK, backupNowResponse{BackupRun: toBackupRunDTO(successRun), Retention: retentionResult})
}

// finalizeBackupError redacts the database password (when configured) out of
// err/logOutput and truncates both to the size bounds enforced on
// backup_runs.error_message/log_output (RN-BACKUP-027). Extracted out of
// performBackup's defer so this exact logic — including the dbPassword=""
// case (e.g. Postgres with trust/peer auth, no password configured), which
// must still truncate even though there's nothing to redact — is
// unit-testable without a live SSH/storage backend.
func finalizeBackupError(err error, logOutput, dbPassword string) (error, string) {
	if err != nil {
		err = errors.New(backupcore.TruncateText(backupcore.RedactSecret(err.Error(), dbPassword), backupcore.MaxStoredErrorLen))
	}
	if logOutput != "" {
		logOutput = backupcore.TruncateText(backupcore.RedactSecret(logOutput, dbPassword), backupcore.MaxStoredLogLen)
	}
	return err, logOutput
}

// performBackup runs the server's configured engine's dump command over SSH
// (RN-BACKUP-008: shell-quoted via scheduler.BuildDumpPlan, the same builder
// used by the scheduler's executor.go) and streams it into the server's
// storage backend, without ever touching local disk. It returns the blob
// name, its size, and timing for both stages.
func (h *BackupHandlers) performBackup(ctx context.Context, server domain.Server, backend storage.Backend) (blobName string, blobSizeBytes int64, dumpDurationMS, uploadDurationMS int, logOutput string, err error) {
	// dbPassword is declared up front so the deferred call below can close
	// over it regardless of which return path fires: err/logOutput must
	// never carry the plaintext database password back to the caller,
	// whether it ends up in backup_runs.error_message/log_output or the
	// server-side log (RN-BACKUP-027, same masking spirit as RN-BACKUP-021).
	var dbPassword string
	defer func() {
		err, logOutput = finalizeBackupError(err, logOutput, dbPassword)
	}()

	privateKeyPEMStr, err := h.Sealer.Decrypt(server.ID, "ssh_private_key_encrypted", server.SSHPrivateKeyEncrypted)
	if err != nil {
		return "", 0, 0, 0, "", fmt.Errorf("decrypt ssh private key: %w", err)
	}

	if server.DBPasswordEncrypted != nil {
		dbPassword, err = h.Sealer.Decrypt(server.ID, domain.DBPasswordAAD, server.DBPasswordEncrypted)
		if err != nil {
			return "", 0, 0, 0, "", fmt.Errorf("decrypt db password: %w", err)
		}
	}

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	host := fmt.Sprintf("%s:%d", server.Host, server.Port)
	client, _, err := sshclient.Connect(connectCtx, sshclient.ConnectOptions{
		User:              server.SSHUser,
		Host:              host,
		PrivateKeyPEM:     []byte(privateKeyPEMStr),
		Timeout:           30 * time.Second,
		ExpectedHostKeyFP: server.SSHHostKeyFingerprint,
	})
	if err != nil {
		return "", 0, 0, 0, "", fmt.Errorf("ssh connect: %w", err)
	}
	defer client.Close()

	// Resolve the configured (possibly partial) container name against the
	// remote Docker daemon before building the dump command (RN-BACKUP-023).
	// This requires a live SSH session, so it can only happen after Connect
	// above — unlike the scheduler's executor.go, this handler used to build
	// the plan before connecting.
	dumpServer := server
	if server.DeploymentMode == domain.DeploymentModeDocker {
		resolvedName, resolveErr := scheduler.ResolveContainerName(ctx, client, *server.ContainerName)
		if resolveErr != nil {
			return "", 0, 0, 0, "", fmt.Errorf("resolve container name: %w", resolveErr)
		}
		dumpServer.ContainerName = &resolvedName
	}

	plan, err := scheduler.BuildDumpPlan(dumpServer, dbPassword)
	if err != nil {
		return "", 0, 0, 0, "", fmt.Errorf("build dump plan: %w", err)
	}

	if plan.CleanupCmd != "" {
		defer func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			if _, _, exitCode, cleanupErr := client.RunCommand(cleanupCtx, plan.CleanupCmd); cleanupErr != nil || exitCode != 0 {
				h.logError(ctx, server.ID, "", "clean up SQL Server temp backup file (non-fatal)", fmt.Errorf("exit %d: %w", exitCode, cleanupErr))
			}
		}()
	}

	if plan.PreCmd != "" {
		preStdout, stderr, exitCode, runErr := client.RunCommand(ctx, plan.PreCmd)
		if runErr != nil || exitCode != 0 {
			logOutput = strings.TrimSpace("stdout: " + preStdout + "\nstderr: " + stderr)
			if runErr != nil {
				return "", 0, 0, 0, logOutput, fmt.Errorf("run backup database command: %w", runErr)
			}
			return "", 0, 0, 0, logOutput, fmt.Errorf("run backup database command: exit code %d: %s", exitCode, strings.TrimSpace(stderr))
		}
	}

	dumpStart := time.Now()
	stdout, wait, err := client.StreamCommand(ctx, plan.StreamCmd)
	if err != nil {
		return "", 0, 0, 0, "", fmt.Errorf("start dump stream: %w", err)
	}

	slug := slugify(server.Name)
	dbSlug := slugify(server.DBName)
	blobName = fmt.Sprintf("%s/%s_%s.dump", slug, dbSlug, time.Now().UTC().Format("20060102T150405Z"))

	uploadStart := time.Now()
	size, uploadErr := backend.UploadStream(ctx, blobName, stdout)
	uploadDurationMS = int(time.Since(uploadStart).Milliseconds())

	// Always wait() to release the SSH session and learn whether the dump
	// itself exited cleanly — a non-zero exit means the stream the upload
	// just consumed was truncated or empty, even if the upload "succeeded".
	waitErr := wait()
	dumpDurationMS = int(time.Since(dumpStart).Milliseconds())

	if waitErr != nil || uploadErr != nil {
		// The dump exited non-zero and/or the upload itself failed: the blob
		// (if it was created at all) holds truncated or empty data. Best-
		// effort clean it up so a later ListBlobs/retention sweep never
		// treats corrupt data as a legitimate backup to keep. Cleanup
		// failure is logged but must not mask the original error.
		if delErr := backend.DeleteBlob(ctx, blobName); delErr != nil {
			h.logError(ctx, server.ID, "", "cleanup partial blob after failed backup", delErr)
		}
		if waitErr != nil {
			logOutput = waitErr.Error()
			return "", 0, dumpDurationMS, uploadDurationMS, logOutput, fmt.Errorf("dump: %w", waitErr)
		}
		return "", 0, dumpDurationMS, uploadDurationMS, "", fmt.Errorf("upload: %w", uploadErr)
	}

	return blobName, size, dumpDurationMS, uploadDurationMS, "", nil
}

// auditWriteFailureAfterDelete is called when RetentionDeletions.Create
// fails after the storage backend's DeleteBlob has already succeeded — the
// blob is irrevocably gone by this point, so the only remaining question is
// whether its name survives somewhere. This function guarantees it does in
// two independent places: a structured server-side log line (with a stable
// "stage" marker so it is greppable/alertable even if nothing reads the
// return value), and the returned error itself (so the name also reaches
// whatever logs/wraps the error further up the call chain, e.g.
// BackupHandlers.logError). It is a standalone function specifically so it
// can be unit-tested without a real Postgres connection (RetentionDeletionRepo
// requires one; this function does not).
func auditWriteFailureAfterDelete(ctx context.Context, logger *slog.Logger, serverID, backupRunID, blobName string, auditErr error) error {
	if logger != nil {
		logger.ErrorContext(ctx, "retention deletion audit write failed after delete succeeded",
			slog.String("stage", "audit_write_after_delete_succeeded"),
			slog.String("serverId", serverID),
			slog.String("backupRunId", backupRunID),
			slog.String("blobName", blobName),
			slog.String("error", auditErr.Error()),
		)
	}
	return fmt.Errorf("record retention deletion audit for blob %q (blob already deleted): %w", blobName, auditErr)
}

// sweepRetention lists the server's existing blobs and applies the GFS
// retention policy, delegating to the shared backupcore.SweepRetention
// helper also used by the scheduler's executor.go — see docs/Arquitetura.md
// for the duplication this replaced. With dryRun=true, nothing is deleted
// and no audit rows are written (RN-BACKUP-007) — the response only reports
// what would be deleted.
func (h *BackupHandlers) sweepRetention(ctx context.Context, server domain.Server, backend storage.Backend, backupRunID string, dryRun bool) (*retentionResultDTO, error) {
	dbPolicy, err := h.RetentionPolicies.EffectiveForServer(ctx, server.ID)
	if err != nil {
		return nil, fmt.Errorf("load retention policy: %w", err)
	}
	policy := retention.Policy{RecentCount: dbPolicy.RecentCount, MonthlyCount: dbPolicy.MonthlyCount}

	onAuditFailure := func(ctx context.Context, serverID, runID, blobName string, auditErr error) error {
		return auditWriteFailureAfterDelete(ctx, h.Logger, serverID, runID, blobName, auditErr)
	}

	result, err := backupcore.SweepRetention(ctx, backend, slugify(server.Name)+"/", policy, server.ID, backupRunID, dryRun, h.RetentionDeletions, onAuditFailure)
	if err != nil {
		return nil, fmt.Errorf("sweep: %w", err)
	}

	dto := &retentionResultDTO{DryRun: dryRun}
	if dryRun {
		dto.WouldDelete = result.Affected
	} else {
		dto.Deleted = result.Affected
	}
	return dto, nil
}

func (h *BackupHandlers) logError(ctx context.Context, serverID, runID, stage string, err error) {
	if h.Logger == nil {
		return
	}
	h.Logger.ErrorContext(ctx, "backup-now failed",
		slog.String("serverId", serverID),
		slog.String("backupRunId", runID),
		slog.String("stage", stage),
		slog.String("error", err.Error()),
	)
}

// defaultBackupRunPageSize and maxBackupRunPageSize bound ListBackupRuns'
// pageSize query param (RN-BACKUP-025): default keeps the page small even
// when the caller omits it, max prevents an unbounded LIMIT from a client
// asking for e.g. pageSize=1000000.
const (
	defaultBackupRunPageSize = 50
	maxBackupRunPageSize     = 200
)

// backupRunListResponse is the paginated envelope returned by ListBackupRuns
// (RN-BACKUP-025) — a breaking change from the bare array previously
// returned, documented in docs/API.md.
type backupRunListResponse struct {
	Items    []backupRunDTO `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

// validBackupRunStatuses is used to reject an unrecognized ?status= value
// with 400 instead of silently matching zero rows.
var validBackupRunStatuses = map[string]bool{
	string(domain.BackupRunStatusQueued):  true,
	string(domain.BackupRunStatusRunning): true,
	string(domain.BackupRunStatusSuccess): true,
	string(domain.BackupRunStatusFailed):  true,
}

// parseBackupRunListParams parses and validates ListBackupRuns' page,
// pageSize, and status query params (RN-BACKUP-025), pulled out of the
// handler so the validation rules (bounds, unknown status) are unit
// testable without an HTTP request or a database.
func parseBackupRunListParams(query url.Values) (page, pageSize int, status *string, err error) {
	page = 1
	if v := query.Get("page"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 1 {
			return 0, 0, nil, fmt.Errorf("invalid page")
		}
		page = parsed
	}

	pageSize = defaultBackupRunPageSize
	if v := query.Get("pageSize"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed < 1 || parsed > maxBackupRunPageSize {
			return 0, 0, nil, fmt.Errorf("invalid pageSize: must be between 1 and %d", maxBackupRunPageSize)
		}
		pageSize = parsed
	}

	if v := query.Get("status"); v != "" {
		if !validBackupRunStatuses[v] {
			return 0, 0, nil, fmt.Errorf("invalid status")
		}
		status = &v
	}

	return page, pageSize, status, nil
}

// ListBackupRuns returns a paginated page of the backup history for a
// server, most recent first, optionally filtered by status (RN-BACKUP-025).
func (h *BackupHandlers) ListBackupRuns(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.Servers.Get(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	page, pageSize, status, err := parseBackupRunListParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	runs, total, err := h.BackupRuns.List(r.Context(), id, status, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	dtos := make([]backupRunDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, toBackupRunDTO(run))
	}
	writeJSON(w, http.StatusOK, backupRunListResponse{
		Items:    dtos,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// parseAllBackupRunListParams parses and validates ListAllBackupRuns' page,
// pageSize, status, and serverId query params. serverId is optional — its
// absence means "every server" (RN-BACKUP-026), unlike the nested
// /api/servers/{id}/backup-runs route where the server is structurally
// required by the path.
func parseAllBackupRunListParams(query url.Values) (page, pageSize int, status *string, serverID *string, err error) {
	page, pageSize, status, err = parseBackupRunListParams(query)
	if err != nil {
		return 0, 0, nil, nil, err
	}
	if v := query.Get("serverId"); v != "" {
		serverID = &v
	}
	return page, pageSize, status, serverID, nil
}

// ListAllBackupRuns returns a paginated page of backup history across every
// server, most recent first, defaulting to page 1 / 50 items when called
// with no query params — the default view of the History screen (T-06),
// which previously showed nothing until a server was selected
// (RN-BACKUP-026). Optionally filtered by status and/or a specific serverId.
func (h *BackupHandlers) ListAllBackupRuns(w http.ResponseWriter, r *http.Request) {
	page, pageSize, status, serverID, err := parseAllBackupRunListParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if serverID != nil {
		if _, err := h.Servers.Get(r.Context(), *serverID); errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "server not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	runs, total, err := h.BackupRuns.ListAll(r.Context(), serverID, status, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	dtos := make([]backupRunDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, toBackupRunDTO(run))
	}
	writeJSON(w, http.StatusOK, backupRunListResponse{
		Items:    dtos,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
