package handlers

import (
	"testing"

	"backapeando-backup-manager/internal/domain"
)

func TestUpsertServerRequest_ValidateConnectionMode(t *testing.T) {
	base := func() upsertServerRequest {
		return upsertServerRequest{
			Name:           "prod",
			Host:           "10.0.0.1",
			Port:           22,
			SSHUser:        "ubuntu",
			DBEngine:       "postgres",
			DeploymentMode: "docker",
			ContainerName:  "pg",
			DBName:         "app",
			DBUser:         "backup",
		}
	}

	t.Run("ssh without host fails", func(t *testing.T) {
		req := base()
		req.ConnectionMode = "ssh"
		req.Host = "  "
		req.applyConnectionDefaults()
		if err := req.validate(false); err == nil || err.Error() != "host is required" {
			t.Fatalf("validate() = %v, want host is required", err)
		}
	})

	t.Run("ssh without user fails", func(t *testing.T) {
		req := base()
		req.ConnectionMode = "ssh"
		req.SSHUser = ""
		req.applyConnectionDefaults()
		if err := req.validate(false); err == nil || err.Error() != "sshUser is required" {
			t.Fatalf("validate() = %v, want sshUser is required", err)
		}
	})

	t.Run("omitted mode stays ssh and defaults port", func(t *testing.T) {
		req := base()
		req.Port = 0
		req.applyConnectionDefaults()
		if req.ConnectionMode != string(domain.ConnectionModeSSH) {
			t.Fatalf("connectionMode = %q, want ssh", req.ConnectionMode)
		}
		if req.Port != 22 {
			t.Fatalf("port = %d, want 22", req.Port)
		}
		if err := req.validate(false); err != nil {
			t.Fatalf("validate() = %v", err)
		}
	})

	t.Run("local does not require ssh fields", func(t *testing.T) {
		req := base()
		req.ConnectionMode = "local"
		req.Host = ""
		req.SSHUser = ""
		req.Port = 0
		req.applyConnectionDefaults()
		if err := req.validate(false); err != nil {
			t.Fatalf("validate() = %v", err)
		}
		if req.Host != "localhost" || req.Port != 0 || req.SSHUser != "" {
			t.Fatalf("local placeholders = host %q port %d user %q", req.Host, req.Port, req.SSHUser)
		}
	})

	t.Run("local with host deployment fails", func(t *testing.T) {
		req := base()
		req.ConnectionMode = "local"
		req.DeploymentMode = "host"
		req.ContainerName = ""
		req.applyConnectionDefaults()
		if err := req.validate(false); err == nil || err.Error() != "deploymentMode must be docker when connectionMode=local" {
			t.Fatalf("validate() = %v", err)
		}
	})
}
