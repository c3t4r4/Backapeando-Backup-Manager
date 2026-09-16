package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
	"backapeando-backup-manager/internal/scheduler"
	"backapeando-backup-manager/internal/sshclient"
	"backapeando-backup-manager/internal/sshkeys"
	"backapeando-backup-manager/internal/storage"
)

// sanitizeSSHTestError reduces an SSH error to a safe message before
// persisting in servers.last_test_connection_error or returning to the client.
// It must never contain authentication details, host key info, or raw SSH library internals.
// The full error is always logged server-side via h.Logger before this function is called,
// so nothing is lost — this is only about what leaves the process boundary.
func sanitizeSSHTestError(err error) string {
	return "SSH connection test failed; see server logs for details"
}

// checkResult is one component of a combined test-connection check.
type checkResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// testConnectionChecks is the per-component breakdown returned alongside
// the server DTO by TestConnection (RN-BACKUP-013). Storage is nil when the
// server has no storageTargetId configured — there is nothing to check.
// DumpTool is the Docker-container-running check in docker mode, or a
// "dump binary is on PATH" check in host mode (there is no container to
// inspect) — renamed from the pre-existing "docker" key now that it covers
// both paths.
type testConnectionChecks struct {
	SSH      checkResult  `json:"ssh"`
	DumpTool checkResult  `json:"dumpTool"`
	Storage  *checkResult `json:"storage,omitempty"`
}

// composeCheckError joins the failing checks into the single human-readable
// string persisted in servers.last_test_connection_error, matching the
// column's pre-existing shape (one string, not structured data).
func composeCheckError(c testConnectionChecks) string {
	var parts []string
	if !c.SSH.OK {
		parts = append(parts, "SSH: "+c.SSH.Error)
	}
	if !c.DumpTool.OK {
		parts = append(parts, "Dump tool: "+c.DumpTool.Error)
	}
	if c.Storage != nil && !c.Storage.OK {
		parts = append(parts, "Storage: "+c.Storage.Error)
	}
	return strings.Join(parts, "; ")
}

// testConnectionResponse extends the plain server DTO with the per-check
// breakdown, so the frontend can render individual pass/fail badges instead
// of a single opaque error string.
type testConnectionResponse struct {
	serverDTO
	Checks testConnectionChecks `json:"checks"`
}

// TestConnection runs a combined readiness check for a server: SSH+TOFU
// connectivity (always), the target Docker container's running state
// (always, requires a live SSH session), and — when the server has a
// storageTargetId configured — that the storage target's credentials are
// actually usable. Implements RN-BACKUP-013: status only advances to ready
// (and RN-BACKUP-005's auto-enable only fires) when every applicable check
// passes.
func (h *ServerHandlers) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	server, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Server must have SSH key material to test
	if server.SSHPrivateKeyEncrypted == nil || server.SSHPublicKey == nil {
		writeError(w, http.StatusBadRequest, "server has no SSH key pair")
		return
	}

	// Decrypt private key
	privateKeyPEMStr, err := h.Sealer.Decrypt(id, "ssh_private_key_encrypted", server.SSHPrivateKeyEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt SSH private key")
		return
	}
	privateKeyPEM := []byte(privateKeyPEMStr)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var checks testConnectionChecks

	host := fmt.Sprintf("%s:%d", server.Host, server.Port)
	opts := sshclient.ConnectOptions{
		User:              server.SSHUser,
		Host:              host,
		PrivateKeyPEM:     privateKeyPEM,
		Timeout:           30 * time.Second,
		ExpectedHostKeyFP: server.SSHHostKeyFingerprint,
	}

	client, observedHostKeyFP, sshErr := sshclient.Connect(ctx, opts)
	if sshErr != nil {
		if h.Logger != nil {
			h.Logger.ErrorContext(ctx, "SSH connection failed", slog.String("serverId", id), slog.String("error", sshErr.Error()))
		}
		checks.SSH = checkResult{OK: false, Error: sanitizeSSHTestError(sshErr)}
		checks.DumpTool = checkResult{OK: false, Error: "not attempted: SSH connection failed"}
	} else {
		defer client.Close()
		checks.SSH = checkResult{OK: true}

		// Store the observed host key fingerprint if this is first time (TOFU)
		if server.SSHHostKeyFingerprint == nil && observedHostKeyFP != nil {
			if err := h.Servers.SetHostKeyFingerprint(r.Context(), id, *observedHostKeyFP); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to store host key fingerprint")
				return
			}
		}

		checks.DumpTool = h.checkDumpTool(ctx, client, server)
	}

	// Storage check — independent of the SSH result, only when configured.
	// Uses its own timeout budget derived from r.Context() (not the SSH
	// dial's `ctx`, which can already be exhausted by a slow/failed SSH
	// attempt above — reusing it here would fail the storage check with a
	// context-deadline error unrelated to the storage target's actual
	// reachability).
	if server.StorageTargetID != nil {
		storageCtx, storageCancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer storageCancel()
		checks.Storage = h.checkStorageTarget(storageCtx, id, *server.StorageTargetID)
	}

	allOK := checks.SSH.OK && checks.DumpTool.OK && (checks.Storage == nil || checks.Storage.OK)

	newStatus := server.Status
	shouldEnable := server.Enabled
	nextRunAt := server.NextRunAt
	var testErrorForDB *string
	if allOK {
		newStatus = domain.ServerStatusReady
		if server.Status == domain.ServerStatusAwaitingAuthorization {
			// First successful transition: auto-enable (RN-BACKUP-005)
			shouldEnable = true
		}
		var bootstrapErr error
		nextRunAt, bootstrapErr = bootstrapNextRunAt(server.NextRunAt, server.CronExpression, time.Now())
		if bootstrapErr != nil && h.Logger != nil {
			h.Logger.WarnContext(r.Context(), "could not bootstrap next_run_at: invalid cron expression",
				slog.String("serverId", id), slog.String("error", bootstrapErr.Error()))
		}
	} else {
		composed := composeCheckError(checks)
		testErrorForDB = &composed
		// status/enabled unchanged on failure, matching pre-existing behavior
	}

	if err := h.Servers.RecordTestConnectionResult(
		r.Context(), id, newStatus, shouldEnable, nextRunAt, testErrorForDB,
	); err != nil {
		if h.Logger != nil {
			h.Logger.ErrorContext(r.Context(), "failed to record test result", slog.String("serverId", id), slog.String("error", err.Error()))
		}
		writeError(w, http.StatusInternalServerError, "failed to record test result")
		return
	}

	updated, err := h.Servers.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated server")
		return
	}

	writeJSON(w, http.StatusOK, testConnectionResponse{serverDTO: toServerDTO(updated), Checks: checks})
}

// checkDumpTool is the RN-BACKUP-013 replacement for the once-unconditional
// Docker check: when deploymentMode=docker it resolves the configured
// (possibly partial, RN-BACKUP-023) container name against "docker ps" —
// which only lists running containers, so a single match also confirms it is
// running; when deploymentMode=host there is no container to inspect, so it
// confirms the engine's dump binary is reachable on the remote host's PATH
// instead.
func (h *ServerHandlers) checkDumpTool(ctx context.Context, client *sshclient.Client, server domain.Server) checkResult {
	if server.DeploymentMode == domain.DeploymentModeDocker {
		if _, err := scheduler.ResolveContainerName(ctx, client, *server.ContainerName); err != nil {
			if h.Logger != nil {
				h.Logger.ErrorContext(ctx, "docker container check failed",
					slog.String("serverId", server.ID), slog.String("error", err.Error()),
				)
			}
			return checkResult{OK: false, Error: err.Error()}
		}
		return checkResult{OK: true}
	}

	binary := dumpBinaryFor(server.DBEngine)
	cmd := "command -v " + sshclient.ShellQuote(binary)
	_, stderr, exitCode, runErr := client.RunCommand(ctx, cmd)
	if runErr != nil || exitCode != 0 {
		if h.Logger != nil {
			h.Logger.ErrorContext(ctx, "host dump binary check failed",
				slog.String("serverId", server.ID), slog.String("binary", binary),
				slog.Int("exitCode", exitCode), slog.String("stderr", strings.TrimSpace(stderr)),
			)
		}
		return checkResult{OK: false, Error: fmt.Sprintf("%s not found on host PATH", binary)}
	}
	return checkResult{OK: true}
}

// dumpBinaryFor maps a DB engine to the remote binary checkDumpTool must
// find on PATH when deploymentMode=host.
func dumpBinaryFor(engine domain.DBEngine) string {
	switch engine {
	case domain.DBEngineMySQL:
		return "mysqldump"
	case domain.DBEngineSQLServer:
		return "sqlcmd"
	default:
		return "pg_dump"
	}
}

// checkStorageTarget builds the storage backend for the target (decrypting
// whichever secret its type needs) and verifies it is actually usable via
// Backend.CheckAccess. Never returns a decrypted secret or a %v/%+v-
// formatted target/backend in any error string.
func (h *ServerHandlers) checkStorageTarget(ctx context.Context, serverID, storageTargetID string) *checkResult {
	target, err := h.StorageTargets.Get(ctx, storageTargetID)
	if errors.Is(err, repository.ErrNotFound) {
		return &checkResult{OK: false, Error: "storage target not found"}
	}
	if err != nil {
		return &checkResult{OK: false, Error: "failed to load storage target"}
	}

	backend, err := storage.NewBackend(target, h.Sealer)
	if err != nil {
		return &checkResult{OK: false, Error: "failed to decrypt storage credentials"}
	}

	if err := backend.CheckAccess(ctx); err != nil {
		if h.Logger != nil {
			h.Logger.ErrorContext(ctx, "storage target check failed",
				slog.String("serverId", serverID), slog.String("storageTargetId", storageTargetID), slog.String("error", err.Error()),
			)
		}
		return &checkResult{OK: false, Error: "storage connection failed"}
	}
	return &checkResult{OK: true}
}

// RegenerateKey generates a new SSH key pair, stores it, and resets the server to awaiting_authorization.
func (h *ServerHandlers) RegenerateKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Generate new SSH key pair
	privateKeyPEM, publicKeyOpenSSH, fingerprint, err := sshkeys.GenerateKeyPair()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate SSH key pair")
		return
	}

	// Encrypt private key
	encryptedPrivateKey, err := h.Sealer.Encrypt(id, "ssh_private_key_encrypted", string(privateKeyPEM))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt SSH private key")
		return
	}

	// Store encrypted key and public key (transitions back to awaiting_authorization, disables enabled)
	publicKeyStr := string(publicKeyOpenSSH)
	if err := h.Servers.SetSSHKeyPair(r.Context(), id, encryptedPrivateKey, publicKeyStr, fingerprint); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store SSH key pair")
		return
	}

	// Fetch updated server and return
	updated, err := h.Servers.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated server")
		return
	}

	writeJSON(w, http.StatusOK, toServerDTO(updated))
}

// Enable turns on the server's manual on/off switch (RN-BACKUP-024).
func (h *ServerHandlers) Enable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}

// Disable turns off the server's manual on/off switch (RN-BACKUP-024): the
// scheduler stops claiming it for automatic runs, but run-now/backup-now
// remain available.
func (h *ServerHandlers) Disable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *ServerHandlers) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	id := r.PathValue("id")
	_, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.Servers.SetEnabled(r.Context(), id, enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update server")
		return
	}

	updated, err := h.Servers.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated server")
		return
	}

	writeJSON(w, http.StatusOK, toServerDTO(updated))
}

// ResetHostKey clears the pinned SSH host key fingerprint (TOFU reset).
func (h *ServerHandlers) ResetHostKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := h.Servers.Get(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Reset (clear) the host key fingerprint
	if err := h.Servers.ResetHostKeyFingerprint(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reset host key fingerprint")
		return
	}

	// Fetch updated server and return
	updated, err := h.Servers.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated server")
		return
	}

	writeJSON(w, http.StatusOK, toServerDTO(updated))
}

// bootstrapNextRunAt returns the next_run_at value to persist after a
// successful test-connection. If current is already set, it is returned
// unchanged — scheduler.claimAndEnqueue recomputes it on every claim anyway,
// so a stale value here only matters between now and the next claim (see
// the known limitation on cronExpression edits in docs/Arquitetura.md
// "Pontos a definir"). If current is nil (a server that has never been
// claimed by the scheduler — the case for every server on its first
// successful test-connection, RN-BACKUP-005), the next run time is computed
// from cronExpression so the server does not stay permanently invisible to
// GetReadyServersForScheduling, which requires next_run_at IS NOT NULL. An
// invalid cron expression is reported to the caller (to log) but is not
// fatal to the test-connection request itself — current is returned
// unchanged (nil), matching the graceful-degradation behavior of
// claimAndEnqueue when it encounters the same error.
func bootstrapNextRunAt(current *time.Time, cronExpression string, now time.Time) (*time.Time, error) {
	if current != nil {
		return current, nil
	}
	computed, err := scheduler.NextRunTime(cronExpression, now)
	if err != nil {
		return nil, err
	}
	return &computed, nil
}
