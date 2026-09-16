package handlers

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"testing"
)

// TestAuditWriteFailureAfterDelete covers the ALTO finding (Validator): if
// RetentionDeletions.Create fails after azureblob.DeleteBlob has already
// succeeded, the deleted blob's name must never disappear silently. This
// test cannot exercise the real repository.RetentionDeletionRepo (it
// requires a live Postgres connection), so it exercises
// auditWriteFailureAfterDelete directly — the exact function the del
// closure in sweepRetention calls on that error path — and asserts the
// blob name is captured in two independent places: the structured log
// output, and the returned error's message.
func TestAuditWriteFailureAfterDelete(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	const (
		serverID    = "srv-123"
		backupRunID = "run-456"
		blobName    = "my-server/my_db_20260914T120000Z.dump"
	)
	underlying := errors.New("connection refused")

	err := auditWriteFailureAfterDelete(context.Background(), logger, serverID, backupRunID, blobName, underlying)

	if err == nil {
		t.Fatal("expected a non-nil error to be returned")
	}
	if !strings.Contains(err.Error(), blobName) {
		t.Errorf("returned error does not mention the deleted blob name: %v", err)
	}
	if !errors.Is(err, underlying) {
		t.Errorf("returned error does not wrap the underlying audit error: %v", err)
	}

	logged := logBuf.String()
	if !strings.Contains(logged, blobName) {
		t.Errorf("log output does not mention the deleted blob name; a real deletion with a failed audit write must never be untraceable. log: %s", logged)
	}
	if !strings.Contains(logged, "audit_write_after_delete_succeeded") {
		t.Errorf("log output missing the stable stage marker used for alerting/grep; log: %s", logged)
	}
	if !strings.Contains(logged, serverID) || !strings.Contains(logged, backupRunID) {
		t.Errorf("log output missing serverId/backupRunId context; log: %s", logged)
	}
}

// TestAuditWriteFailureAfterDelete_NilLogger ensures a nil Logger (e.g. a
// BackupHandlers wired without one, such as in a minimal test) never
// panics — the blob name must still survive in the returned error.
func TestAuditWriteFailureAfterDelete_NilLogger(t *testing.T) {
	err := auditWriteFailureAfterDelete(context.Background(), nil, "srv-1", "run-1", "blob-1", errors.New("boom"))
	if err == nil || !strings.Contains(err.Error(), "blob-1") {
		t.Errorf("expected error mentioning blob-1 even with nil logger, got: %v", err)
	}
}

// TestFinalizeBackupError is a regression test for a bug found by the
// Validator: performBackup's deferred call used to skip truncation entirely
// whenever dbPassword == "" (e.g. a Postgres server using trust/peer auth,
// a legitimate and common configuration — RN-BACKUP-017), leaving
// error_message/log_output persisted unbounded in that case even though
// RN-BACKUP-027 requires both fields to always be size-capped. Redaction is
// correctly a no-op with an empty secret (backupcore.RedactSecret already
// handles that); truncation must never be skipped.
func TestFinalizeBackupError(t *testing.T) {
	t.Run("dbPassword empty: no redaction needed, but still truncates", func(t *testing.T) {
		long := strings.Repeat("x", 9000)
		err, logOutput := finalizeBackupError(errors.New(long), long, "")
		if len(err.Error()) >= 9000 {
			t.Errorf("error was not truncated when dbPassword is empty: len=%d", len(err.Error()))
		}
		if len(logOutput) >= 9000 {
			t.Errorf("logOutput was not truncated when dbPassword is empty: len=%d", len(logOutput))
		}
	})

	t.Run("dbPassword set: redacts and truncates", func(t *testing.T) {
		err, logOutput := finalizeBackupError(
			errors.New("dump failed: MYSQL_PWD=hunter2 mysqldump: access denied"),
			"stdout: (empty)\nstderr: MYSQL_PWD=hunter2 mysqldump: access denied",
			"hunter2",
		)
		if strings.Contains(err.Error(), "hunter2") {
			t.Errorf("error still contains the db password: %v", err)
		}
		if strings.Contains(logOutput, "hunter2") {
			t.Errorf("logOutput still contains the db password: %q", logOutput)
		}
	})

	t.Run("nil error and empty logOutput pass through untouched", func(t *testing.T) {
		err, logOutput := finalizeBackupError(nil, "", "secret")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if logOutput != "" {
			t.Errorf("expected empty logOutput, got %q", logOutput)
		}
	})
}

// TestParseBackupRunListParams covers RN-BACKUP-025's query-param validation
// for GET /api/servers/{id}/backup-runs — defaults, bounds, and rejection of
// unknown values — without needing an HTTP request or a database.
func TestParseBackupRunListParams(t *testing.T) {
	t.Run("defaults when nothing is provided", func(t *testing.T) {
		page, pageSize, status, err := parseBackupRunListParams(url.Values{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if page != 1 {
			t.Errorf("page = %d, want 1", page)
		}
		if pageSize != defaultBackupRunPageSize {
			t.Errorf("pageSize = %d, want %d", pageSize, defaultBackupRunPageSize)
		}
		if status != nil {
			t.Errorf("status = %v, want nil", *status)
		}
	})

	t.Run("valid page/pageSize/status are parsed through", func(t *testing.T) {
		page, pageSize, status, err := parseBackupRunListParams(url.Values{
			"page":     {"3"},
			"pageSize": {"25"},
			"status":   {"failed"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if page != 3 || pageSize != 25 {
			t.Errorf("page=%d pageSize=%d, want 3/25", page, pageSize)
		}
		if status == nil || *status != "failed" {
			t.Errorf("status = %v, want \"failed\"", status)
		}
	})

	t.Run("page below 1 is rejected", func(t *testing.T) {
		if _, _, _, err := parseBackupRunListParams(url.Values{"page": {"0"}}); err == nil {
			t.Error("expected error for page=0")
		}
	})

	t.Run("non-numeric page is rejected", func(t *testing.T) {
		if _, _, _, err := parseBackupRunListParams(url.Values{"page": {"abc"}}); err == nil {
			t.Error("expected error for non-numeric page")
		}
	})

	t.Run("pageSize above the cap is rejected", func(t *testing.T) {
		if _, _, _, err := parseBackupRunListParams(url.Values{"pageSize": {"1000000"}}); err == nil {
			t.Error("expected error for pageSize exceeding maxBackupRunPageSize")
		}
	})

	t.Run("pageSize of 0 is rejected", func(t *testing.T) {
		if _, _, _, err := parseBackupRunListParams(url.Values{"pageSize": {"0"}}); err == nil {
			t.Error("expected error for pageSize=0")
		}
	})

	t.Run("unknown status is rejected", func(t *testing.T) {
		if _, _, _, err := parseBackupRunListParams(url.Values{"status": {"bogus"}}); err == nil {
			t.Error("expected error for an unrecognized status value")
		}
	})

	for _, valid := range []string{"queued", "running", "success", "failed"} {
		t.Run("status "+valid+" is accepted", func(t *testing.T) {
			_, _, status, err := parseBackupRunListParams(url.Values{"status": {valid}})
			if err != nil {
				t.Fatalf("unexpected error for status=%s: %v", valid, err)
			}
			if status == nil || *status != valid {
				t.Errorf("status = %v, want %q", status, valid)
			}
		})
	}
}

// TestParseAllBackupRunListParams covers RN-BACKUP-026's query-param
// validation for GET /api/backup-runs — the same page/pageSize/status rules
// as the per-server endpoint, plus an optional serverId that defaults to nil
// ("every server") when absent.
func TestParseAllBackupRunListParams(t *testing.T) {
	t.Run("defaults when nothing is provided: no server filter", func(t *testing.T) {
		page, pageSize, status, serverID, err := parseAllBackupRunListParams(url.Values{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if page != 1 || pageSize != defaultBackupRunPageSize {
			t.Errorf("page=%d pageSize=%d, want 1/%d", page, pageSize, defaultBackupRunPageSize)
		}
		if status != nil {
			t.Errorf("status = %v, want nil", *status)
		}
		if serverID != nil {
			t.Errorf("serverID = %v, want nil", *serverID)
		}
	})

	t.Run("serverId is parsed through when present", func(t *testing.T) {
		_, _, _, serverID, err := parseAllBackupRunListParams(url.Values{"serverId": {"srv-123"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if serverID == nil || *serverID != "srv-123" {
			t.Errorf("serverID = %v, want \"srv-123\"", serverID)
		}
	})

	t.Run("invalid page is still rejected", func(t *testing.T) {
		if _, _, _, _, err := parseAllBackupRunListParams(url.Values{"page": {"0"}}); err == nil {
			t.Error("expected error for page=0")
		}
	})

	t.Run("unknown status is still rejected", func(t *testing.T) {
		if _, _, _, _, err := parseAllBackupRunListParams(url.Values{"status": {"bogus"}}); err == nil {
			t.Error("expected error for an unrecognized status value")
		}
	})
}
