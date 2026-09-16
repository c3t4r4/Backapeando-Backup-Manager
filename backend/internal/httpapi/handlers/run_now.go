package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

// backupRunRepository defines the methods needed by RunNowHandlers to queue backups.
type backupRunRepository interface {
	CreateQueued(ctx context.Context, serverID string) (domain.BackupRun, error)
}

// serverRepository defines the methods needed by RunNowHandlers to retrieve servers.
type serverRepository interface {
	Get(ctx context.Context, id string) (domain.Server, error)
}

// RunNowHandlers implements the POST /api/servers/{id}/run-now endpoint,
// which enqueues a backup for immediate execution by the scheduler (Fase 4).
// Unlike the synchronous backup-now handler (BackupHandlers.BackupNow), this
// handler creates a backup run with status='queued' and returns 202 Accepted,
// letting the scheduler discover and execute it asynchronously.
type RunNowHandlers struct {
	BackupRuns backupRunRepository
	Servers    serverRepository
	Logger     *slog.Logger
}

type runNowResponse struct {
	BackupRunID string `json:"backupRunId"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// RunServerBackupNow enqueues a backup for a server, returning 202 Accepted.
// The scheduler will pick up the queued run and execute it asynchronously.
//
// Prerequisites (RN-BACKUP-006):
// - Server must exist
// - Server status must be "ready"
// - Server must be enabled (enabled=true)
// - Server must have a storage target configured (StorageTargetID != nil)
//
// If any prerequisite is not met, returns 409 Conflict.
func (h *RunNowHandlers) RunServerBackupNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// 1. Fetch server
	server, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		if h.Logger != nil {
			h.Logger.ErrorContext(r.Context(), "get server error", slog.String("error", err.Error()), slog.String("serverId", id))
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// 2. Validate server prerequisites (RN-BACKUP-006)
	if server.Status != domain.ServerStatusReady || !server.Enabled || server.StorageTargetID == nil {
		writeError(w, http.StatusConflict, "server is not ready for backup: requires status=ready, enabled=true, and a storage target configured")
		return
	}

	// 3. Create BackupRun with status='queued'
	backupRun, err := h.BackupRuns.CreateQueued(r.Context(), server.ID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.ErrorContext(r.Context(), "create queued backup run error", slog.String("error", err.Error()), slog.String("serverId", id))
		}
		writeError(w, http.StatusInternalServerError, "failed to create backup run")
		return
	}

	// 4. Return 202 Accepted with run info
	if h.Logger != nil {
		h.Logger.InfoContext(r.Context(), "backup run created via /run-now", slog.String("runId", backupRun.ID), slog.String("serverId", id))
	}
	response := runNowResponse{
		BackupRunID: backupRun.ID,
		Status:      string(backupRun.Status),
		Message:     "backup queued; scheduler will execute it",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(response)
}
