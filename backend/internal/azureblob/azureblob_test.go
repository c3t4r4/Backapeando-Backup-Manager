package azureblob

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

// This file closes the Fase 3 gap flagged by the Validator: azureblob was
// previously untestable against the Azurite emulator because Target hard-
// coded the production service URL format (https://<account>.blob.core.windows.net).
// Target.ServiceURLOverride (see azureblob.go) removes that blocker — this
// test points the package's own newClient/UploadStream/ListBlobs/DeleteBlob
// at a real, disposable Azurite container.
//
// SAS generation approach: Azurite ships with a well-known, publicly
// documented development account ("devstoreaccount1") and account key
// (documented by Microsoft at
// https://learn.microsoft.com/azure/storage/common/storage-use-azurite#well-known-storage-account-and-key).
// This is not a production secret; it is a fixed constant baked into the
// emulator itself and safe to hardcode in test code. This test uses that
// key, via azblob.NewSharedKeyCredential, ONLY to (a) create the test
// container and (b) sign a real account SAS token locally with the SDK's
// own sas.AccountSignatureValues.SignWithSharedKey — i.e. we generate the
// same kind of SAS token production would receive from an Azure portal/CLI,
// then hand that SAS string to the package under test exactly as
// production does (Target.SASToken). The package's own newClient/
// UploadStream/ListBlobs/DeleteBlob functions never see or use the shared
// key directly — only the SAS token they are designed to consume. The
// shared-key client declared below exists solely to set up the container
// before the real test begins, mirroring how a human operator would have
// already created the container out of band in production.
const (
	azuriteDevAccountName = "devstoreaccount1"
	// Well-known Azurite development account key, publicly documented by
	// Microsoft; not a secret, see comment above.
	azuriteDevAccountKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

// startAzurite launches a disposable Azurite blob-service container bound
// to a random free host port, waits for it to accept TCP connections, and
// registers cleanup. It skips the test (not fails it) if Docker is
// unavailable, so this suite never blocks environments without Docker.
func startAzurite(t *testing.T) (serviceURL string) {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH; skipping azureblob/Azurite integration test")
	}

	// Use port 0 so Docker assigns a free host port, avoiding collisions
	// with anything else already listening on the fixed Azurite default
	// (10000) on the host running this test.
	// --skipApiVersionCheck: the azure-sdk-for-go client sends an
	// x-ms-version header for a very recent Storage API version (this repo
	// pins azblob v1.8.1); the "latest"-tagged Azurite image observed in
	// development lags behind and otherwise rejects every request with
	// "InvalidHeaderValue: The API version ... is not supported by
	// Azurite", exactly as Azurite's own error message instructs callers to
	// work around via this flag.
	runCmd := exec.Command("docker", "run", "-d", "--rm",
		"-p", "127.0.0.1::10000",
		"mcr.microsoft.com/azure-storage/azurite",
		"azurite-blob", "--blobHost", "0.0.0.0", "--skipApiVersionCheck",
	)
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Skipf("could not start Azurite container (docker run failed): %v\noutput: %s", err, out)
	}
	containerID := strings.TrimSpace(string(out))
	if containerID == "" {
		t.Skip("docker run produced no container id; skipping")
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "stop", containerID).Run()
	})

	// Note: "docker port <id> 10000/tcp" (filtering to a single port) returns
	// "page not found" against at least one Docker context observed in
	// development (OrbStack's Docker API proxy) even though the mapping
	// exists and "docker port <id>" (no filter) / "docker inspect" both
	// report it correctly. Use "docker inspect" with a Go template instead,
	// which is also less dependent on the CLI's human-readable text format.
	inspectOut, err := exec.Command("docker", "inspect", "-f",
		`{{(index (index .NetworkSettings.Ports "10000/tcp") 0).HostPort}}`,
		containerID,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("docker inspect failed: %v\noutput: %s", err, inspectOut)
	}
	port := strings.TrimSpace(string(inspectOut))
	if port == "" {
		t.Fatalf("docker inspect returned an empty host port; output: %s", inspectOut)
	}

	serviceURL = fmt.Sprintf("http://127.0.0.1:%s/%s", port, azuriteDevAccountName)

	// Poll until Azurite accepts TCP connections (image needs a moment to
	// start listening after "docker run -d" returns).
	deadline := time.Now().Add(30 * time.Second)
	for {
		conn, dialErr := net.DialTimeout("tcp", "127.0.0.1:"+port, 500*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Azurite did not start listening on port %s within 30s: %v", port, dialErr)
		}
		time.Sleep(300 * time.Millisecond)
	}

	return serviceURL
}

// generateContainerSAS signs a real service-level (container-scoped) SAS
// query string against the Azurite dev account/key, granting
// read/add/create/write/delete/list access to a single container —
// equivalent in shape to what a production azure-type storage_targets row
// stores (a scoped SAS token, see docs/RegrasNegocio.md), just generated
// locally instead of via the Azure portal/CLI.
//
// Two concrete difficulties were hit and resolved while writing this test
// (documented here and in docs/Memoria.md, not silently worked around):
//
//  1. An earlier version of this test used an account-level SAS
//     (sas.AccountSignatureValues), which the SDK happily signs, but
//     Azurite rejected every request signed that way with 403
//     AuthorizationFailure. A container-scoped service SAS
//     (sas.BlobSignatureValues with BlobName left empty) is the far more
//     common credential shape in practice, and Azurite validates it
//     correctly.
//  2. Even the container-scoped SAS above initially failed with the same
//     403 AuthorizationFailure. Root cause: sas.BlobSignatureValues'
//     ExpiryTime/StartTime formatter (sas.formatTime) does not convert to
//     UTC — it calls time.Time.Format with a layout ending in a literal
//     "Z", so whatever wall-clock time is in the time.Time value is
//     stamped with a "Z" suffix as if it were already UTC. On a host whose
//     local time zone is not UTC (this development machine is UTC-3), an
//     unconverted time.Now().Add(1*time.Hour) produces a signed "se" value
//     that is *behind* the true current UTC time by the zone offset — an
//     already-expired SAS from the server's point of view, which Azure/
//     Azurite both report as the same generic 403 AuthorizationFailure as
//     a bad signature. Fix: always call .UTC() before handing a time.Time
//     to sas.BlobSignatureValues (or sas.AccountSignatureValues).
func generateContainerSAS(t *testing.T, containerName string) string {
	t.Helper()

	cred, err := azblob.NewSharedKeyCredential(azuriteDevAccountName, azuriteDevAccountKey)
	if err != nil {
		t.Fatalf("NewSharedKeyCredential: %v", err)
	}

	permissions := sas.ContainerPermissions{Read: true, Add: true, Create: true, Write: true, Delete: true, List: true}

	values := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPSandHTTP,
		ExpiryTime:    time.Now().UTC().Add(1 * time.Hour),
		Permissions:   permissions.String(),
		ContainerName: containerName,
		// BlobName left empty: this scopes the SAS to the whole container
		// (resource="c"), matching what UploadStream/ListBlobs/DeleteBlob
		// need (they operate on arbitrary blob names within one container).
	}

	qp, err := values.SignWithSharedKey(cred)
	if err != nil {
		t.Fatalf("SignWithSharedKey: %v", err)
	}
	return qp.Encode()
}

// createTestContainer creates the container the test will upload into,
// using the shared-key client directly (test setup only — never the
// package under test, which is SAS-only by design).
func createTestContainer(t *testing.T, serviceURL, containerName string) {
	t.Helper()

	cred, err := azblob.NewSharedKeyCredential(azuriteDevAccountName, azuriteDevAccountKey)
	if err != nil {
		t.Fatalf("NewSharedKeyCredential: %v", err)
	}
	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		t.Fatalf("NewClientWithSharedKeyCredential: %v", err)
	}
	if _, err := client.CreateContainer(context.Background(), containerName, nil); err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
}

func TestAzureblob_UploadListDelete_AgainstAzurite(t *testing.T) {
	serviceURL := startAzurite(t)

	const containerName = "backup-test-container"
	createTestContainer(t, serviceURL, containerName)

	sasToken := generateContainerSAS(t, containerName)

	target := Target{
		AccountName:        azuriteDevAccountName,
		ContainerName:      containerName,
		SASToken:           sasToken,
		ServiceURLOverride: serviceURL,
	}

	ctx := context.Background()
	const blobName = "some-server/some_db_20260914T120000Z.dump"
	content := []byte("fake pg_dump payload for integration test")

	// Upload
	size, err := UploadStream(ctx, target, blobName, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("UploadStream: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("UploadStream returned size %d, want %d", size, len(content))
	}

	// List: the uploaded blob must be present with the right name and size.
	blobs, err := ListBlobs(ctx, target, "some-server/")
	if err != nil {
		t.Fatalf("ListBlobs: %v", err)
	}
	found := false
	for _, b := range blobs {
		if b.Name == blobName {
			found = true
			if b.SizeBytes != int64(len(content)) {
				t.Errorf("listed blob size = %d, want %d", b.SizeBytes, len(content))
			}
		}
	}
	if !found {
		t.Fatalf("uploaded blob %q not found in ListBlobs result: %+v", blobName, blobs)
	}

	// Delete
	if err := DeleteBlob(ctx, target, blobName); err != nil {
		t.Fatalf("DeleteBlob: %v", err)
	}

	// Confirm removal via a subsequent list.
	blobsAfterDelete, err := ListBlobs(ctx, target, "some-server/")
	if err != nil {
		t.Fatalf("ListBlobs after delete: %v", err)
	}
	for _, b := range blobsAfterDelete {
		if b.Name == blobName {
			t.Fatalf("blob %q still present after DeleteBlob", blobName)
		}
	}
}

func TestAzureblob_CheckAccess_AgainstAzurite(t *testing.T) {
	serviceURL := startAzurite(t)

	const containerName = "backup-test-checkaccess"
	createTestContainer(t, serviceURL, containerName)
	sasToken := generateContainerSAS(t, containerName)

	valid := Target{
		AccountName:        azuriteDevAccountName,
		ContainerName:      containerName,
		SASToken:           sasToken,
		ServiceURLOverride: serviceURL,
	}
	if err := CheckAccess(context.Background(), valid); err != nil {
		t.Errorf("CheckAccess with valid target: expected nil error, got %v", err)
	}

	nonExistentContainer := Target{
		AccountName:        azuriteDevAccountName,
		ContainerName:      "container-that-does-not-exist",
		SASToken:           sasToken,
		ServiceURLOverride: serviceURL,
	}
	if err := CheckAccess(context.Background(), nonExistentContainer); err == nil {
		t.Error("CheckAccess against a non-existent container: expected an error, got nil")
	}
}
