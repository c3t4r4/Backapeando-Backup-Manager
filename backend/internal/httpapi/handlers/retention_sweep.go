package handlers

import (
	"net/http"

	"backapeando-backup-manager/internal/repository"
)

type RetentionSweepHandlers struct {
	Requests *repository.RetentionSweepRequestRepo
}

// Trigger enqueues a new global retention sweep request.
// Returns 202 Accepted with the request ID and initial status.
func (h *RetentionSweepHandlers) Trigger(w http.ResponseWriter, r *http.Request) {
	req, err := h.Requests.Create(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create sweep request")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":     req.ID,
		"status": req.Status,
	})
}

// Latest returns the most recently created sweep request (for polling the status).
func (h *RetentionSweepHandlers) Latest(w http.ResponseWriter, r *http.Request) {
	req, err := h.Requests.GetLatest(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get latest sweep request")
		return
	}

	if req == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "no_requests",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         req.ID,
		"status":     req.Status,
		"summary":    req.Summary,
		"error":      req.Error,
		"startedAt":  req.StartedAt,
		"finishedAt": req.FinishedAt,
	})
}
