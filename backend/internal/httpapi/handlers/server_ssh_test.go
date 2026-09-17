package handlers

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSanitizeSSHTestError(t *testing.T) {
	// Test that sanitizeSSHTestError returns generic message
	testErr := errors.New("SSH key authentication failed: permission denied (publickey)")
	sanitized := sanitizeSSHTestError(testErr)

	if sanitized != "SSH connection test failed; see server logs for details" {
		t.Errorf("expected generic message, got: %q", sanitized)
	}
}

func TestSanitizeSSHTestError_NoSensitiveStrings(t *testing.T) {
	// Sensitive strings that must NEVER appear in sanitized output
	sensitiveStrings := []string{
		"permission denied",
		"authentication failed",
		"Host key verification",
		"SSH key",
		"authenticity of host",
		"publickey",
		"password",
		"private key",
		"private_key",
		"fingerprint",
		"cannot parse",
		"bad private key",
		"failed to load key",
	}

	testErr := errors.New("SSH key authentication failed: permission denied (publickey) on host example.com")
	sanitized := sanitizeSSHTestError(testErr)

	for _, sensitive := range sensitiveStrings {
		if strings.Contains(strings.ToLower(sanitized), strings.ToLower(sensitive)) {
			t.Errorf("sanitized error contains sensitive string: %q in output: %q", sensitive, sanitized)
		}
	}
}

func TestSanitizeSSHTestError_PreservesReadability(t *testing.T) {
	// Sanitized error should be readable and actionable for operators
	testErr := errors.New("any SSH error")
	sanitized := sanitizeSSHTestError(testErr)

	if sanitized == "" {
		t.Error("sanitized error should not be empty")
	}

	if !strings.Contains(sanitized, "failed") {
		t.Errorf("sanitized error should contain 'failed', got: %q", sanitized)
	}

	if !strings.Contains(sanitized, "logs") {
		t.Errorf("sanitized error should reference logs, got: %q", sanitized)
	}
}

func TestComposeCheckError_AllPass(t *testing.T) {
	checks := testConnectionChecks{
		SSH:      checkResult{OK: true},
		DumpTool: checkResult{OK: true},
		Storage:  &checkResult{OK: true},
	}
	if got := composeCheckError(checks); got != "" {
		t.Errorf("expected empty string when all checks pass, got %q", got)
	}
}

func TestComposeCheckError_SSHFailsSkipsDumpTool(t *testing.T) {
	checks := testConnectionChecks{
		SSH:      checkResult{OK: false, Error: "ssh boom"},
		DumpTool: checkResult{OK: false, Error: "not attempted: SSH connection failed"},
	}
	got := composeCheckError(checks)
	if !strings.Contains(got, "SSH: ssh boom") {
		t.Errorf("expected SSH failure reported, got %q", got)
	}
	if !strings.Contains(got, "Dump tool: not attempted: SSH connection failed") {
		t.Errorf("expected dump tool not-attempted reason reported, got %q", got)
	}
}

func TestComposeCheckError_DumpToolFailsAloneReportsOnlyDumpTool(t *testing.T) {
	checks := testConnectionChecks{
		SSH:      checkResult{OK: true},
		DumpTool: checkResult{OK: false, Error: "container not found or not running"},
	}
	got := composeCheckError(checks)
	if strings.Contains(got, "SSH:") {
		t.Errorf("SSH passed, should not appear in composed error: %q", got)
	}
	if !strings.Contains(got, "Dump tool: container not found or not running") {
		t.Errorf("expected dump tool failure reported, got %q", got)
	}
}

func TestComposeCheckError_NilStorageIgnored(t *testing.T) {
	checks := testConnectionChecks{
		SSH:      checkResult{OK: true},
		DumpTool: checkResult{OK: true},
		Storage:  nil,
	}
	if got := composeCheckError(checks); got != "" {
		t.Errorf("expected empty string when storage is nil (not configured), got %q", got)
	}
}

func TestComposeCheckError_StorageFailsAlone(t *testing.T) {
	checks := testConnectionChecks{
		SSH:      checkResult{OK: true},
		DumpTool: checkResult{OK: true},
		Storage:  &checkResult{OK: false, Error: "storage connection failed"},
	}
	got := composeCheckError(checks)
	if got != "Storage: storage connection failed" {
		t.Errorf("expected only storage failure reported, got %q", got)
	}
}

func TestComposeCheckError_AllThreeFail(t *testing.T) {
	checks := testConnectionChecks{
		SSH:      checkResult{OK: false, Error: "ssh boom"},
		DumpTool: checkResult{OK: false, Error: "not attempted: SSH connection failed"},
		Storage:  &checkResult{OK: false, Error: "storage connection failed"},
	}
	got := composeCheckError(checks)
	for _, want := range []string{"SSH: ssh boom", "Dump tool: not attempted", "Storage: storage connection failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected composed error to contain %q, got %q", want, got)
		}
	}
}

func TestSanitizeSSHTestError_ConsistentOutput(t *testing.T) {
	// Same input should always produce same output (no randomization)
	err1 := errors.New("SSH connection timeout")
	err2 := errors.New("SSH key not found")

	sanitized1 := sanitizeSSHTestError(err1)
	sanitized2 := sanitizeSSHTestError(err2)

	// Both different SSH errors should produce the same generic message
	if sanitized1 != sanitized2 {
		t.Errorf("different SSH errors should produce the same sanitized message\nError 1 sanitized: %q\nError 2 sanitized: %q", sanitized1, sanitized2)
	}

	// And it should match the expected message
	expectedMsg := "SSH connection test failed; see server logs for details"
	if sanitized1 != expectedMsg {
		t.Errorf("expected %q, got %q", expectedMsg, sanitized1)
	}
}

// TestBootstrapNextRunAt_FirstTimeReady is a regression test for the
// next_run_at bootstrap gap (docs/Progresso.md, docs/Arquitetura.md "Pontos
// a definir"): a server transitioning to status=ready for the first time
// (current == nil, the state of every server before its first successful
// test-connection) must come out with a non-nil next_run_at, or it stays
// invisible to GetReadyServersForScheduling (which requires
// next_run_at IS NOT NULL) forever.
func TestBootstrapNextRunAt_FirstTimeReady(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := bootstrapNextRunAt(nil, "0 3 * * *", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected a non-nil next_run_at, got nil (this is exactly the bootstrap bug)")
	}
	// NextRunTime now always interprets in America/Sao_Paulo, so from 2026-01-01 00:00 UTC
	// (= 2026-12-31 21:00 in Sao Paulo), next 3am is 2026-01-01 06:00 UTC (= 03:00 Sao Paulo)
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	want := time.Date(2026, 1, 1, 3, 0, 0, 0, loc).UTC()
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBootstrapNextRunAt_AlreadyScheduledIsLeftUntouched(t *testing.T) {
	existing := time.Date(2026, 3, 15, 9, 30, 0, 0, time.UTC)
	got, err := bootstrapNextRunAt(&existing, "0 3 * * *", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || !got.Equal(existing) {
		t.Errorf("expected existing next_run_at %v to be left untouched, got %v", existing, got)
	}
}

func TestBootstrapNextRunAt_InvalidCronLeavesNilWithoutFailing(t *testing.T) {
	got, err := bootstrapNextRunAt(nil, "not a cron expression", time.Now())
	if err == nil {
		t.Fatal("expected an error for an invalid cron expression")
	}
	if got != nil {
		t.Errorf("expected nil next_run_at on invalid cron, got %v", got)
	}
}
