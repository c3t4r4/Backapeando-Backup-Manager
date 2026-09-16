package scheduler

import (
	"context"
	"fmt"
	"strings"

	"backapeando-backup-manager/internal/sshclient"
)

// buildResolveContainerCommand renders the remote "docker ps | grep" command
// used to resolve a partial container name (RN-BACKUP-023). "docker ps"
// without "-a" only lists running containers, so a single match also
// confirms the container is running — no separate "docker inspect" check is
// needed.
//
// Both the format string and partial are individually shell-quoted
// (RN-BACKUP-008: every free-text component of a remote command is quoted),
// even though partial is operator-configured, not arbitrary end-user input.
func buildResolveContainerCommand(partial string) string {
	return fmt.Sprintf(
		"docker ps --format %s | grep -F -- %s",
		sshclient.ShellQuote("{{.Names}}"),
		sshclient.ShellQuote(partial),
	)
}

// parseContainerMatches turns the raw "docker ps | grep" output into exactly
// one resolved container name. Zero or multiple matches are treated as an
// error rather than guessing: picking the first match on ambiguity risks
// dumping the wrong database.
func parseContainerMatches(stdout string, exitCode int, partial string) (string, error) {
	var matches []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			matches = append(matches, line)
		}
	}

	if exitCode != 0 || len(matches) == 0 {
		return "", fmt.Errorf("nenhum container em execução corresponde a %q", partial)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("múltiplos containers correspondem a %q: %s — informe um nome mais específico", partial, strings.Join(matches, ", "))
	}
	return matches[0], nil
}

// ResolveContainerName resolves a partial container name (RN-BACKUP-023) into
// the exact, currently running container name, by grepping the remote
// "docker ps" output over the already-connected SSH session.
func ResolveContainerName(ctx context.Context, client *sshclient.Client, partial string) (string, error) {
	stdout, _, exitCode, err := client.RunCommand(ctx, buildResolveContainerCommand(partial))
	if err != nil {
		return "", fmt.Errorf("resolve container name: %w", err)
	}
	return parseContainerMatches(stdout, exitCode, partial)
}
