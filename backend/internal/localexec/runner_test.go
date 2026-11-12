package localexec

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunCommand_Echo(t *testing.T) {
	stdout, stderr, exitCode, err := New().RunCommand(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr)
	}
	if strings.TrimSpace(stdout) != "hello" {
		t.Fatalf("stdout = %q, want hello", stdout)
	}
}

func TestRunCommand_NonZeroExitIsNotAnError(t *testing.T) {
	_, _, exitCode, err := New().RunCommand(context.Background(), "exit 3")
	if err != nil {
		t.Fatalf("RunCommand err = %v, want nil for a command that ran", err)
	}
	if exitCode != 3 {
		t.Fatalf("exit code = %d, want 3", exitCode)
	}
}

func TestRunCommand_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err := New().RunCommand(ctx, "sleep 30")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestStreamCommand_EchoAndCancel(t *testing.T) {
	stdout, wait, err := New().StreamCommand(context.Background(), "echo streamed")
	if err != nil {
		t.Fatalf("StreamCommand: %v", err)
	}
	body, readErr := io.ReadAll(stdout)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if err := wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if strings.TrimSpace(string(body)) != "streamed" {
		t.Fatalf("stdout = %q, want streamed", body)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stdout, wait, err = New().StreamCommand(ctx, "sleep 30")
	if err != nil {
		t.Fatalf("StreamCommand sleep: %v", err)
	}
	cancel()
	done := make(chan error, 1)
	go func() {
		_, _ = io.Copy(io.Discard, stdout)
		done <- wait()
	}()
	select {
	case waitErr := <-done:
		if waitErr == nil {
			t.Fatal("expected wait error after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return after context cancel")
	}
}
