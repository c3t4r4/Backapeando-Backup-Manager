package handlers

import (
	"errors"
	"net/http"

	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

type RetentionPolicyHandlers struct {
	Policies *repository.RetentionPolicyRepo
}

type retentionPolicyDTO struct {
	ID           string  `json:"id"`
	ServerID     *string `json:"serverId,omitempty"`
	RecentCount  int     `json:"recentCount"`
	MonthlyCount int     `json:"monthlyCount"`
}

func toRetentionPolicyDTO(p domain.RetentionPolicy) retentionPolicyDTO {
	return retentionPolicyDTO{ID: p.ID, ServerID: p.ServerID, RecentCount: p.RecentCount, MonthlyCount: p.MonthlyCount}
}

type upsertRetentionPolicyRequest struct {
	RecentCount  int `json:"recentCount"`
	MonthlyCount int `json:"monthlyCount"`
}

func (req upsertRetentionPolicyRequest) validate() error {
	if req.RecentCount < 0 {
		return errors.New("recentCount must be zero or positive")
	}
	if req.MonthlyCount < 0 {
		return errors.New("monthlyCount must be zero or positive")
	}
	if req.RecentCount == 0 && req.MonthlyCount == 0 {
		return errors.New("recentCount and monthlyCount cannot both be zero — that would delete every backup")
	}
	return nil
}

func (h *RetentionPolicyHandlers) GetGlobal(w http.ResponseWriter, r *http.Request) {
	p, err := h.Policies.GetGlobalDefault(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toRetentionPolicyDTO(p))
}

func (h *RetentionPolicyHandlers) UpdateGlobal(w http.ResponseWriter, r *http.Request) {
	var req upsertRetentionPolicyRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := h.Policies.UpdateGlobalDefault(r.Context(), req.RecentCount, req.MonthlyCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toRetentionPolicyDTO(p))
}

// GetForServer returns the effective policy (server override if present,
// else the global default) so the SPA always has something to display.
func (h *RetentionPolicyHandlers) GetForServer(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	p, err := h.Policies.EffectiveForServer(r.Context(), serverID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toRetentionPolicyDTO(p))
}

func (h *RetentionPolicyHandlers) UpsertForServer(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	var req upsertRetentionPolicyRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := h.Policies.UpsertForServer(r.Context(), serverID, req.RecentCount, req.MonthlyCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toRetentionPolicyDTO(p))
}

// DeleteForServer removes the server-specific override, falling back to the
// global default again.
func (h *RetentionPolicyHandlers) DeleteForServer(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	if err := h.Policies.DeleteForServer(r.Context(), serverID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
