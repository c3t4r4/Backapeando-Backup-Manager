// Package cmdrun is the command-execution surface shared by SSH and local
// runners. Dump, container resolution, and test-connection all talk to this
// interface so a local server never opens an SSH session.
package cmdrun

import (
	"context"
	"io"
)

// Runner executes an already shell-quoted command string (RN-BACKUP-008).
// RunCommand returns a non-zero exit code with a nil error when the command
// itself ran and failed; a non-nil error means the command could not be
// started or the context was canceled. StreamCommand must not buffer stdout.
type Runner interface {
	RunCommand(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error)
	StreamCommand(ctx context.Context, cmd string) (stdout io.Reader, wait func() error, err error)
	Close() error
}
