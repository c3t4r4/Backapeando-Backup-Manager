package scheduler

import (
	"bytes"
	"context"
	"crypto/rand"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"backapeando-backup-manager/internal/crypto"
	"backapeando-backup-manager/internal/db"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
	"backapeando-backup-manager/internal/sshkeys"
)

// startScriptedSSHServer starts a minimal real SSH server that accepts
// multiple sequential "session" channels over a single connection — matching
// sshclient.RunCommand/StreamCommand, which each open a fresh session per
// remote command. The first exec request it receives replies with
// firstStdout/firstStderr and exits with firstExitCode; every subsequent
// exec request (e.g. a deferred cleanup command run after the first one
// fails) succeeds immediately with no output. This is enough to reproduce
// ExecuteBackup's SQL Server BACKUP DATABASE failure branch
// (executor.go:201-217) without needing to script every command in a dump
// plan.
func startScriptedSSHServer(t *testing.T, firstStdout, firstStderr string, firstExitCode uint32) string {
	t.Helper()

	privPEM, _, _, err := sshkeys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate host signer key pair: %v", err)
	}
	hostSigner, err := ssh.ParsePrivateKey(privPEM)
	if err != nil {
		t.Fatalf("parse host signer: %v", err)
	}

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	var execCount int32

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		defer sshConn.Close()
		go ssh.DiscardRequests(reqs)

		for newChan := range chans {
			if newChan.ChannelType() != "session" {
				_ = newChan.Reject(ssh.UnknownChannelType, "unsupported channel type")
				continue
			}
			channel, requests, err := newChan.Accept()
			if err != nil {
				return
			}
			go func(channel ssh.Channel, requests <-chan *ssh.Request) {
				defer channel.Close()
				for req := range requests {
					if req.Type != "exec" {
						if req.WantReply {
							_ = req.Reply(false, nil)
						}
						continue
					}
					if req.WantReply {
						_ = req.Reply(true, nil)
					}

					stdout, stderr, exitCode := "", "", uint32(0)
					if atomic.AddInt32(&execCount, 1) == 1 {
						stdout, stderr, exitCode = firstStdout, firstStderr, firstExitCode
					}
					if stdout != "" {
						_, _ = channel.Write([]byte(stdout))
					}
					if stderr != "" {
						_, _ = channel.Stderr().Write([]byte(stderr))
					}
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(&struct{ Status uint32 }{Status: exitCode}))
					return
				}
			}(channel, requests)
		}
	}()

	return ln.Addr().String()
}

// TestExecuteBackup_SlogNeverContainsRawSecretOnRemoteCommandFailure is an
// end-to-end regression/defense-in-depth test for RN-BACKUP-027
// (docs/RegrasNegocio.md, docs/Progresso.md): a Security Specialist review
// found that executor.go once logged raw stdout/stderr from a failed remote
// command via slog *before* the redacted version was computed, so the
// database password could leak into the application log in realistic
// scenarios (shell-quoting edge case, non-POSIX remote shell, verbose
// extra-args). That was fixed by reordering redaction before logging, but
// the suggestion to prove it end-to-end (capturing real slog output against
// a real SSH exchange, rather than just re-reading the code) was not
// implemented at the time. This test closes that gap: it runs ExecuteBackup
// against a real (test) SSH server whose BACKUP DATABASE command fails with
// stderr containing a sentinel equal to the server's decrypted DB password,
// and asserts the sentinel never appears in the captured slog output.
func TestExecuteBackup_SlogNeverContainsRawSecretOnRemoteCommandFailure(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	dbPool, err := repository.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(dbPool.Close)

	repos := repository.New(dbPool)

	const sentinel = "S3ntinel-Sh0uldN3verLeak"

	// The BACKUP DATABASE command fails (exit code 1) with stderr containing
	// the sentinel — the exact realistic scenario the Security Specialist
	// flagged (a shell-quoting/verbose-flags edge case echoing the password
	// used on the command line back on stderr).
	addr := startScriptedSSHServer(t, "", "sqlcmd: Login failed, password was: "+sentinel, 1)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}

	clientKeyPEM, _, _, err := sshkeys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate client key pair: %v", err)
	}

	sealerKey := make([]byte, 32)
	if _, err := rand.Read(sealerKey); err != nil {
		t.Fatalf("generate sealer key: %v", err)
	}
	sealer, err := crypto.NewSealer(sealerKey, nil)
	if err != nil {
		t.Fatalf("new sealer: %v", err)
	}

	fsRoot := t.TempDir()
	storageTarget := domain.StorageTarget{
		ID:         uuid.NewString(),
		Name:       "slog-e2e-test-target",
		Type:       domain.StorageTargetTypeFilesystem,
		FSRootPath: &fsRoot,
	}
	createdTarget, err := repos.StorageTargets.Create(ctx, storageTarget)
	if err != nil {
		t.Fatalf("create storage target: %v", err)
	}
	t.Cleanup(func() { _ = repos.StorageTargets.Delete(context.Background(), createdTarget.ID) })

	encryptedPassword, err := sealer.Encrypt("", domain.DBPasswordAAD, sentinel)
	if err != nil {
		t.Fatalf("encrypt db password: %v", err)
	}

	server, err := repos.Servers.Create(ctx, domain.Server{
		Name:                "slog-e2e-test-server",
		Host:                host,
		Port:                port,
		SSHUser:             "irrelevant",
		DBEngine:            domain.DBEngineSQLServer,
		DeploymentMode:      domain.DeploymentModeHost,
		DBName:              "testdb",
		DBUser:              "testuser",
		DBPasswordEncrypted: encryptedPassword,
		CronExpression:      "0 3 * * *",
		StorageTargetID:     &createdTarget.ID,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() { _ = repos.Servers.Delete(context.Background(), server.ID) })

	// DBPasswordEncrypted's AAD was created against the real server ID above
	// (crypto.Sealer authenticates the AAD, RN-BACKUP-019/domain.DBPasswordAAD)
	// — re-encrypt now that the ID is known, matching how handlers.Create
	// does it in production (encrypt after the row's ID exists).
	encryptedPassword, err = sealer.Encrypt(server.ID, domain.DBPasswordAAD, sentinel)
	if err != nil {
		t.Fatalf("re-encrypt db password with real server id: %v", err)
	}
	if err := repos.Servers.SetDBPassword(ctx, server.ID, encryptedPassword); err != nil {
		t.Fatalf("set db password: %v", err)
	}

	encryptedPrivateKey, err := sealer.Encrypt(server.ID, "ssh_private_key_encrypted", string(clientKeyPEM))
	if err != nil {
		t.Fatalf("encrypt ssh private key: %v", err)
	}
	if err := repos.Servers.SetSSHKeyPair(ctx, server.ID, encryptedPrivateKey, "unused-public-key", "unused-fingerprint"); err != nil {
		t.Fatalf("set ssh key pair: %v", err)
	}

	server, err = repos.Servers.Get(ctx, server.ID)
	if err != nil {
		t.Fatalf("reload server: %v", err)
	}

	backupRun, err := repos.BackupRuns.Create(ctx, server.ID)
	if err != nil {
		t.Fatalf("create backup run: %v", err)
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	executor := NewBackupExecutor(dbPool, repos, sealer, logger)

	runErr := executor.ExecuteBackup(ctx, &backupRun, &server, &createdTarget)
	if runErr == nil {
		t.Fatal("expected ExecuteBackup to return an error for the failed BACKUP DATABASE command, got nil")
	}

	logOutput := logBuf.String()
	if strings.Contains(logOutput, sentinel) {
		t.Fatalf("slog output contains the raw database password sentinel — RN-BACKUP-027 redaction regressed:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "***") {
		t.Errorf("expected the redacted placeholder %q to appear in slog output (proving redaction actually ran, not just that the sentinel happens to be absent), got:\n%s", "***", logOutput)
	}

	storedRun, err := repos.BackupRuns.Get(ctx, backupRun.ID)
	if err != nil {
		t.Fatalf("get backup run: %v", err)
	}
	if storedRun.ErrorMessage != nil && strings.Contains(*storedRun.ErrorMessage, sentinel) {
		t.Errorf("persisted error_message contains the raw sentinel: %q", *storedRun.ErrorMessage)
	}
	if storedRun.LogOutput != nil && strings.Contains(*storedRun.LogOutput, sentinel) {
		t.Errorf("persisted log_output contains the raw sentinel: %q", *storedRun.LogOutput)
	}
}
