// Package localexec runs shell commands in the API/worker process.
// Used when a server's connection mode is local: the same quoted command
// string that would have gone over SSH is passed to sh -c as a single
// argument (no extra shell interpolation).
package localexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// Runner executes commands on the local machine.
type Runner struct{}

// New returns a local command runner. Close is a no-op.
func New() *Runner {
	return &Runner{}
}

// Close implements cmdrun.Runner. There is no session to release.
func (r *Runner) Close() error {
	return nil
}

// RunCommand runs cmd via sh -c. A non-zero exit is reported in exitCode
// with a nil error, matching sshclient.Client.RunCommand. A non-nil error
// means the process could not be started or the context ended first.
func (r *Runner) RunCommand(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error) {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf

	runErr := c.Run()
	if runErr == nil {
		return outBuf.String(), errBuf.String(), 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return outBuf.String(), errBuf.String(), exitErr.ExitCode(), nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return outBuf.String(), errBuf.String(), 1, fmt.Errorf("local command: %w", ctxErr)
	}
	return outBuf.String(), errBuf.String(), 1, fmt.Errorf("local command: %w", runErr)
}

// StreamCommand starts cmd and returns its stdout without buffering the
// full output. wait must be called once; it returns a non-nil error when
// the process exits non-zero or the context is canceled.
func (r *Runner) StreamCommand(ctx context.Context, cmd string) (stdout io.Reader, wait func() error, err error) {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	c.Stderr = &stderrBuf
	if err := c.Start(); err != nil {
		return nil, nil, fmt.Errorf("start: %w", err)
	}

	wait = func() error {
		waitErr := c.Wait()
		if waitErr == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("local command: %w (ctx: %w)", waitErr, ctxErr)
		}
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return fmt.Errorf("command exited %d: %s", exitErr.ExitCode(), stderrBuf.String())
		}
		return fmt.Errorf("local command: %w", waitErr)
	}
	return stdoutPipe, wait, nil
}
