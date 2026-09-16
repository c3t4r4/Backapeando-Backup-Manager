// Package azureblob is a thin, SAS-only client over Azure Blob Storage. It
// intentionally does not support account-key or Azure AD credentials: every
// azure-type storage_targets row stores a scoped SAS token (see
// docs/RegrasNegocio.md and internal/crypto), so the client only ever needs
// azblob.NewClientWithNoCredential built from a full service URL that
// already embeds the SAS query string.
//
// Security note: Target.SASToken is decrypted plaintext held only in
// memory. It must never be logged, and it must never be interpolated into
// an error via %v/%+v on the Target struct or on the constructed service
// URL — every error path below wraps SDK-returned errors only, never the
// struct or URL that contains the token.
package azureblob

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// BlobInfo describes a single blob as needed by the retention algorithm and
// the backup-runs API.
type BlobInfo struct {
	Name         string
	LastModified time.Time
	SizeBytes    int64
}

// Target identifies where to upload/list/delete blobs. SASToken is the
// decrypted token (without the leading '?'); it is only ever held in
// memory for the duration of a single request.
type Target struct {
	AccountName   string
	ContainerName string
	SASToken      string

	// ServiceURLOverride, when non-empty, replaces the entire production
	// service URL (https://<account>.blob.core.windows.net) that would
	// otherwise be derived from AccountName. It exists solely to let tests
	// point this package at the Azurite emulator, which uses path-style
	// addressing (http://127.0.0.1:<port>/<account>) instead of the
	// production DNS-style host. In production this field is always empty
	// and behavior is unchanged. Never set from user/request input.
	ServiceURLOverride string
}

func newClient(t Target) (*azblob.Client, error) {
	base := t.ServiceURLOverride
	if base == "" {
		base = fmt.Sprintf("https://%s.blob.core.windows.net", t.AccountName)
	}
	serviceURL := fmt.Sprintf("%s/?%s", base, t.SASToken)
	client, err := azblob.NewClientWithNoCredential(serviceURL, nil)
	if err != nil {
		// Deliberately not wrapping serviceURL or t here: both contain the
		// SAS token. azblob.NewClientWithNoCredential's own error does not
		// echo the URL back, only parse/validation failure detail.
		return nil, fmt.Errorf("azblob.NewClientWithNoCredential: %w", err)
	}
	return client, nil
}

// UploadStream uploads r as a blob named blobName, streaming without
// buffering the whole content in memory (the SDK reads r in blocks and
// commits the block list once the upload completes). Returns the number of
// bytes uploaded, read back via a follow-up GetProperties call since the
// upload response itself does not reliably carry content length across SDK
// versions.
func UploadStream(ctx context.Context, t Target, blobName string, r io.Reader) (int64, error) {
	client, err := newClient(t)
	if err != nil {
		return 0, err
	}

	if _, err := client.UploadStream(ctx, t.ContainerName, blobName, r, nil); err != nil {
		return 0, fmt.Errorf("UploadStream: %w", err)
	}

	props, err := client.ServiceClient().
		NewContainerClient(t.ContainerName).
		NewBlobClient(blobName).
		GetProperties(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("GetProperties after upload: %w", err)
	}
	if props.ContentLength == nil {
		return 0, nil
	}
	return *props.ContentLength, nil
}

// ListBlobs lists all blobs under the given prefix (e.g. a server's slug
// folder, "my-server/").
func ListBlobs(ctx context.Context, t Target, prefix string) ([]BlobInfo, error) {
	client, err := newClient(t)
	if err != nil {
		return nil, err
	}

	var result []BlobInfo
	pager := client.NewListBlobsFlatPager(t.ContainerName, &azblob.ListBlobsFlatOptions{Prefix: &prefix})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("NextPage: %w", err)
		}
		for _, item := range page.Segment.BlobItems {
			if item.Name == nil || item.Properties == nil || item.Properties.LastModified == nil {
				continue
			}
			var size int64
			if item.Properties.ContentLength != nil {
				size = *item.Properties.ContentLength
			}
			result = append(result, BlobInfo{
				Name:         *item.Name,
				LastModified: *item.Properties.LastModified,
				SizeBytes:    size,
			})
		}
	}
	return result, nil
}

// CheckAccess validates that the SAS token/account/container in t are
// usable without uploading or deleting anything: it lists at most one blob
// and discards the result. Used by the server test-connection flow to
// verify an Azure target's credentials are actually usable.
func CheckAccess(ctx context.Context, t Target) error {
	client, err := newClient(t)
	if err != nil {
		return err
	}

	maxResults := int32(1)
	pager := client.NewListBlobsFlatPager(t.ContainerName, &azblob.ListBlobsFlatOptions{MaxResults: &maxResults})
	if pager.More() {
		if _, err := pager.NextPage(ctx); err != nil {
			return fmt.Errorf("NextPage: %w", err)
		}
	}
	return nil
}

// DeleteBlob permanently deletes a blob. Irreversible — callers in the
// retention sweep must have already recorded (or be about to record) a
// retention_deletions audit row before/alongside calling this.
func DeleteBlob(ctx context.Context, t Target, blobName string) error {
	client, err := newClient(t)
	if err != nil {
		return err
	}
	if _, err := client.DeleteBlob(ctx, t.ContainerName, blobName, nil); err != nil {
		return fmt.Errorf("DeleteBlob: %w", err)
	}
	return nil
}

// Note on storage.Backend: this package deliberately does NOT provide a
// NewBackend(t Target) storage.Backend adapter here. internal/storage
// defines the Backend interface and its own dispatch.NewBackend (which
// imports this package to construct Azure backends); if this package also
// imported internal/storage to reference storage.Backend/storage.BlobInfo
// in an adapter's method signatures, the two packages would import each
// other — an import cycle. Instead, the thin adapter that satisfies
// storage.Backend for Azure targets lives in internal/storage itself
// (see storage/adapters.go), calling the free functions above and
// converting BlobInfo below to storage.BlobInfo. This package stays exactly
// as it was before storage-backend pluggability was introduced.
