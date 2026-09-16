/**
 * Mirrors backend/internal/scheduler/dumpcommand.go byte-for-byte (same
 * token order, same shell-quoting rule via shellQuote/ShellQuote) so the
 * preview shown in the server form never diverges from the command that
 * will actually run over SSH. If you change the Go builders, change this
 * file too — and vice versa.
 *
 * The real DB password is NEVER passed into this module: callers only pass
 * `hasPassword` (whether one is configured), and PASSWORD_MASK stands in
 * for it in the rendered preview (RN-BACKUP-021).
 */

export type DBEngine = 'postgres' | 'mysql' | 'sqlserver'
export type DeploymentMode = 'docker' | 'host'

export interface DumpCommandPreviewInput {
  dbEngine: DBEngine
  deploymentMode: DeploymentMode
  containerName: string
  dbUser: string
  dbName: string
  hasPassword: boolean
  pgDumpExtraArgs: string
  mysqlDumpExtraArgs: string
  sqlCmdExtraArgs: string
}

export interface DumpCommandPreview {
  /** One command line for Postgres/MySQL; three for SQL Server (backup, read, cleanup). */
  lines: string[]
}

const PASSWORD_MASK = '***'

/** Mirrors sshclient.isAlphaNumericDash exactly. */
function isAlphaNumericDash(s: string): boolean {
  for (const ch of s) {
    if (!/[a-zA-Z0-9_.-]/.test(ch)) {
      return false
    }
  }
  return true
}

/** Mirrors sshclient.ShellQuote exactly — same alphabet, same quoting rule. */
export function shellQuote(s: string): string {
  if (s.length === 0) {
    return "''"
  }
  if (isAlphaNumericDash(s)) {
    return s
  }
  return "'" + s.replaceAll("'", "'\\''") + "'"
}

/** Mirrors Go's strings.Fields: split on whitespace runs, no empty tokens. */
function fields(s: string): string[] {
  return s.split(/\s+/).filter((tok) => tok.length > 0)
}

/** Mirrors dumpcommand.go's assembleRemoteCommand exactly. */
function assembleRemoteCommand(
  input: DumpCommandPreviewInput,
  env: string,
  argv: string[]
): string {
  const quoted = argv.map(shellQuote)

  if (input.deploymentMode === 'docker') {
    const parts = ['docker', 'exec']
    if (env !== '') {
      parts.push('-e', shellQuote(env))
    }
    parts.push(shellQuote(input.containerName))
    parts.push(...quoted)
    return parts.join(' ')
  }

  const parts: string[] = []
  if (env !== '') {
    const eq = env.indexOf('=')
    parts.push(env.slice(0, eq + 1) + shellQuote(env.slice(eq + 1)))
  }
  parts.push(...quoted)
  return parts.join(' ')
}

/** Mirrors dumpcommand.go's buildPgDumpCommand exactly. */
function buildPgDumpCommand(
  input: DumpCommandPreviewInput,
  dbPassword: string
): string {
  const argv = [
    'pg_dump',
    '-U',
    input.dbUser,
    '-Fc',
    ...fields(input.pgDumpExtraArgs),
    input.dbName,
  ]
  const env = dbPassword !== '' ? 'PGPASSWORD=' + dbPassword : ''
  return assembleRemoteCommand(input, env, argv)
}

/** Mirrors dumpcommand.go's buildMySQLDumpCommand exactly. */
function buildMySQLDumpCommand(
  input: DumpCommandPreviewInput,
  dbPassword: string
): string {
  const argv = [
    'mysqldump',
    '-u',
    input.dbUser,
    ...fields(input.mysqlDumpExtraArgs),
    input.dbName,
  ]
  const env = 'MYSQL_PWD=' + dbPassword
  return assembleRemoteCommand(input, env, argv)
}

/** Mirrors dumpcommand.go's sqlServerBracketIdentifier exactly. */
function sqlServerBracketIdentifier(name: string): string {
  return '[' + name.replaceAll(']', ']]') + ']'
}

/** Mirrors dumpcommand.go's sqlServerDiskLiteral exactly. */
function sqlServerDiskLiteral(path: string): string {
  return "'" + path.replaceAll("'", "''") + "'"
}

/**
 * Mirrors dumpcommand.go's buildSQLServerDumpPlan. The real backend
 * generates a fresh random temp path per run (uuid.New()) — this preview
 * shows a placeholder (`<temp-file>`) instead of fabricating a fake UUID,
 * since the exact path is never meaningful to the operator, only the shape
 * of the three commands that will run.
 */
function buildSQLServerDumpPlan(
  input: DumpCommandPreviewInput,
  dbPassword: string
): string[] {
  const tempPath = '/tmp/backapeando-sqlserver-<temp-file>.bak'

  const backupQuery = `BACKUP DATABASE ${sqlServerBracketIdentifier(input.dbName)} TO DISK=${sqlServerDiskLiteral(tempPath)} WITH FORMAT`
  const argv = [
    'sqlcmd',
    '-S',
    'localhost',
    '-U',
    input.dbUser,
    '-Q',
    backupQuery,
    ...fields(input.sqlCmdExtraArgs),
  ]
  const env = 'SQLCMDPASSWORD=' + dbPassword
  const preCmd = assembleRemoteCommand(input, env, argv)
  const streamCmd = assembleRemoteCommand(input, '', ['cat', tempPath])
  const cleanupCmd = assembleRemoteCommand(input, '', ['rm', '-f', tempPath])

  return [preCmd, streamCmd, cleanupCmd]
}

/**
 * Builds the exact remote command(s) that will run, mirroring the Go
 * builders in backend/internal/scheduler/dumpcommand.go. The real database
 * password is never interpolated — masked as `***` whenever hasPassword is
 * true, and omitted entirely when false (matching the real backend, which
 * only adds an env prefix for Postgres when a password is configured; MySQL
 * and SQL Server always include the env assignment, even if empty, mirrored
 * here the same way).
 */
export function buildDumpCommandPreview(
  input: DumpCommandPreviewInput
): DumpCommandPreview {
  const dbPassword = input.hasPassword ? PASSWORD_MASK : ''

  switch (input.dbEngine) {
    case 'postgres':
      return { lines: [buildPgDumpCommand(input, dbPassword)] }
    case 'mysql':
      return { lines: [buildMySQLDumpCommand(input, dbPassword)] }
    case 'sqlserver':
      return { lines: buildSQLServerDumpPlan(input, dbPassword) }
    default:
      return { lines: [] }
  }
}
