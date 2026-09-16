package handlers

import (
	"errors"
	"net"
	"net/http"
	"time"

	"backapeando-backup-manager/internal/auth"
	"backapeando-backup-manager/internal/httpapi/middleware"
	"backapeando-backup-manager/internal/repository"
)

type AuthHandlers struct {
	Users         *repository.AdminUserRepo
	Sessions      *auth.SessionManager
	RateLimiter   auth.RateLimiter
	SecureCookies bool
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	allowed, err := h.RateLimiter.Allow(r.Context(), ip)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}

	var req loginRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.Users.GetByEmail(r.Context(), req.Email)
	userFound := err == nil
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Always run the (expensive, deliberately slow) Argon2id verification,
	// against the real hash if the user exists or a dummy one otherwise —
	// same error message AND same response latency in both cases, so an
	// unknown email can't be distinguished from a wrong password by timing.
	hashToVerify := auth.DummyHash()
	if userFound {
		hashToVerify = user.PasswordHash
	}
	verifyErr := auth.VerifyPassword(hashToVerify, req.Password)

	if !userFound || verifyErr != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := h.Sessions.Issue(r.Context(), w, user.ID, r.UserAgent(), ip); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if _, err := auth.IssueCSRFToken(w, h.SecureCookies); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	_ = h.Users.TouchLastLogin(r.Context(), user.ID, time.Now())

	writeJSON(w, http.StatusOK, map[string]string{"id": user.ID, "email": user.Email, "role": user.Role})
}

func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.Sessions.Clear(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	user, err := h.Users.GetByID(r.Context(), session.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": user.ID, "email": user.Email, "role": user.Role})
}
