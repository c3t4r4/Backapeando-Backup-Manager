// Package sshclient provides SSH connectivity with TOFU (Trust On First Use) host-key pinning.
package sshclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client wraps an SSH connection.
type Client struct {
	*ssh.Client
}

// ConnectOptions configures SSH connection parameters.
type ConnectOptions struct {
	User              string        // SSH user (e.g., "backapeando")
	Host              string        // hostname:port (e.g., "backup.local:22")
	PrivateKeyPEM     []byte        // private key in OpenSSH format
	Timeout           time.Duration // connection timeout (e.g., 10s)
	ExpectedHostKeyFP *string       // expected host key fingerprint; nil = TOFU (first connection)
}

// Connect establishes an SSH connection with TOFU host-key pinning.
// Returns the client, the observed host key fingerprint (for TOFU), and any error.
func Connect(ctx context.Context, opts ConnectOptions) (*Client, *string, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Parse private key
	signer, err := ssh.ParsePrivateKey(opts.PrivateKeyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("ParsePrivateKey: %w", err)
	}

	var observedHostKeyFP *string
	hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		hostKeyFP := formatFingerprint(key)
		observedHostKeyFP = &hostKeyFP

		// If expected fingerprint is provided, validate it using constant-time comparison
		if opts.ExpectedHostKeyFP != nil {
			if subtle.ConstantTimeCompare([]byte(hostKeyFP), []byte(*opts.ExpectedHostKeyFP)) != 1 {
				return fmt.Errorf("host key mismatch: expected %s, got %s", *opts.ExpectedHostKeyFP, hostKeyFP)
			}
		}
		// Otherwise, accept it (TOFU)
		return nil
	}

	config := &ssh.ClientConfig{
		User: opts.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         opts.Timeout,
	}

	// ssh.Dial (used here previously) accepts no context, and its
	// config.Timeout only bounds the TCP connect phase — the SSH version
	// exchange and handshake that follow can still block forever against a
	// peer that accepts the TCP connection but stalls mid-protocol. Dial the
	// TCP connection with DialContext, then run the handshake in a goroutine
	// raced against ctx, closing the raw connection to unblock it on
	// timeout — the same pattern RunCommand already uses for session.Run().
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", opts.Host)
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}

	type handshakeResult struct {
		client *ssh.Client
		err    error
	}
	resultCh := make(chan handshakeResult, 1)
	go func() {
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, opts.Host, config)
		if err != nil {
			resultCh <- handshakeResult{err: err}
			return
		}
		resultCh <- handshakeResult{client: ssh.NewClient(sshConn, chans, reqs)}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil {
			// DO NOT return observedHostKeyFP on error: if MITM succeeded on host-key callback but auth failed,
			// we must not pin the attacker's host key. Return nil for fingerprint.
			return nil, nil, fmt.Errorf("ssh handshake: %w", res.err)
		}
		return &Client{res.client}, observedHostKeyFP, nil

	case <-ctx.Done():
		conn.Close()
		// The handshake goroutine is now unblocked (its ops on conn will
		// fail), but it may already have raced past that point and be about
		// to send a *successful* result — closing conn doesn't retroactively
		// un-succeed it. Drain resultCh in the background so that case
		// doesn't leak an open *ssh.Client (with its own read-loop/keepalive
		// goroutines) that nothing else will ever Close().
		go func() {
			if res := <-resultCh; res.client != nil {
				res.client.Close()
			}
		}()
		return nil, nil, fmt.Errorf("ssh handshake: %w", ctx.Err())
	}
}

// RunCommand executes a remote command and captures stdout, stderr, and exit code.
func (c *Client) RunCommand(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error) {
	session, err := c.NewSession()
	if err != nil {
		return "", "", 1, fmt.Errorf("NewSession: %w", err)
	}
	defer session.Close()

	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf

	// Execute with context cancellation support.
	// ssh.Session.Run() does not natively respect context.Done(), so we run it in a goroutine
	// and use select to wait for either completion or context cancellation.
	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case err := <-done:
		if err != nil {
			// Capture exit code if available
			if exitErr, ok := err.(*ssh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
			} else {
				// Connection error, not exit code
				return "", "", 1, fmt.Errorf("session.Run: %w", err)
			}
		}
		return outBuf.String(), errBuf.String(), exitCode, nil

	case <-ctx.Done():
		// Context cancelled or deadline exceeded; close the session to interrupt the command
		session.Close()
		return "", "", 1, fmt.Errorf("session.Run: %w", ctx.Err())
	}
}

// StreamCommand starts a remote command and returns its stdout as a stream,
// without buffering the full output in memory. This is required for
// pg_dump'ing large databases through RunCommand's bytes.Buffer would hold
// the entire dump in the API process's memory.
//
// The caller must read stdout to completion and then call wait() to release
// the session and get the final error (including a non-zero exit status).
// wait() is safe to call exactly once; it always closes the session.
//
// ctx bounds the entire lifetime of the command: session.Read/Wait do not
// natively respect context.Done() (same limitation as RunCommand), so a
// background goroutine closes the session when ctx is done, which unblocks
// any pending read on stdout and makes wait() return promptly instead of
// hanging forever behind a stalled network/remote process.
func (c *Client) StreamCommand(ctx context.Context, cmd string) (stdout io.Reader, wait func() error, err error) {
	session, err := c.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("NewSession: %w", err)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, nil, fmt.Errorf("StdoutPipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	if err := session.Start(cmd); err != nil {
		session.Close()
		return nil, nil, fmt.Errorf("session.Start: %w", err)
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-done:
		}
	}()

	wait = func() error {
		defer close(done)
		defer session.Close()
		err := session.Wait()
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				return fmt.Errorf("command exited %d: %s", exitErr.ExitStatus(), stderrBuf.String())
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("session.Wait: %w (ctx: %w)", err, ctxErr)
			}
			return fmt.Errorf("session.Wait: %w", err)
		}
		return nil
	}

	return stdoutPipe, wait, nil
}

// ShellQuote escapes a string for safe shell usage.
func ShellQuote(s string) string {
	// Empty string must be quoted to remain distinct
	if len(s) == 0 {
		return "''"
	}
	// If no special characters, return as-is
	if isAlphaNumericDash(s) {
		return s
	}
	// Otherwise, wrap in single quotes and escape inner single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func isAlphaNumericDash(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}

func formatFingerprint(key ssh.PublicKey) string {
	hash := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])
}
