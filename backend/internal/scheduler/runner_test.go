package scheduler

import (
	"context"
	"strings"
	"testing"

	"backapeando-backup-manager/internal/domain"
)

func TestOpenCommandRunner_LocalDoesNotDialSSH(t *testing.T) {
	runner, fp, err := OpenCommandRunner(context.Background(), domain.Server{
		ConnectionMode: domain.ConnectionModeLocal,
		Host:           "localhost",
		Port:           0,
	}, nil)
	if err != nil {
		t.Fatalf("OpenCommandRunner: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	if fp != nil {
		t.Fatal("local runner must not observe an SSH host key")
	}

	stdout, _, exitCode, runErr := runner.RunCommand(context.Background(), "echo local-ok")
	if runErr != nil || exitCode != 0 {
		t.Fatalf("local command exit=%d err=%v", exitCode, runErr)
	}
	if strings.TrimSpace(stdout) != "local-ok" {
		t.Fatalf("stdout = %q", stdout)
	}
}
