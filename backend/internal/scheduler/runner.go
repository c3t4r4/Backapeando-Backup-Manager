package scheduler

import (
	"context"
	"fmt"
	"time"

	"backapeando-backup-manager/internal/cmdrun"
	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/localexec"
	"backapeando-backup-manager/internal/sshclient"
)

// OpenCommandRunner returns the executor for server. Local mode never dials
// SSH and ignores privateKeyPEM. SSH mode returns the connected client and
// the observed host-key fingerprint (TOFU). The caller must Close the runner.
func OpenCommandRunner(ctx context.Context, server domain.Server, privateKeyPEM []byte) (cmdrun.Runner, *string, error) {
	if server.ConnectionMode == domain.ConnectionModeLocal {
		return localexec.New(), nil, nil
	}

	host := fmt.Sprintf("%s:%d", server.Host, server.Port)
	client, observed, err := sshclient.Connect(ctx, sshclient.ConnectOptions{
		User:              server.SSHUser,
		Host:              host,
		PrivateKeyPEM:     privateKeyPEM,
		Timeout:           30 * time.Second,
		ExpectedHostKeyFP: server.SSHHostKeyFingerprint,
	})
	if err != nil {
		return nil, nil, err
	}
	return client, observed, nil
}
