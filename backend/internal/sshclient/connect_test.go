package sshclient

import (
	"context"
	"net"
	"testing"
	"time"

	"backapeando-backup-manager/internal/sshkeys"
)

// TestConnectTimesOutOnStalledHandshake is a regression test for a bug where
// Connect() created a context.WithTimeout but never actually used it: the
// TCP dial went through ssh.Dial directly, whose config.Timeout only bounds
// the TCP connect phase, not the SSH version exchange/handshake that
// follows. Against a peer that accepts the TCP connection but never speaks
// the SSH protocol (a stalled/misbehaving daemon, or — as happened in
// production — a genuinely slow handshake), Connect() would block forever,
// leaving the caller's backup run stuck in "running" with no error, ever.
func TestConnectTimesOutOnStalledHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		close(accepted)
		defer conn.Close()
		// Accept the TCP connection but never write the SSH version banner
		// or anything else — simulates a peer stalled mid-protocol. Block
		// until the test closes the connection (on Connect()'s timeout).
		buf := make([]byte, 1)
		_, _ = conn.Read(buf)
	}()

	privKeyPEM, _, _, err := sshkeys.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err = Connect(ctx, ConnectOptions{
		User:          "irrelevant",
		Host:          ln.Addr().String(),
		PrivateKeyPEM: privKeyPEM,
		Timeout:       200 * time.Millisecond,
	})
	elapsed := time.Since(start)

	select {
	case <-accepted:
	default:
		t.Fatal("test TCP listener never accepted the connection")
	}

	if err == nil {
		t.Fatal("expected Connect() to return an error against a stalled peer, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Connect() took %v to time out against a stalled handshake; expected it to respect the ~200ms configured Timeout, not hang", elapsed)
	}
}
