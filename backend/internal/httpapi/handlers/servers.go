package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
	"backapeando-backup-manager/internal/scheduler"
	"backapeando-backup-manager/internal/sshkeys"
)

var containerNameRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type ServerHandlers struct {
	Servers        *repository.ServerRepo
	StorageTargets *repository.StorageTargetRepo
	Sealer         *crypto.Sealer
	Logger         *slog.Logger
}

// serverDTO is the wire representation of a server. It deliberately omits
// SSHPrivateKeyEncrypted and DBPasswordEncrypted — those fields must never
// leave the backend, even encrypted. HasDBPassword only indicates whether a
// password is currently configured, so the frontend can render "leave blank
// to keep the current one" without ever seeing the value (RN-BACKUP-021).
type serverDTO struct {
	ID                      string     `json:"id"`
	Name                    string     `json:"name"`
	Host                    string     `json:"host"`
	Port                    int        `json:"port"`
	SSHUser                 string     `json:"sshUser"`
	DBEngine                string     `json:"dbEngine"`
	DeploymentMode          string     `json:"deploymentMode"`
	ContainerName           *string    `json:"containerName,omitempty"`
	DBName                  string     `json:"dbName"`
	DBUser                  string     `json:"dbUser"`
	HasDBPassword           bool       `json:"hasDbPassword"`
	PgDumpExtraArgs         string     `json:"pgDumpExtraArgs"`
	MySQLDumpExtraArgs      string     `json:"mysqlDumpExtraArgs"`
	SqlCmdExtraArgs         string     `json:"sqlCmdExtraArgs"`
	SSHPublicKey            *string    `json:"sshPublicKey,omitempty"`
	SSHKeyFingerprint       *string    `json:"sshKeyFingerprint,omitempty"`
	StorageTargetID         *string    `json:"storageTargetId,omitempty"`
	CronExpression          string     `json:"cronExpression"`
	Enabled                 bool       `json:"enabled"`
	Status                  string     `json:"status"`
	LastTestConnectionAt    *time.Time `json:"lastTestConnectionAt,omitempty"`
	LastTestConnectionOK    *bool      `json:"lastTestConnectionOk,omitempty"`
	LastTestConnectionError *string    `json:"lastTestConnectionError,omitempty"`
	NextRunAt               *time.Time `json:"nextRunAt,omitempty"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

func toServerDTO(s domain.Server) serverDTO {
	return serverDTO{
		ID: s.ID, Name: s.Name, Host: s.Host, Port: s.Port, SSHUser: s.SSHUser,
		DBEngine: string(s.DBEngine), DeploymentMode: string(s.DeploymentMode),
		ContainerName: s.ContainerName, DBName: s.DBName, DBUser: s.DBUser,
		HasDBPassword:      s.DBPasswordEncrypted != nil,
		PgDumpExtraArgs:    s.PgDumpExtraArgs,
		MySQLDumpExtraArgs: s.MySQLDumpExtraArgs, SqlCmdExtraArgs: s.SqlCmdExtraArgs,
		SSHPublicKey:      s.SSHPublicKey,
		SSHKeyFingerprint: s.SSHKeyFingerprint, StorageTargetID: s.StorageTargetID,
		CronExpression: s.CronExpression, Enabled: s.Enabled, Status: string(s.Status),
		LastTestConnectionAt: s.LastTestConnectionAt, LastTestConnectionOK: s.LastTestConnectionOK,
		LastTestConnectionError: s.LastTestConnectionError,
		NextRunAt:               s.NextRunAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

type upsertServerRequest struct {
	Name               string  `json:"name"`
	Host               string  `json:"host"`
	Port               int     `json:"port"`
	SSHUser            string  `json:"sshUser"`
	DBEngine           string  `json:"dbEngine"`
	DeploymentMode     string  `json:"deploymentMode"`
	ContainerName      string  `json:"containerName"`
	DBName             string  `json:"dbName"`
	DBUser             string  `json:"dbUser"`
	DBPassword         string  `json:"dbPassword"`
	PgDumpExtraArgs    string  `json:"pgDumpExtraArgs"`
	MySQLDumpExtraArgs string  `json:"mysqlDumpExtraArgs"`
	SqlCmdExtraArgs    string  `json:"sqlCmdExtraArgs"`
	StorageTargetID    *string `json:"storageTargetId"`
	CronExpression     string  `json:"cronExpression"`
}

// validate checks structural/format rules for every field, plus the
// cross-field rules introduced by RN-BACKUP-017/018: containerName is
// required only in docker mode (and forbidden in host mode), and dbPassword
// is required for non-postgres engines whenever hasExistingPassword is
// false — i.e. always on Create, and on Update only when the server has no
// password already stored (an empty dbPassword on Update otherwise means
// "keep the current one", RN-BACKUP-019).
func (req upsertServerRequest) validate(hasExistingPassword bool) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(req.Host) == "" {
		return errors.New("host is required")
	}
	if strings.TrimSpace(req.SSHUser) == "" {
		return errors.New("sshUser is required")
	}

	switch domain.DBEngine(req.DBEngine) {
	case domain.DBEnginePostgres, domain.DBEngineMySQL, domain.DBEngineSQLServer:
	default:
		return errors.New("dbEngine must be one of postgres, mysql, sqlserver")
	}

	switch domain.DeploymentMode(req.DeploymentMode) {
	case domain.DeploymentModeDocker:
		if strings.TrimSpace(req.ContainerName) == "" {
			return errors.New("containerName is required when deploymentMode=docker")
		}
		if !containerNameRegex.MatchString(req.ContainerName) {
			return errors.New("containerName must start with alphanumeric and contain only alphanumeric, underscore, dot, or hyphen")
		}
	case domain.DeploymentModeHost:
		if strings.TrimSpace(req.ContainerName) != "" {
			return errors.New("containerName must be empty when deploymentMode=host")
		}
	default:
		return errors.New("deploymentMode must be one of docker, host")
	}

	if strings.TrimSpace(req.DBName) == "" {
		return errors.New("dbName is required")
	}
	if !containerNameRegex.MatchString(req.DBName) {
		return errors.New("dbName must start with alphanumeric and contain only alphanumeric, underscore, dot, or hyphen")
	}
	if strings.TrimSpace(req.DBUser) == "" {
		return errors.New("dbUser is required")
	}
	if !containerNameRegex.MatchString(req.DBUser) {
		return errors.New("dbUser must start with alphanumeric and contain only alphanumeric, underscore, dot, or hyphen")
	}
	if req.Port <= 0 || req.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	if req.DBEngine != string(domain.DBEnginePostgres) && !hasExistingPassword && strings.TrimSpace(req.DBPassword) == "" {
		return errors.New("dbPassword is required for mysql/sqlserver")
	}

	return nil
}

func (h *ServerHandlers) List(w http.ResponseWriter, r *http.Request) {
	servers, err := h.Servers.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	dtos := make([]serverDTO, 0, len(servers))
	for _, s := range servers {
		dtos = append(dtos, toServerDTO(s))
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (h *ServerHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toServerDTO(s))
}

func (h *ServerHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req upsertServerRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Port == 0 {
		req.Port = 22
	}
	if req.CronExpression == "" {
		req.CronExpression = "0 3 * * *"
	}
	if req.DBEngine == "" {
		req.DBEngine = string(domain.DBEnginePostgres)
	}
	if req.DeploymentMode == "" {
		req.DeploymentMode = string(domain.DeploymentModeDocker)
	}
	if err := req.validate(false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var containerName *string
	if req.DeploymentMode == string(domain.DeploymentModeDocker) {
		containerName = &req.ContainerName
	}

	// The ID is generated here, before INSERT, rather than left to the
	// column's DB-side default: the servers_password_required_by_engine
	// CHECK constraint rejects a non-postgres row with a NULL password at
	// insert time, so the password must be encrypted (which needs the ID as
	// AAD) and included in the same INSERT — a later SetDBPassword UPDATE
	// would be too late for mysql/sqlserver.
	id := uuid.New().String()
	var encryptedPassword []byte
	if strings.TrimSpace(req.DBPassword) != "" {
		var err error
		encryptedPassword, err = h.Sealer.Encrypt(id, domain.DBPasswordAAD, req.DBPassword)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt DB password")
			return
		}
	}

	created, err := h.Servers.Create(r.Context(), domain.Server{
		ID: id, Name: req.Name, Host: req.Host, Port: req.Port, SSHUser: req.SSHUser,
		DBEngine: domain.DBEngine(req.DBEngine), DeploymentMode: domain.DeploymentMode(req.DeploymentMode),
		ContainerName: containerName, DBName: req.DBName, DBUser: req.DBUser,
		DBPasswordEncrypted: encryptedPassword,
		PgDumpExtraArgs:     req.PgDumpExtraArgs, MySQLDumpExtraArgs: req.MySQLDumpExtraArgs,
		SqlCmdExtraArgs: req.SqlCmdExtraArgs, StorageTargetID: req.StorageTargetID,
		CronExpression: req.CronExpression,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Generate SSH key pair
	privateKeyPEM, publicKeyOpenSSH, fingerprint, err := sshkeys.GenerateKeyPair()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate SSH key pair")
		return
	}

	// Encrypt private key
	encryptedPrivateKey, err := h.Sealer.Encrypt(created.ID, "ssh_private_key_encrypted", string(privateKeyPEM))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt SSH private key")
		return
	}

	// Store encrypted key and public key in database
	publicKeyStr := string(publicKeyOpenSSH)
	if err := h.Servers.SetSSHKeyPair(r.Context(), created.ID, encryptedPrivateKey, publicKeyStr, fingerprint); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store SSH key pair")
		return
	}

	// Re-fetch to get updated fields
	created, err = h.Servers.Get(r.Context(), created.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve created server")
		return
	}

	writeJSON(w, http.StatusCreated, toServerDTO(created))
}

// recomputeNextRunAtOnCronChange returns the next_run_at to persist after a
// PUT /api/servers/{id} that changes cronExpression, or nil if no recompute
// is needed. It only acts on servers that were already scheduled at least
// once (currentNextRunAt non-nil) — a server that never reached ready still
// relies solely on bootstrapNextRunAt (RN-BACKUP-029) to populate this field
// for the first time. Mirrors bootstrapNextRunAt's degrade-gracefully
// behavior on an invalid cron: the caller logs a warning and leaves
// next_run_at untouched instead of failing the request.
func recomputeNextRunAtOnCronChange(oldCron, newCron string, currentNextRunAt *time.Time, now time.Time) (*time.Time, error) {
	if newCron == "" || newCron == oldCron || currentNextRunAt == nil {
		return nil, nil
	}
	next, err := scheduler.NextRunTime(newCron, now)
	if err != nil {
		return nil, err
	}
	return &next, nil
}

func (h *ServerHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var req upsertServerRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Port == 0 {
		req.Port = 22
	}
	if err := req.validate(existing.DBPasswordEncrypted != nil); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var containerName *string
	if req.DeploymentMode == string(domain.DeploymentModeDocker) {
		containerName = &req.ContainerName
	}

	existing.Name, existing.Host, existing.Port = req.Name, req.Host, req.Port
	existing.SSHUser, existing.ContainerName = req.SSHUser, containerName
	existing.DBEngine, existing.DeploymentMode = domain.DBEngine(req.DBEngine), domain.DeploymentMode(req.DeploymentMode)
	existing.DBName, existing.DBUser = req.DBName, req.DBUser
	existing.PgDumpExtraArgs = req.PgDumpExtraArgs
	existing.MySQLDumpExtraArgs, existing.SqlCmdExtraArgs = req.MySQLDumpExtraArgs, req.SqlCmdExtraArgs
	existing.StorageTargetID = req.StorageTargetID
	oldCron := existing.CronExpression
	if req.CronExpression != "" {
		existing.CronExpression = req.CronExpression
	}

	if err := h.Servers.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// RN-BACKUP-030: a cron change on a server that's already scheduled must
	// take effect immediately, not wait for the stale next_run_at to fire.
	if newNextRunAt, err := recomputeNextRunAtOnCronChange(oldCron, req.CronExpression, existing.NextRunAt, time.Now()); err != nil {
		if h.Logger != nil {
			h.Logger.WarnContext(r.Context(), "could not recompute next_run_at after cron change: invalid cron expression",
				slog.String("serverId", existing.ID), slog.String("error", err.Error()))
		}
	} else if newNextRunAt != nil {
		if err := h.Servers.RescheduleNextRunAt(r.Context(), existing.ID, *newNextRunAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	// RN-BACKUP-019: only re-encrypt/store when the operator actually
	// submitted a new password — an empty dbPassword means "keep the
	// current one".
	if strings.TrimSpace(req.DBPassword) != "" {
		encryptedPassword, err := h.Sealer.Encrypt(existing.ID, domain.DBPasswordAAD, req.DBPassword)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt DB password")
			return
		}
		if err := h.Servers.SetDBPassword(r.Context(), existing.ID, encryptedPassword); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store DB password")
			return
		}
	}

	updated, err := h.Servers.Get(r.Context(), existing.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated server")
		return
	}
	writeJSON(w, http.StatusOK, toServerDTO(updated))
}

func (h *ServerHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Servers.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
