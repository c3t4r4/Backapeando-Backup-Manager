package scheduler

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"backapeando-backup-manager/internal/domain"
	"backapeando-backup-manager/internal/sshclient"
)

// DumpPlan describes how to obtain a database dump over an already-connected
// SSH session. Postgres/MySQL are a single streamed command (StreamCmd only).
// SQL Server is two-phase: PreCmd runs BACKUP DATABASE synchronously to a
// temp file on the remote host, then StreamCmd ("cat <path>") reads it, then
// CleanupCmd ("rm -f <path>") always runs afterward — whether the dump
// succeeded or failed — to avoid leaving a plaintext backup file behind.
type DumpPlan struct {
	// StreamCmd's stdout is the dump bytes to upload. For Postgres/MySQL
	// this is the dump command itself; for SQL Server it is "cat
	// <tempPath>", run only after PreCmd succeeds.
	StreamCmd string

	// PreCmd is non-empty only for SQL Server: the synchronous
	// "sqlcmd ... BACKUP DATABASE ..." command.
	PreCmd string

	// CleanupCmd is non-empty only for SQL Server: removes the temp backup
	// file on the remote host (or inside the container, in docker mode).
	CleanupCmd string
}

// BuildDumpPlan builds the remote command(s) to obtain a dump for server,
// combining DBEngine and DeploymentMode. dbPassword is the already-decrypted
// plaintext password (empty string for postgres servers with no password
// configured, preserving pre-existing trust/peer-auth behavior). Every
// free-text/user-supplied component (ContainerName, DBUser, DBName, each
// engine's ExtraArgs tokens, dbPassword) is individually shell-quoted via
// sshclient.ShellQuote (RN-BACKUP-008).
func BuildDumpPlan(server domain.Server, dbPassword string) (DumpPlan, error) {
	switch server.DBEngine {
	case domain.DBEnginePostgres:
		return DumpPlan{StreamCmd: buildPgDumpCommand(server, dbPassword)}, nil
	case domain.DBEngineMySQL:
		return DumpPlan{StreamCmd: buildMySQLDumpCommand(server, dbPassword)}, nil
	case domain.DBEngineSQLServer:
		return buildSQLServerDumpPlan(server, dbPassword), nil
	default:
		return DumpPlan{}, fmt.Errorf("unsupported db engine %q", server.DBEngine)
	}
}

// buildPgDumpCommand assembles "pg_dump -U <user> -Fc <extraArgs...>
// <dbname>". When dbPassword is non-empty, PGPASSWORD (the standard libpq
// env var) is set for the pg_dump process; when empty, no env var is set at
// all — this preserves the exact command generated before DB passwords
// existed, for servers that still rely on trust/peer auth inside the
// container.
func buildPgDumpCommand(server domain.Server, dbPassword string) string {
	argv := []string{"pg_dump", "-U", server.DBUser, "-Fc"}
	argv = append(argv, strings.Fields(server.PgDumpExtraArgs)...)
	argv = append(argv, server.DBName)

	env := ""
	if dbPassword != "" {
		env = "PGPASSWORD=" + dbPassword
	}
	return assembleRemoteCommand(server, env, argv)
}

// buildMySQLDumpCommand assembles "mysqldump -u <user> <extraArgs...>
// <dbname>" with MYSQL_PWD set. Unlike Postgres, MySQL has no ambient
// trust/peer-auth equivalent usable here, so dbPassword is mandatory for
// this engine (enforced by upsertServerRequest.validate and the
// servers_password_required_by_engine DB constraint) and always set.
func buildMySQLDumpCommand(server domain.Server, dbPassword string) string {
	argv := []string{"mysqldump", "-u", server.DBUser}
	argv = append(argv, strings.Fields(server.MySQLDumpExtraArgs)...)
	argv = append(argv, server.DBName)

	env := "MYSQL_PWD=" + dbPassword
	return assembleRemoteCommand(server, env, argv)
}

// buildSQLServerDumpPlan builds the 3-command DumpPlan for SQL Server, whose
// native BACKUP DATABASE has no stdout-streaming equivalent to pg_dump/
// mysqldump: it writes a file on the remote host (or, in docker mode, inside
// the container's filesystem), so the file must be read back and then
// removed. tempPath is generated per-call (never derived from operator
// input) to avoid collisions between concurrent/overlapping runs of the same
// server and to sidestep any need to shell-quote it beyond what
// ShellQuote already does defensively.
func buildSQLServerDumpPlan(server domain.Server, dbPassword string) DumpPlan {
	tempPath := fmt.Sprintf("/tmp/backapeando-sqlserver-%s.bak", uuid.New().String())

	backupQuery := fmt.Sprintf("BACKUP DATABASE %s TO DISK=%s WITH FORMAT", sqlServerBracketIdentifier(server.DBName), sqlServerDiskLiteral(tempPath))
	argv := []string{"sqlcmd", "-S", "localhost", "-U", server.DBUser, "-Q", backupQuery}
	argv = append(argv, strings.Fields(server.SqlCmdExtraArgs)...)

	env := "SQLCMDPASSWORD=" + dbPassword
	preCmd := assembleRemoteCommand(server, env, argv)

	streamCmd := assembleRemoteCommand(server, "", []string{"cat", tempPath})
	cleanupCmd := assembleRemoteCommand(server, "", []string{"rm", "-f", tempPath})

	return DumpPlan{PreCmd: preCmd, StreamCmd: streamCmd, CleanupCmd: cleanupCmd}
}

// sqlServerBracketIdentifier wraps a name as a T-SQL bracketed identifier
// (e.g. BACKUP DATABASE [name]), escaping any literal ']' as ']]' per T-SQL
// identifier-quoting rules. DBName is already constrained by
// upsertServerRequest.validate (containerNameRegex, which excludes ']') at
// the HTTP boundary, but this builder does not rely on that invariant
// holding in every caller — the escaping is applied unconditionally.
func sqlServerBracketIdentifier(name string) string {
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}

// sqlServerDiskLiteral wraps a path as a T-SQL string literal for use inside
// a BACKUP DATABASE ... TO DISK=<literal> clause. This is a distinct escaping
// concern from shell quoting: tempPath is generated by this package (never
// operator input), but the single-quote wrapping is still applied
// defensively, matching T-SQL string literal syntax.
func sqlServerDiskLiteral(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}

// assembleRemoteCommand renders argv (plain, unquoted tokens) plus an
// optional "NAME=value" env assignment into the single shell command string
// actually sent over SSH (the remote sshd invokes the login shell with
// "-c <this string>", so the whole thing is shell-parsed once, remotely).
//
// Every argv token is shell-quoted individually (RN-BACKUP-008). The env
// assignment, when present, is handled differently per deployment mode:
//   - host: rendered as a leading POSIX assignment word "NAME=<quoted
//     value>", which the remote shell applies only to the process about to
//     be exec'd (argv[0]) — safe because argv is the actual command here.
//   - docker: argv[0] is "docker", not the target process, so a leading
//     assignment word would set the env for "docker" itself, not for the
//     command it execs inside the container. docker exec's own "-e
//     NAME=value" flag is used instead, which docker forwards into the
//     container's exec environment.
func assembleRemoteCommand(server domain.Server, env string, argv []string) string {
	quoted := make([]string, len(argv))
	for i, tok := range argv {
		quoted[i] = sshclient.ShellQuote(tok)
	}

	if server.DeploymentMode == domain.DeploymentModeDocker {
		parts := []string{"docker", "exec"}
		if env != "" {
			parts = append(parts, "-e", sshclient.ShellQuote(env))
		}
		parts = append(parts, sshclient.ShellQuote(*server.ContainerName))
		parts = append(parts, quoted...)
		return strings.Join(parts, " ")
	}

	var parts []string
	if env != "" {
		eq := strings.IndexByte(env, '=')
		parts = append(parts, env[:eq+1]+sshclient.ShellQuote(env[eq+1:]))
	}
	parts = append(parts, quoted...)
	return strings.Join(parts, " ")
}
