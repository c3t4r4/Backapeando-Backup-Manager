package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/repository"
)

// StubServerRepo implements the serverRepository interface for testing.
type StubServerRepo struct {
	getFn func(ctx context.Context, id string) (domain.Server, error)
}

func (s *StubServerRepo) Get(ctx context.Context, id string) (domain.Server, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return domain.Server{}, nil
}

// StubBackupRunRepo implements the backupRunRepository interface for testing.
type StubBackupRunRepo struct {
	createQueuedFn func(ctx context.Context, serverID string) (domain.BackupRun, error)
}

func (s *StubBackupRunRepo) CreateQueued(ctx context.Context, serverID string) (domain.BackupRun, error) {
	if s.createQueuedFn != nil {
		return s.createQueuedFn(ctx, serverID)
	}
	return domain.BackupRun{}, nil
}

// TestRunServerBackupNow_Success tests the happy path: server is ready,
// enabled, has a storage target, and a backup run is successfully queued.
func TestRunServerBackupNow_Success(t *testing.T) {
	const serverID = "srv-123"
	const runID = "run-456"

	targetID := "storage-target-1"
	mockServer := domain.Server{
		ID:              serverID,
		Name:            "prod-db",
		Status:          domain.ServerStatusReady,
		Enabled:         true,
		StorageTargetID: &targetID,
	}

	mockRun := domain.BackupRun{
		ID:       runID,
		ServerID: serverID,
		Status:   domain.BackupRunStatusQueued,
	}

	backupRunRepo := &StubBackupRunRepo{
		createQueuedFn: func(ctx context.Context, s string) (domain.BackupRun, error) {
			if s != serverID {
				t.Errorf("expected serverID %q, got %q", serverID, s)
			}
			return mockRun, nil
		},
	}

	serverRepo := &StubServerRepo{
		getFn: func(ctx context.Context, id string) (domain.Server, error) {
			if id != serverID {
				t.Errorf("expected serverID %q, got %q", serverID, id)
			}
			return mockServer, nil
		},
	}

	handler := &RunNowHandlers{
		BackupRuns: backupRunRepo,
		Servers:    serverRepo,
		Logger:     nil,
	}

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/run-now", nil)
	req.SetPathValue("id", serverID)
	w := httptest.NewRecorder()

	handler.RunServerBackupNow(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202 Accepted, got %d", w.Code)
	}

	var response runNowResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.BackupRunID != runID {
		t.Errorf("expected backupRunId %q, got %q", runID, response.BackupRunID)
	}
	if response.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", response.Status)
	}
	if response.Message == "" {
		t.Errorf("expected non-empty message")
	}
}

// TestRunServerBackupNow_ServerNotFound tests the case where the server
// does not exist, expecting a 404 Not Found response.
func TestRunServerBackupNow_ServerNotFound(t *testing.T) {
	const serverID = "srv-nonexistent"

	serverRepo := &StubServerRepo{
		getFn: func(ctx context.Context, id string) (domain.Server, error) {
			return domain.Server{}, repository.ErrNotFound
		},
	}

	handler := &RunNowHandlers{
		BackupRuns: &StubBackupRunRepo{},
		Servers:    serverRepo,
		Logger:     nil,
	}

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/run-now", nil)
	req.SetPathValue("id", serverID)
	w := httptest.NewRecorder()

	handler.RunServerBackupNow(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 Not Found, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] == "" {
		t.Errorf("expected non-empty error message")
	}
}

// TestRunServerBackupNow_ServerNotReady tests the case where the server
// exists but does not meet the readiness prerequisites. Tests three scenarios:
// (1) status != ready, (2) enabled=false, (3) StorageTargetID=nil.
// All should return 409 Conflict.
func TestRunServerBackupNow_ServerNotReady(t *testing.T) {
	targetID := "storage-target-1"
	tests := []struct {
		name   string
		server domain.Server
	}{
		{
			name: "status is not ready",
			server: domain.Server{
				ID:              "srv-1",
				Name:            "test-db",
				Status:          domain.ServerStatusPendingKey,
				Enabled:         true,
				StorageTargetID: &targetID,
			},
		},
		{
			name: "enabled is false",
			server: domain.Server{
				ID:              "srv-2",
				Name:            "test-db",
				Status:          domain.ServerStatusReady,
				Enabled:         false,
				StorageTargetID: &targetID,
			},
		},
		{
			name: "storage target is nil",
			server: domain.Server{
				ID:              "srv-3",
				Name:            "test-db",
				Status:          domain.ServerStatusReady,
				Enabled:         true,
				StorageTargetID: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverRepo := &StubServerRepo{
				getFn: func(ctx context.Context, id string) (domain.Server, error) {
					return tt.server, nil
				},
			}

			handler := &RunNowHandlers{
				BackupRuns: &StubBackupRunRepo{},
				Servers:    serverRepo,
				Logger:     nil,
			}

			req := httptest.NewRequest("POST", "/api/servers/"+tt.server.ID+"/run-now", nil)
			req.SetPathValue("id", tt.server.ID)
			w := httptest.NewRecorder()

			handler.RunServerBackupNow(w, req)

			if w.Code != http.StatusConflict {
				t.Errorf("expected status 409 Conflict, got %d", w.Code)
			}

			var response map[string]string
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if response["error"] == "" {
				t.Errorf("expected non-empty error message")
			}
		})
	}
}

// TestRunServerBackupNow_CreateQueuedFailure tests the case where
// BackupRuns.CreateQueued returns an error, expecting a 500 Internal Server Error.
func TestRunServerBackupNow_CreateQueuedFailure(t *testing.T) {
	const serverID = "srv-123"

	targetID := "storage-target-1"
	mockServer := domain.Server{
		ID:              serverID,
		Name:            "prod-db",
		Status:          domain.ServerStatusReady,
		Enabled:         true,
		StorageTargetID: &targetID,
	}

	backupRunRepo := &StubBackupRunRepo{
		createQueuedFn: func(ctx context.Context, s string) (domain.BackupRun, error) {
			return domain.BackupRun{}, context.Canceled
		},
	}

	serverRepo := &StubServerRepo{
		getFn: func(ctx context.Context, id string) (domain.Server, error) {
			return mockServer, nil
		},
	}

	handler := &RunNowHandlers{
		BackupRuns: backupRunRepo,
		Servers:    serverRepo,
		Logger:     nil,
	}

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/run-now", nil)
	req.SetPathValue("id", serverID)
	w := httptest.NewRecorder()

	handler.RunServerBackupNow(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 Internal Server Error, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] == "" {
		t.Errorf("expected non-empty error message")
	}
}
