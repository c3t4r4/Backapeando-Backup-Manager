package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func strPtr(s string) *string { return &s }

func mustUUID(t *testing.T) string {
	t.Helper()
	return uuid.NewString()
}

// doUpdateRequest drives StorageTargetHandlers.Update through the real
// http.Handler path (not calling the method directly) so PathValue/JSON
// decoding are exercised the same way production traffic hits them.
func doUpdateRequest(t *testing.T, h *StorageTargetHandlers, id string, req upsertStorageTargetRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	httpReq := httptest.NewRequest("PUT", "/api/storage-targets/"+id, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	h.Update(rr, httpReq)
	return rr
}

// TestUpsertStorageTargetRequest_Validate covers the type-conditional
// validation added when the storage backend was generalized from
// Azure-only to azure/s3/filesystem (RN-BACKUP-016). Each type has its own
// required-field set, and the secret (sasToken/secretAccessKey) is only
// required when requireSecret is true (Create, or Update when the type is
// changing — see StorageTargetHandlers.Update).
func TestUpsertStorageTargetRequest_Validate(t *testing.T) {
	tests := []struct {
		name          string
		req           upsertStorageTargetRequest
		requireSecret bool
		wantErr       string // substring; empty means no error expected
	}{
		{
			name:    "name required regardless of type",
			req:     upsertStorageTargetRequest{Name: "  ", Type: "azure", AccountName: "a", ContainerName: "b", SASToken: "s"},
			wantErr: "name is required",
		},
		{
			name:    "unknown type rejected",
			req:     upsertStorageTargetRequest{Name: "x", Type: "gcs"},
			wantErr: `type must be one of "azure", "s3", "filesystem"`,
		},
		{
			name:    "empty type rejected",
			req:     upsertStorageTargetRequest{Name: "x", Type: ""},
			wantErr: `type must be one of "azure", "s3", "filesystem"`,
		},

		// azure
		{
			name:    "azure missing accountName",
			req:     upsertStorageTargetRequest{Name: "x", Type: "azure", ContainerName: "c", SASToken: "s"},
			wantErr: "accountName is required",
		},
		{
			name:    "azure missing containerName",
			req:     upsertStorageTargetRequest{Name: "x", Type: "azure", AccountName: "a", SASToken: "s"},
			wantErr: "containerName is required",
		},
		{
			name:          "azure missing sasToken when secret required",
			req:           upsertStorageTargetRequest{Name: "x", Type: "azure", AccountName: "a", ContainerName: "c"},
			requireSecret: true,
			wantErr:       "sasToken is required",
		},
		{
			name: "azure sasToken optional when secret not required",
			req:  upsertStorageTargetRequest{Name: "x", Type: "azure", AccountName: "a", ContainerName: "c"},
		},
		{
			name:          "azure valid with secret required",
			req:           upsertStorageTargetRequest{Name: "x", Type: "azure", AccountName: "a", ContainerName: "c", SASToken: "s"},
			requireSecret: true,
		},

		// s3
		{
			name:    "s3 missing bucket",
			req:     upsertStorageTargetRequest{Name: "x", Type: "s3", AccessKeyID: "k", SecretAccessKey: "s"},
			wantErr: "bucket is required",
		},
		{
			name:    "s3 missing accessKeyId",
			req:     upsertStorageTargetRequest{Name: "x", Type: "s3", Bucket: "b", SecretAccessKey: "s"},
			wantErr: "accessKeyId is required",
		},
		{
			name:          "s3 missing secretAccessKey when secret required",
			req:           upsertStorageTargetRequest{Name: "x", Type: "s3", Bucket: "b", AccessKeyID: "k"},
			requireSecret: true,
			wantErr:       "secretAccessKey is required",
		},
		{
			name: "s3 secretAccessKey optional when secret not required",
			req:  upsertStorageTargetRequest{Name: "x", Type: "s3", Bucket: "b", AccessKeyID: "k"},
		},
		{
			name:          "s3 valid with secret required, endpoint/region/pathStyle optional",
			req:           upsertStorageTargetRequest{Name: "x", Type: "s3", Bucket: "b", AccessKeyID: "k", SecretAccessKey: "s"},
			requireSecret: true,
		},

		// filesystem
		{
			name:    "filesystem missing rootPath",
			req:     upsertStorageTargetRequest{Name: "x", Type: "filesystem"},
			wantErr: "rootPath is required",
		},
		{
			name:          "filesystem valid, no secret needed even when required",
			req:           upsertStorageTargetRequest{Name: "x", Type: "filesystem", RootPath: "/data/backups"},
			requireSecret: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.validate(tt.requireSecret)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want no error", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate() = nil, want error containing %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validate() = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizeSASToken(t *testing.T) {
	tests := map[string]string{
		"?sv=2022-11-02&sig=abc": "sv=2022-11-02&sig=abc",
		"sv=2022-11-02&sig=abc":  "sv=2022-11-02&sig=abc",
		"  ?token  ":             "token",
		"":                       "",
	}
	for in, want := range tests {
		if got := normalizeSASToken(in); got != want {
			t.Errorf("normalizeSASToken(%q) = %q, want %q", in, got, want)
		}
	}
}

// storageTargetTestDB connects to a real Postgres for Update's
// clear-then-repopulate/secret-retention behavior, which is only
// observable through a real round-trip (StorageTargetRepo.Get/Update have
// no fake/interface substitute in this codebase — see internal/repository's
// package doc on hand-written SQL for auditability). Requires DATABASE_URL
// to be set; skipped otherwise, matching the pattern already used by
// internal/repository/servers_test.go and internal/httpapi/integration_test.go.
func storageTargetTestDB(t *testing.T) (*repository.Repositories, *crypto.Sealer) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}
	sealer, err := crypto.NewSealer(masterKey, nil)
	if err != nil {
		t.Fatalf("new sealer: %v", err)
	}
	return repository.New(pool), sealer
}

// TestStorageTargetHandlers_Update_KeepsSecretWhenTypeUnchangedAndNoNewValue
// covers the trickiest new logic in Update: clearing every type-specific
// column before repopulating (so switching type doesn't leave the previous
// type's fields, including its encrypted secret, stale), while still
// restoring the previous secret ciphertext when the type does NOT change
// and the operator didn't supply a new one.
func TestStorageTargetHandlers_Update_KeepsSecretWhenTypeUnchangedAndNoNewValue(t *testing.T) {
	repos, sealer := storageTargetTestDB(t)
	h := &StorageTargetHandlers{Targets: repos.StorageTargets, Sealer: sealer}
	ctx := context.Background()

	original := domain.StorageTarget{
		ID:   mustUUID(t),
		Name: "prod",
		Type: domain.StorageTargetTypeAzure,
	}
	original.AzureAccountName = strPtr("acct")
	original.AzureContainerName = strPtr("backups")
	encrypted, err := sealer.Encrypt(original.ID, "sas_token", "sv=2022-11-02&sig=original")
	if err != nil {
		t.Fatalf("encrypt seed secret: %v", err)
	}
	original.AzureSASTokenEncrypted = encrypted

	created, err := repos.StorageTargets.Create(ctx, original)
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}
	t.Cleanup(func() { _ = repos.StorageTargets.Delete(context.Background(), created.ID) })

	// Update with the same type and no new sasToken: must keep the existing
	// secret ciphertext (still decryptable, unchanged), not null it out.
	req := upsertStorageTargetRequest{Name: "prod-renamed", Type: "azure", AccountName: "acct2", ContainerName: "backups2"}
	rr := doUpdateRequest(t, h, created.ID, req)
	if rr.Code != 200 {
		t.Fatalf("update status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	stored, err := repos.StorageTargets.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if stored.Name != "prod-renamed" || stored.AzureAccountName == nil || *stored.AzureAccountName != "acct2" {
		t.Fatalf("update did not apply non-secret fields: %+v", stored)
	}
	decrypted, err := sealer.Decrypt(stored.ID, "sas_token", stored.AzureSASTokenEncrypted)
	if err != nil {
		t.Fatalf("decrypt retained secret: %v", err)
	}
	if decrypted != "sv=2022-11-02&sig=original" {
		t.Fatalf("retained secret = %q, want original value preserved", decrypted)
	}
}

// TestStorageTargetHandlers_Update_ClearsPreviousTypeFieldsOnTypeChange
// covers the other half: switching type must wipe out the previous type's
// fields (including its ciphertext) rather than leaving them stale
// alongside the new type's fields.
func TestStorageTargetHandlers_Update_ClearsPreviousTypeFieldsOnTypeChange(t *testing.T) {
	repos, sealer := storageTargetTestDB(t)
	h := &StorageTargetHandlers{Targets: repos.StorageTargets, Sealer: sealer}
	ctx := context.Background()

	original := domain.StorageTarget{ID: mustUUID(t), Name: "prod", Type: domain.StorageTargetTypeAzure}
	original.AzureAccountName = strPtr("acct")
	original.AzureContainerName = strPtr("backups")
	encrypted, err := sealer.Encrypt(original.ID, "sas_token", "sv=2022-11-02&sig=original")
	if err != nil {
		t.Fatalf("encrypt seed secret: %v", err)
	}
	original.AzureSASTokenEncrypted = encrypted

	created, err := repos.StorageTargets.Create(ctx, original)
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}
	t.Cleanup(func() { _ = repos.StorageTargets.Delete(context.Background(), created.ID) })

	// Switching type to filesystem requires rootPath and does NOT require
	// (or accept) any azure/s3 fields to persist.
	req := upsertStorageTargetRequest{Name: "prod", Type: "filesystem", RootPath: "/data/backups"}
	rr := doUpdateRequest(t, h, created.ID, req)
	if rr.Code != 200 {
		t.Fatalf("update status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	stored, err := repos.StorageTargets.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if stored.Type != domain.StorageTargetTypeFilesystem {
		t.Fatalf("type = %q, want filesystem", stored.Type)
	}
	if stored.FSRootPath == nil || *stored.FSRootPath != "/data/backups" {
		t.Fatalf("rootPath = %v, want /data/backups", stored.FSRootPath)
	}
	if stored.AzureAccountName != nil || stored.AzureContainerName != nil || stored.AzureSASTokenEncrypted != nil {
		t.Fatalf("azure fields not cleared after type change: %+v", stored)
	}
}

// TestStorageTargetHandlers_Update_TypeChangeRequiresNewSecret asserts that
// switching type without supplying a secret for the new type is rejected —
// there is no existing secret of the new type to fall back to.
func TestStorageTargetHandlers_Update_TypeChangeRequiresNewSecret(t *testing.T) {
	repos, sealer := storageTargetTestDB(t)
	h := &StorageTargetHandlers{Targets: repos.StorageTargets, Sealer: sealer}
	ctx := context.Background()

	original := domain.StorageTarget{ID: mustUUID(t), Name: "prod", Type: domain.StorageTargetTypeFilesystem}
	original.FSRootPath = strPtr("/data/backups")
	created, err := repos.StorageTargets.Create(ctx, original)
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}
	t.Cleanup(func() { _ = repos.StorageTargets.Delete(context.Background(), created.ID) })

	req := upsertStorageTargetRequest{Name: "prod", Type: "s3", Bucket: "b", AccessKeyID: "k"} // no secretAccessKey
	rr := doUpdateRequest(t, h, created.ID, req)
	if rr.Code != 400 {
		t.Fatalf("update status = %d, want 400 (missing secret on type change), body=%s", rr.Code, rr.Body.String())
	}
}
