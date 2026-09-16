package storage

import (
	"crypto/rand"
	"testing"

	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/domain"
)

func newTestSealer(t *testing.T) *crypto.Sealer {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sealer, err := crypto.NewSealer(key, nil)
	if err != nil {
		t.Fatalf("new sealer: %v", err)
	}
	return sealer
}

// TestNewBackend_AzureResolvesToAzureAdapter confirms an azure-type target
// dispatches to the Azure backend and decrypts its SAS token using
// AzureSASTokenAAD.
func TestNewBackend_AzureResolvesToAzureAdapter(t *testing.T) {
	sealer := newTestSealer(t)
	const id = "target-azure-1"
	const sasToken = "sv=2023-01-01&sig=abc"

	encrypted, err := sealer.Encrypt(id, AzureSASTokenAAD, sasToken)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	accountName, containerName := "acct", "container"
	target := domain.StorageTarget{
		ID:                     id,
		Type:                   domain.StorageTargetTypeAzure,
		AzureAccountName:       &accountName,
		AzureContainerName:     &containerName,
		AzureSASTokenEncrypted: encrypted,
	}

	backend, err := NewBackend(target, sealer)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	if _, ok := backend.(*azureAdapter); !ok {
		t.Fatalf("expected *azureAdapter, got %T", backend)
	}
}

// TestNewBackend_S3ResolvesToS3Adapter confirms an s3-type target dispatches
// to the S3 backend and decrypts its secret access key using
// S3SecretAccessKeyAAD.
func TestNewBackend_S3ResolvesToS3Adapter(t *testing.T) {
	sealer := newTestSealer(t)
	const id = "target-s3-1"
	const secretKey = "super-secret-key"

	encrypted, err := sealer.Encrypt(id, S3SecretAccessKeyAAD, secretKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	bucket, accessKeyID := "my-bucket", "AKIAEXAMPLE"
	target := domain.StorageTarget{
		ID:                         id,
		Type:                       domain.StorageTargetTypeS3,
		S3Bucket:                   &bucket,
		S3AccessKeyID:              &accessKeyID,
		S3SecretAccessKeyEncrypted: encrypted,
	}

	backend, err := NewBackend(target, sealer)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	if _, ok := backend.(*s3Adapter); !ok {
		t.Fatalf("expected *s3Adapter, got %T", backend)
	}
}

// TestNewBackend_FilesystemResolvesToFSAdapter confirms a filesystem-type
// target dispatches to the filesystem backend. Filesystem targets have no
// secret to decrypt, so there is no AAD to verify here.
func TestNewBackend_FilesystemResolvesToFSAdapter(t *testing.T) {
	sealer := newTestSealer(t)
	root := t.TempDir()

	target := domain.StorageTarget{
		ID:         "target-fs-1",
		Type:       domain.StorageTargetTypeFilesystem,
		FSRootPath: &root,
	}

	backend, err := NewBackend(target, sealer)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	if _, ok := backend.(*fsAdapter); !ok {
		t.Fatalf("expected *fsAdapter, got %T", backend)
	}
}

// TestNewBackend_UnknownTypeRejected ensures an unrecognized/empty type
// fails loudly instead of silently defaulting to some backend.
func TestNewBackend_UnknownTypeRejected(t *testing.T) {
	sealer := newTestSealer(t)
	target := domain.StorageTarget{ID: "target-unknown", Type: domain.StorageTargetType("carrier-pigeon")}

	if _, err := NewBackend(target, sealer); err == nil {
		t.Fatal("expected an error for an unknown storage target type, got nil")
	}
}

// TestAzureSASTokenAAD_MatchesEncryptionSide and
// TestS3SecretAccessKeyAAD_MatchesEncryptionSide are regression tests for the
// exact bug class that already broke production once (see docs/Memoria.md,
// 2026-09-15): two independent constants for the same AAD column string
// diverging between the encrypt site (httpapi/handlers/storage_targets.go)
// and the decrypt site (NewBackend above). Both sides now import
// AzureSASTokenAAD/S3SecretAccessKeyAAD from this single package, so these
// tests assert round-tripping through the *named constant* succeeds — if a
// future edit ever reintroduces a second, diverging literal at either call
// site, only this exact constant (not a copy) will authenticate.
func TestAzureSASTokenAAD_MatchesEncryptionSide(t *testing.T) {
	sealer := newTestSealer(t)
	const id = "aad-check-azure"
	const plaintext = "sv=2023-01-01&ss=b&srt=co&sig=abc123"

	encrypted, err := sealer.Encrypt(id, AzureSASTokenAAD, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := sealer.Decrypt(id, AzureSASTokenAAD, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Errorf("decrypted = %q, want %q", got, plaintext)
	}
}

func TestS3SecretAccessKeyAAD_MatchesEncryptionSide(t *testing.T) {
	sealer := newTestSealer(t)
	const id = "aad-check-s3"
	const plaintext = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

	encrypted, err := sealer.Encrypt(id, S3SecretAccessKeyAAD, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := sealer.Decrypt(id, S3SecretAccessKeyAAD, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plaintext {
		t.Errorf("decrypted = %q, want %q", got, plaintext)
	}
}
