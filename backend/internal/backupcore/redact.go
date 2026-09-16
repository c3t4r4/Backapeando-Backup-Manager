package backupcore

import "strings"

// MaxStoredErrorLen and MaxStoredLogLen bound how much of a raw error or
// captured remote stdout/stderr is persisted in backup_runs.error_message/
// log_output — a noisy remote command could otherwise grow those text
// columns without limit (RN-BACKUP-027).
const (
	MaxStoredErrorLen = 2000
	MaxStoredLogLen   = 8000
)

// RedactSecret replaces every occurrence of secret in s with "***", when
// secret is non-empty. Mirrors the masking already applied to the dump
// command preview shown to the operator (RN-BACKUP-021, frontend/src/lib/
// dumpCommand.ts) — a persisted error message or captured stdout/stderr,
// and any application log line built from the same text, must never leak
// the database password. Shared by both backup execution paths
// (scheduler/executor.go, httpapi/handlers/backup.go) so a fix here can't
// silently apply to only one of them.
func RedactSecret(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}

// TruncateText bounds s to at most max bytes, appending a marker when it
// had to cut, so a truncated value is never mistaken for the complete text.
// Callers must redact before truncating, never the other way around — a
// secret split across the truncation boundary would leave an unredacted,
// unmatched fragment behind.
func TruncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... (truncated)"
}
