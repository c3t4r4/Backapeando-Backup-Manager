package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
	"backapeando-backup-manager/internal/storage"

	"github.com/google/uuid"
)

// StorageTargetHandlers implements CRUD over storage_targets (the backup
// destination configured per server): Azure Blob Storage, an S3-compatible
// object store, or a local/NFS-mounted filesystem directory, discriminated
// by Type. The AAD literals used to encrypt each type's secret
// (storage.AzureSASTokenAAD, storage.S3SecretAccessKeyAAD) are owned by
// internal/storage — this is the only other place that references them
// (the encrypt side; internal/storage.NewBackend is the decrypt side), so
// there is exactly one literal per secret, never two (see that package's
// dispatch.go doc comment for why this matters).
type StorageTargetHandlers struct {
	Targets *repository.StorageTargetRepo
	Sealer  *crypto.Sealer
}

// storageTargetDTO omits every secret (SAS token, S3 secret access key)
// entirely — once written, a secret is never read back through the API,
// only re-set via Update. Only the fields relevant to Type are populated.
type storageTargetDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`

	AccountName       *string    `json:"accountName,omitempty"`
	ContainerName     *string    `json:"containerName,omitempty"`
	SASTokenExpiresAt *time.Time `json:"sasTokenExpiresAt,omitempty"`

	Endpoint     *string `json:"endpoint,omitempty"`
	Region       *string `json:"region,omitempty"`
	Bucket       *string `json:"bucket,omitempty"`
	AccessKeyID  *string `json:"accessKeyId,omitempty"`
	UsePathStyle bool    `json:"usePathStyle,omitempty"`

	RootPath *string `json:"rootPath,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toStorageTargetDTO(t domain.StorageTarget) storageTargetDTO {
	return storageTargetDTO{
		ID: t.ID, Name: t.Name, Type: string(t.Type),
		AccountName: t.AzureAccountName, ContainerName: t.AzureContainerName, SASTokenExpiresAt: t.AzureSASTokenExpiresAt,
		Endpoint: t.S3Endpoint, Region: t.S3Region, Bucket: t.S3Bucket, AccessKeyID: t.S3AccessKeyID, UsePathStyle: t.S3UsePathStyle,
		RootPath:  t.FSRootPath,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// upsertStorageTargetRequest carries every backend type's fields at once;
// only the ones relevant to Type are read/validated (see validate below).
type upsertStorageTargetRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`

	// Azure fields.
	AccountName       string     `json:"accountName"`
	ContainerName     string     `json:"containerName"`
	SASToken          string     `json:"sasToken"`
	SASTokenExpiresAt *time.Time `json:"sasTokenExpiresAt"`

	// S3 fields.
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	UsePathStyle    bool   `json:"usePathStyle"`

	// Filesystem fields.
	RootPath string `json:"rootPath"`
}

// validate checks the fields required for req.Type. requireSecret is true
// on Create (the secret must be supplied) and false on Update (an empty
// secret means "keep the current one" — the SPA only sends a new value
// when the operator is explicitly rotating it), matching the pre-existing
// Azure-target UX pattern.
func (req upsertStorageTargetRequest) validate(requireSecret bool) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}

	switch domain.StorageTargetType(req.Type) {
	case domain.StorageTargetTypeAzure:
		if strings.TrimSpace(req.AccountName) == "" {
			return errors.New("accountName is required")
		}
		if strings.TrimSpace(req.ContainerName) == "" {
			return errors.New("containerName is required")
		}
		if requireSecret && strings.TrimSpace(req.SASToken) == "" {
			return errors.New("sasToken is required")
		}
	case domain.StorageTargetTypeS3:
		if strings.TrimSpace(req.Bucket) == "" {
			return errors.New("bucket is required")
		}
		if strings.TrimSpace(req.AccessKeyID) == "" {
			return errors.New("accessKeyId is required")
		}
		if requireSecret && strings.TrimSpace(req.SecretAccessKey) == "" {
			return errors.New("secretAccessKey is required")
		}
	case domain.StorageTargetTypeFilesystem:
		if strings.TrimSpace(req.RootPath) == "" {
			return errors.New("rootPath is required")
		}
	default:
		return errors.New(`type must be one of "azure", "s3", "filesystem"`)
	}
	return nil
}

// normalizeSASToken strips a leading '?' — SAS tokens are pasted with or
// without it depending on where the user copied them from (Azure Portal vs
// a generated URL) — normalize instead of rejecting.
func normalizeSASToken(token string) string {
	return strings.TrimPrefix(strings.TrimSpace(token), "?")
}

func optionalString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func (h *StorageTargetHandlers) List(w http.ResponseWriter, r *http.Request) {
	targets, err := h.Targets.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	dtos := make([]storageTargetDTO, 0, len(targets))
	for _, t := range targets {
		dtos = append(dtos, toStorageTargetDTO(t))
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (h *StorageTargetHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := h.Targets.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "storage target not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toStorageTargetDTO(t))
}

func (h *StorageTargetHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req upsertStorageTargetRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// The row's id doesn't exist yet at encryption time, so a fresh random
	// id is used as associated data and then reused as the actual primary
	// key — keeping the AEAD binding consistent from creation (see
	// internal/storage.NewBackend, the decrypt side).
	id := uuid.NewString()
	target := domain.StorageTarget{ID: id, Name: req.Name, Type: domain.StorageTargetType(req.Type)}

	switch target.Type {
	case domain.StorageTargetTypeAzure:
		target.AzureAccountName = &req.AccountName
		target.AzureContainerName = &req.ContainerName
		target.AzureSASTokenExpiresAt = req.SASTokenExpiresAt
		encrypted, err := h.Sealer.Encrypt(id, storage.AzureSASTokenAAD, normalizeSASToken(req.SASToken))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		target.AzureSASTokenEncrypted = encrypted

	case domain.StorageTargetTypeS3:
		target.S3Endpoint = optionalString(req.Endpoint)
		target.S3Region = optionalString(req.Region)
		target.S3Bucket = &req.Bucket
		target.S3AccessKeyID = &req.AccessKeyID
		target.S3UsePathStyle = req.UsePathStyle
		encrypted, err := h.Sealer.Encrypt(id, storage.S3SecretAccessKeyAAD, req.SecretAccessKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		target.S3SecretAccessKeyEncrypted = encrypted

	case domain.StorageTargetTypeFilesystem:
		target.FSRootPath = &req.RootPath
	}

	created, err := h.Targets.Create(r.Context(), target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, toStorageTargetDTO(created))
}

func (h *StorageTargetHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := h.Targets.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "storage target not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var req upsertStorageTargetRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Secrets are optional on update ONLY when the type is not changing —
	// an empty value then means "keep the current one" (the SPA only sends
	// a new value when the operator is explicitly rotating it). If the
	// operator is switching a target's type, there is no existing secret of
	// the new type to keep, so a fresh one is required, same as Create.
	typeChanging := domain.StorageTargetType(req.Type) != existing.Type
	if err := req.validate(typeChanging); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	previousAzureSecret, previousAzureExpiresAt := existing.AzureSASTokenEncrypted, existing.AzureSASTokenExpiresAt
	previousS3Secret := existing.S3SecretAccessKeyEncrypted

	existing.Name = req.Name
	existing.Type = domain.StorageTargetType(req.Type)

	// Clear every type-specific field before repopulating just the ones
	// relevant to the (possibly new) type below — otherwise switching a
	// target's type would leave the previous type's fields (including any
	// encrypted secret) stale in the database indefinitely. Secrets for the
	// *unchanged* type are restored from previousAzureSecret/previousS3Secret
	// below when the operator didn't supply a new one.
	existing.AzureAccountName, existing.AzureContainerName = nil, nil
	existing.AzureSASTokenEncrypted, existing.AzureSASTokenExpiresAt = nil, nil
	existing.S3Endpoint, existing.S3Region, existing.S3Bucket, existing.S3AccessKeyID = nil, nil, nil, nil
	existing.S3SecretAccessKeyEncrypted, existing.S3UsePathStyle = nil, false
	existing.FSRootPath = nil

	switch existing.Type {
	case domain.StorageTargetTypeAzure:
		existing.AzureAccountName = &req.AccountName
		existing.AzureContainerName = &req.ContainerName
		existing.AzureSASTokenExpiresAt = req.SASTokenExpiresAt
		if strings.TrimSpace(req.SASToken) != "" {
			encrypted, err := h.Sealer.Encrypt(existing.ID, storage.AzureSASTokenAAD, normalizeSASToken(req.SASToken))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			existing.AzureSASTokenEncrypted = encrypted
		} else if !typeChanging {
			// No new token supplied and the type didn't change: keep the
			// previously stored ciphertext (still valid — same ID, same AAD).
			existing.AzureSASTokenEncrypted = previousAzureSecret
			existing.AzureSASTokenExpiresAt = previousAzureExpiresAt
		}

	case domain.StorageTargetTypeS3:
		existing.S3Endpoint = optionalString(req.Endpoint)
		existing.S3Region = optionalString(req.Region)
		existing.S3Bucket = &req.Bucket
		existing.S3AccessKeyID = &req.AccessKeyID
		existing.S3UsePathStyle = req.UsePathStyle
		if strings.TrimSpace(req.SecretAccessKey) != "" {
			encrypted, err := h.Sealer.Encrypt(existing.ID, storage.S3SecretAccessKeyAAD, req.SecretAccessKey)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			existing.S3SecretAccessKeyEncrypted = encrypted
		} else if !typeChanging {
			existing.S3SecretAccessKeyEncrypted = previousS3Secret
		}

	case domain.StorageTargetTypeFilesystem:
		existing.FSRootPath = &req.RootPath
	}

	if err := h.Targets.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toStorageTargetDTO(existing))
}

func (h *StorageTargetHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Targets.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
