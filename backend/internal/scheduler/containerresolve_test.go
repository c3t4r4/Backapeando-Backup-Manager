package scheduler

import (
	"strings"
	"testing"
)

func TestBuildResolveContainerCommand_ShellQuoted(t *testing.T) {
	// RN-BACKUP-008: every free-text component of a remote command must be
	// shell-quoted individually, even a container name fragment the operator
	// configured (not literally arbitrary end-user input, but still not
	// trusted to be shell-metacharacter-free). The command intentionally
	// contains exactly one unquoted "|" (docker ps piped into grep), so each
	// side of that pipe is checked independently for injection instead of
	// checking the whole string, which would also flag our own pipe.
	cmd := buildResolveContainerCommand("app; rm -rf / #")
	if !strings.Contains(cmd, "docker ps") || !strings.Contains(cmd, "grep") {
		t.Fatalf("expected a docker ps | grep pipeline, got: %s", cmd)
	}

	sides := strings.SplitN(cmd, " | ", 2)
	if len(sides) != 2 {
		t.Fatalf("expected exactly one ' | ' separating docker ps from grep, got: %s", cmd)
	}
	for _, side := range sides {
		if err := assertNoUnquotedShellMetachars(side); err != nil {
			t.Fatalf("buildResolveContainerCommand did not shell-quote its input safely: %v\nside: %s\nfull command: %s", err, side, cmd)
		}
	}
}

func TestParseContainerMatches(t *testing.T) {
	tests := []struct {
		name        string
		stdout      string
		exitCode    int
		wantName    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "single match",
			stdout:   "myapp-postgres-1\n",
			exitCode: 0,
			wantName: "myapp-postgres-1",
		},
		{
			name:     "single match, no trailing newline",
			stdout:   "myapp-postgres-1",
			exitCode: 0,
			wantName: "myapp-postgres-1",
		},
		{
			name:        "zero matches (grep exit 1)",
			stdout:      "",
			exitCode:    1,
			wantErr:     true,
			errContains: "nenhum container",
		},
		{
			name:        "exit code zero but empty output",
			stdout:      "\n\n",
			exitCode:    0,
			wantErr:     true,
			errContains: "nenhum container",
		},
		{
			name:        "multiple matches",
			stdout:      "myapp-postgres-1\nmyapp-postgres-2\n",
			exitCode:    0,
			wantErr:     true,
			errContains: "múltiplos containers",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseContainerMatches(tc.stdout, tc.exitCode, "app")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got name %q", got)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantName {
				t.Fatalf("got %q, want %q", got, tc.wantName)
			}
		})
	}
}

func TestParseContainerMatches_AmbiguousErrorListsNames(t *testing.T) {
	_, err := parseContainerMatches("a\nb\n", 0, "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "a, b") {
		t.Fatalf("expected ambiguous-match error to list the matched names, got: %v", err)
	}
}
