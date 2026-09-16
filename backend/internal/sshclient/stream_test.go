package sshclient

import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"backapeando-backup-manager/internal/sshkeys"
)

// startHungSSHServer starts a minimal real SSH server that accepts exactly
// one connection and one "session" channel, acknowledges an "exec" request,
// and then never writes anything else and never sends an exit status —
// simulating a remote command that hangs forever (a stalled network path or
// an unresponsive remote process). It returns the listener address and a
// teardown func.
func startHungSSHServer(t *testing.T) string {
	t.Helper()

	_, hostSigner, err := generateHostSigner()
	if err != nil {
		t.Fatalf("generate host signer: %v", err)
	}

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

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
			go func() {
				for req := range requests {
					if req.Type == "exec" && req.WantReply {
						_ = req.Reply(true, nil)
					}
					// Deliberately never close the channel or send an
					// exit-status: simulates a hung remote command. The
					// channel is only closed when the test tears down the
					// connection.
				}
			}()
			_ = channel
		}
	}()

	return ln.Addr().String()
}

// generateHostSigner builds an ssh.Signer for the test server's host key,
// reusing the same ed25519 generation already used for client keys
// elsewhere in this package's tests.
func generateHostSigner() (ssh.PublicKey, ssh.Signer, error) {
	privPEM, _, _, err := sshkeys.GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}
	signer, err := ssh.ParsePrivateKey(privPEM)
	if err != nil {
		return nil, nil, err
	}
	return signer.PublicKey(), signer, nil
}

// TestStreamCommand_CtxCancellationUnblocksHungCommand is a regression test
// for the scheduler/backup-now bug where a backup stream stuck behind a
// stalled network read or an unresponsive remote command occupied its
// worker forever: StreamCommand previously took no context at all, so
// nothing could ever interrupt session.Wait() or unblock a pending read on
// stdout. With ctx now threaded through, wait() must return promptly once
// ctx is canceled instead of hanging (see docs/RegrasNegocio.md
// RN-BACKUP-028).
func TestStreamCommand_CtxCancellationUnblocksHungCommand(t *testing.T) {
	addr := startHungSSHServer(t)

	privKeyPEM, _, _, err := sshkeys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate client key pair: %v", err)
	}

	client, _, err := Connect(context.Background(), ConnectOptions{
		User:          "irrelevant",
		Host:          addr,
		PrivateKeyPEM: privKeyPEM,
		Timeout:       2 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, wait, err := client.StreamCommand(ctx, "irrelevant-hung-command")
	if err != nil {
		t.Fatalf("StreamCommand: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- wait() }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected wait() to return an error when the session is closed by ctx cancellation, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait() never returned after ctx was canceled — the hung command blocked its worker forever, reproducing the reported cron bug")
	}
}
