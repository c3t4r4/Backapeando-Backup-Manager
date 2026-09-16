package storage

import (
	"fmt"

	"backapeando-backup-manager/internal/azureblob"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/fsstorage"
	"backapeando-backup-manager/internal/s3storage"
)

// These two constants are the single source of truth, for the entire
// codebase, of the AAD ("associated data") column-name literal each secret
// was encrypted under (see internal/crypto.Sealer). AES-256-GCM
// authenticates the AAD alongside the ciphertext, so Decrypt fails outright
// (not "wrong value" — a hard crypto failure) if this string differs by even
// one character from whatever string was passed to Encrypt for that same
// row.
//
// This exact class of bug already broke production once: two independent
// constants for the same Azure SAS token AAD column diverged
// ("sas_token_encrypted" in the worker vs "sas_token" at the encrypt site),
// silently breaking decryption for every already-saved token (see
// docs/Memoria.md, 2026-09-15). NewBackend below is now the *only* place in
// the codebase that decrypts a storage secret, and the storage-targets
// handler (encrypt side) imports these same constants — so there is exactly
// one literal per secret, never two.
const (
	// AzureSASTokenAAD must exactly match the AAD used when a StorageTarget's
	// Azure SAS token is encrypted. Both sides — encrypt in
	// httpapi/handlers/storage_targets.go, decrypt in NewBackend below —
	// import this same constant, so there is exactly one literal, never two.
	AzureSASTokenAAD = "sas_token"

	// S3SecretAccessKeyAAD must exactly match the AAD used when a
	// StorageTarget's S3 secret access key is encrypted
	// (httpapi/handlers/storage_targets.go).
	S3SecretAccessKeyAAD = "s3_secret_access_key"
)

// sealer is the minimal Decrypt surface NewBackend needs from
// *crypto.Sealer, kept as an interface so it can be faked in tests without a
// real master key.
type sealer interface {
	Decrypt(recordID, column string, sealed []byte) (string, error)
}

// NewBackend is the single, central place that knows how to turn a
// domain.StorageTarget into a concrete storage.Backend: it switches on
// target.Type, decrypts whichever secret column that type needs (using the
// AAD constants above), and constructs the matching backend package. No
// other code in this project should decrypt a storage secret or import more
// than one of azureblob/s3storage/fsstorage directly.
func NewBackend(target domain.StorageTarget, sealer sealer) (Backend, error) {
	switch target.Type {
	case domain.StorageTargetTypeAzure:
		if target.AzureAccountName == nil || target.AzureContainerName == nil {
			return nil, fmt.Errorf("storage: azure target %q missing account/container name", target.ID)
		}
		sasToken, err := sealer.Decrypt(target.ID, AzureSASTokenAAD, target.AzureSASTokenEncrypted)
		if err != nil {
			return nil, fmt.Errorf("storage: decrypt azure sas token: %w", err)
		}
		return &azureAdapter{target: azureblob.Target{
			AccountName:   *target.AzureAccountName,
			ContainerName: *target.AzureContainerName,
			SASToken:      sasToken,
		}}, nil

	case domain.StorageTargetTypeS3:
		if target.S3Bucket == nil || target.S3AccessKeyID == nil {
			return nil, fmt.Errorf("storage: s3 target %q missing bucket/access key id", target.ID)
		}
		secretKey, err := sealer.Decrypt(target.ID, S3SecretAccessKeyAAD, target.S3SecretAccessKeyEncrypted)
		if err != nil {
			return nil, fmt.Errorf("storage: decrypt s3 secret access key: %w", err)
		}
		endpoint := ""
		if target.S3Endpoint != nil {
			endpoint = *target.S3Endpoint
		}
		region := ""
		if target.S3Region != nil {
			region = *target.S3Region
		}
		s3Backend, err := s3storage.NewBackend(s3storage.Target{
			Endpoint:        endpoint,
			Region:          region,
			Bucket:          *target.S3Bucket,
			AccessKeyID:     *target.S3AccessKeyID,
			SecretAccessKey: secretKey,
			UsePathStyle:    target.S3UsePathStyle,
		})
		if err != nil {
			return nil, fmt.Errorf("storage: build s3 backend: %w", err)
		}
		return &s3Adapter{backend: s3Backend}, nil

	case domain.StorageTargetTypeFilesystem:
		if target.FSRootPath == nil {
			return nil, fmt.Errorf("storage: filesystem target %q missing root path", target.ID)
		}
		return &fsAdapter{backend: fsstorage.NewBackend(fsstorage.Target{RootPath: *target.FSRootPath})}, nil

	default:
		return nil, fmt.Errorf("storage: unknown storage target type %q", target.Type)
	}
}
