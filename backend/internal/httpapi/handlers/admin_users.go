package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/cpf"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/httpapi/middleware"
	"backapeando-backup-manager/internal/repository"

	"github.com/jackc/pgx/v5/pgconn"
)

// AdminUserHandlers implements CRUD over admin_users — the operators who can
// log into this system. Management was CLI-only (./api create-admin,
// ./api reset-password) until this web CRUD was added; the CLI remains as a
// host-level fallback.
type AdminUserHandlers struct {
	Users *repository.AdminUserRepo
}

// adminUserDTO never includes password_hash — once written, a password is
// only ever re-set via Update, never read back (same rule as storage target
// secrets in storage_targets.go).
type adminUserDTO struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	CPF         string     `json:"cpf"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

func toAdminUserDTO(u domain.AdminUser) adminUserDTO {
	return adminUserDTO{
		ID:          u.ID,
		Email:       u.Email,
		CPF:         u.CPF,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

type createAdminUserRequest struct {
	Email    string `json:"email"`
	CPF      string `json:"cpf"`
	Password string `json:"password"`
}

// validate mirrors the CLI's own rule (backend/cmd/api/main.go runCreateAdmin)
// — byte length, not rune count, kept identical on purpose.
func (req createAdminUserRequest) validate() error {
	if strings.TrimSpace(req.Email) == "" {
		return errors.New("email is required")
	}
	if len(req.Password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	return nil
}

// updateAdminUserRequest.Password is optional: empty means "keep the current
// password" — same UX rule already used for storage target secrets
// (upsertStorageTargetRequest in storage_targets.go).
type updateAdminUserRequest struct {
	Email    string `json:"email"`
	CPF      string `json:"cpf"`
	Password string `json:"password"`
}

func (req updateAdminUserRequest) validate() error {
	if strings.TrimSpace(req.Email) == "" {
		return errors.New("email is required")
	}
	if req.Password != "" && len(req.Password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	return nil
}

// writeAdminUserConflictError inspects err for a unique-constraint violation
// on admin_users (email or cpf) and, if found, writes a 409 with a friendly
// message. Returns true if it already wrote a response — the caller must not
// write again in that case.
func writeAdminUserConflictError(w http.ResponseWriter, err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	switch pgErr.ConstraintName {
	case "admin_users_email_key":
		writeError(w, http.StatusConflict, "email already registered")
	case "admin_users_cpf_unique":
		writeError(w, http.StatusConflict, "cpf already registered")
	default:
		writeError(w, http.StatusConflict, "duplicate value")
	}
	return true
}

func (h *AdminUserHandlers) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.Users.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	dtos := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		dtos = append(dtos, toAdminUserDTO(u))
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (h *AdminUserHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u, err := h.Users.GetByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "admin user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserDTO(u))
}

func (h *AdminUserHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req createAdminUserRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	normalizedCPF, err := cpf.Validate(req.CPF)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid cpf: %v", err))
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// role is hardcoded "admin" — the system is single-role today (same as
	// the CLI's runCreateAdmin); no role selector is exposed in the UI.
	created, err := h.Users.Create(r.Context(), req.Email, normalizedCPF, hash, "admin")
	if err != nil {
		if writeAdminUserConflictError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, toAdminUserDTO(created))
}

func (h *AdminUserHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateAdminUserRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	normalizedCPF, err := cpf.Validate(req.CPF)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid cpf: %v", err))
		return
	}

	updated, err := h.Users.Update(r.Context(), id, req.Email, normalizedCPF)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "admin user not found")
		return
	}
	if err != nil {
		if writeAdminUserConflictError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if err := h.Users.UpdatePassword(r.Context(), id, hash); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		// Re-fetch so updatedAt in the response reflects the password change
		// too (UpdatePassword bumps it after the row above was read).
		updated, err = h.Users.GetByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	writeJSON(w, http.StatusOK, toAdminUserDTO(updated))
}

// Delete enforces two confirmed business rules (RN-AUTH-002, RN-AUTH-003): an
// admin cannot delete their own account, and the last remaining admin can
// never be deleted. Both checks run before the actual DELETE. Self-deletion
// is checked first so a lone admin gets the more specific message.
func (h *AdminUserHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if session.UserID == id {
		writeError(w, http.StatusConflict, "you cannot delete your own account")
		return
	}

	count, err := h.Users.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if count <= 1 {
		writeError(w, http.StatusConflict, "cannot delete the last remaining admin")
		return
	}

	if err := h.Users.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
