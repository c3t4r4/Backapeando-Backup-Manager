package backupcore

import (
	"strings"
	"testing"
)

// TestRedactSecret covers RN-BACKUP-027's secret masking for persisted
// error messages/log output: the database password must never survive in
// text that reaches backup_runs (or an application log line built from the
// same text), mirroring the masking already applied to the dump command
// preview (RN-BACKUP-021). Shared by scheduler/executor.go and
// httpapi/handlers/backup.go — a single test here covers both call sites.
func TestRedactSecret(t *testing.T) {
	t.Run("replaces every occurrence of a non-empty secret", func(t *testing.T) {
		got := RedactSecret("connection failed: password=hunter2 (hunter2 rejected)", "hunter2")
		if strings.Contains(got, "hunter2") {
			t.Errorf("RedactSecret() = %q, still contains the secret", got)
		}
	})

	t.Run("empty secret is a no-op", func(t *testing.T) {
		const s = "some error text"
		if got := RedactSecret(s, ""); got != s {
			t.Errorf("RedactSecret() = %q, want unchanged %q", got, s)
		}
	})

	t.Run("text without the secret is unchanged", func(t *testing.T) {
		const s = "connection refused"
		if got := RedactSecret(s, "hunter2"); got != s {
			t.Errorf("RedactSecret() = %q, want unchanged %q", got, s)
		}
	})
}

// TestTruncateText covers the bound applied to persisted error messages/log
// output (RN-BACKUP-027), so a noisy remote command can't grow those text
// columns without limit.
func TestTruncateText(t *testing.T) {
	t.Run("text within the limit is unchanged", func(t *testing.T) {
		if got := TruncateText("short", 10); got != "short" {
			t.Errorf("TruncateText() = %q, want unchanged", got)
		}
	})

	t.Run("text over the limit is cut and marked", func(t *testing.T) {
		got := TruncateText("0123456789extra", 10)
		if len(got) <= 10 {
			t.Errorf("TruncateText() = %q, expected the truncation marker to make it longer than 10 bytes", got)
		}
		if !strings.HasPrefix(got, "0123456789") {
			t.Errorf("TruncateText() = %q, want to start with the first 10 bytes", got)
		}
		if !strings.Contains(got, "truncated") {
			t.Errorf("TruncateText() = %q, want a truncation marker", got)
		}
	})
}
