/**
 * Type definitions for Backapeando API
 * Generated from docs/API.md and backend domain models
 */

/**
 * Authentication request payload
 */
export interface LoginRequest {
  email: string
  password: string
}

/**
 * Authentication response and user context
 */
export interface AuthResponse {
  id: string
  email: string
  role: string
}

/**
 * Discriminates which database engine a server's dump commands target.
 * Postgres preserves trust/peer-auth (no password required); MySQL and
 * SQL Server require a DB password to be configured.
 */
export type DBEngine = 'postgres' | 'mysql' | 'sqlserver'

/**
 * Discriminates whether a server's database runs inside a Docker container
 * (containerName required) or directly on the remote host/instance
 * (containerName absent).
 */
export type DeploymentMode = 'docker' | 'host'

/**
 * Server (remote database instance) DTO
 * Represents a single database server to be backed up
 * Status machine: pending_key → awaiting_authorization → ready
 */
export interface ServerDTO {
  id: string
  name: string
  host: string
  port: number
  sshUser: string
  dbEngine: DBEngine
  deploymentMode: DeploymentMode
  containerName?: string // present only when deploymentMode='docker'
  dbName: string
  dbUser: string
  /** Never the password itself — only whether one is currently configured. */
  hasDbPassword: boolean
  pgDumpExtraArgs: string
  mysqlDumpExtraArgs: string
  sqlCmdExtraArgs: string
  sshPublicKey?: string
  sshKeyFingerprint?: string
  sshHostKeyFingerprint?: string
  storageTargetId?: string
  cronExpression: string
  enabled: boolean
  status: 'pending_key' | 'awaiting_authorization' | 'ready'
  lastTestConnectionError?: string
  createdAt: string
  updatedAt: string
}

/**
 * Server creation/update request
 * Used for both POST /api/servers and PUT /api/servers/{id}
 */
export interface UpsertServerRequest {
  name: string
  host: string
  port?: number // defaults to 22
  sshUser: string
  dbEngine: DBEngine
  deploymentMode: DeploymentMode
  containerName?: string // required when deploymentMode='docker', omitted otherwise
  dbName: string
  dbUser: string
  /**
   * Write-only. Required on Create when dbEngine != 'postgres'. On Update,
   * an empty/omitted value keeps the currently stored password — only a
   * non-empty value re-encrypts and replaces it.
   */
  dbPassword?: string
  pgDumpExtraArgs?: string
  mysqlDumpExtraArgs?: string
  sqlCmdExtraArgs?: string
  storageTargetId?: string
  cronExpression?: string // defaults to "0 3 * * *"
}

/**
 * Backup run (execution record) DTO
 * Records the result of a single backup attempt
 */
export interface BackupRunDTO {
  id: string
  serverId: string
  status: 'queued' | 'running' | 'success' | 'failed'
  startedAt?: string
  finishedAt?: string
  blobName?: string
  blobSizeBytes?: number
  dumpDurationMs?: number
  uploadDurationMs?: number
  errorMessage?: string
  logOutput?: string
  createdAt: string
}

/**
 * Paginated envelope returned by GET /api/servers/{id}/backup-runs
 * (RN-BACKUP-025) — replaces the previous bare-array response.
 */
export interface BackupRunListResponse {
  items: BackupRunDTO[]
  total: number
  page: number
  pageSize: number
}

/**
 * One component of the combined test-connection result (RN-BACKUP-013)
 */
export interface CheckResult {
  ok: boolean
  error?: string
}

/**
 * Per-check breakdown returned by POST /api/servers/{id}/test-connection.
 * `dumpTool` is the Docker-container-running check in docker mode, or a
 * "dump binary is on PATH" check in host mode. `storage` is absent when the
 * server has no storageTargetId configured.
 */
export interface TestConnectionChecks {
  ssh: CheckResult
  dumpTool: CheckResult
  storage?: CheckResult
}

/**
 * Response from POST /api/servers/{id}/test-connection (RN-BACKUP-013)
 * Extends the plain server DTO with the SSH/dump-tool/Storage check breakdown.
 */
export interface TestConnectionResult extends ServerDTO {
  checks: TestConnectionChecks
}

/**
 * Response from POST /api/servers/{id}/run-now
 * Enqueues a backup with status "queued" for async execution by scheduler
 */
export interface RunNowResponse {
  backupRunId: string
  status: string
  message: string
}

/**
 * Response from POST /api/servers/{id}/backup-now
 * Executes a backup synchronously and returns result + retention info
 * (Synchronous variant for Fase 3)
 */
export interface BackupNowResponse {
  backupRun: BackupRunDTO
  retention: RetentionResultDTO | null
}

/**
 * Retention execution result (dry-run or actual)
 */
export interface RetentionResultDTO {
  dryRun: boolean
  wouldDelete?: string[] // blob names (dry-run only)
  deleted?: string[] // blob names (actual only)
}

/**
 * Discriminates which concrete backend a storage target configures.
 */
export type StorageTargetType = 'azure' | 's3' | 'filesystem'

/**
 * Backup storage target DTO: Azure Blob Storage, an S3-compatible object
 * store, or a local/NFS-mounted filesystem directory (discriminated by
 * `type`). Only the fields relevant to `type` are populated. Secrets (SAS
 * token, S3 secret access key) are never present — once written, a secret is
 * only ever re-set via Update, never read back.
 */
export interface StorageTargetDTO {
  id: string
  name: string
  type: StorageTargetType
  // Azure fields.
  accountName?: string
  containerName?: string
  // S3 fields.
  endpoint?: string
  region?: string
  bucket?: string
  accessKeyId?: string
  usePathStyle?: boolean
  // Filesystem fields.
  rootPath?: string
  createdAt: string
}

/**
 * Storage target creation/update request. Only the fields relevant to
 * `type` are read by the backend; secrets are write-only and optional on
 * update (omitting them keeps the currently stored secret).
 */
export interface UpsertStorageTargetRequest {
  name: string
  type: StorageTargetType
  // Azure fields.
  accountName?: string
  containerName?: string
  sasToken?: string // required for POST when type=azure, optional for PUT
  // S3 fields.
  endpoint?: string
  region?: string
  bucket?: string
  accessKeyId?: string
  secretAccessKey?: string // required for POST when type=s3, optional for PUT
  usePathStyle?: boolean
  // Filesystem fields.
  rootPath?: string
}

/**
 * Retention policy DTO
 * Can be global (GET /api/retention-policy/default) or per-server (override)
 */
export interface RetentionPolicyDTO {
  id: string
  serverId?: string // null/undefined for global policy
  recentCount: number
  monthlyCount: number
}

/**
 * Error response from any endpoint
 */
export interface ErrorResponse {
  error: string
}

/**
 * Admin user (operator account) DTO. password_hash is never present — once
 * written, a password is only ever re-set via PUT, never read back.
 */
export interface AdminUserDTO {
  id: string
  email: string
  cpf: string
  role: string
  createdAt: string
  updatedAt: string
  lastLoginAt?: string
}

/**
 * Admin user creation request (POST /api/admin-users). Password must be at
 * least 12 characters, matching the CLI's create-admin rule.
 */
export interface CreateAdminUserRequest {
  email: string
  cpf: string
  password: string
}

/**
 * Admin user update request (PUT /api/admin-users/{id}). Password is
 * optional — omit or leave empty to keep the current password.
 */
export interface UpdateAdminUserRequest {
  email: string
  cpf: string
  password?: string
}

/**
 * Server counts by status, for the dashboard summary's KPIs.
 */
export interface DashboardServerStatusCountsDTO {
  pendingKey: number
  awaitingAuthorization: number
  ready: number
  disabled: number
}

export interface DashboardServersDTO {
  total: number
  byStatus: DashboardServerStatusCountsDTO
  withConnectionError: number
}

/**
 * Backup-run counts over a rolling window (currently fixed at 30 days),
 * for the dashboard summary's KPIs.
 */
export interface DashboardBackupWindowDTO {
  since: string
  total: number
  success: number
  failed: number
  successRatePercent: number
}

/**
 * Response from GET /api/dashboard/summary
 */
export interface DashboardSummaryDTO {
  servers: DashboardServersDTO
  backupRunsLast30Days: DashboardBackupWindowDTO
  attentionPoints: {
    serversAwaitingAuthorization: ServerDTO[]
    serversWithConnectionError: ServerDTO[]
    recentFailures: BackupRunDTO[]
  }
}

/**
 * One entry of the destinations list shared by every series in
 * DashboardBackupStatsDTO — the single source of truth for ordering, legend
 * labels, and color assignment across the 4 per-destination charts.
 * storageTargetId is null for the synthetic "Sem destino" entry (servers
 * with no storage target configured).
 */
export interface DashboardDestinationDTO {
  storageTargetId: string | null
  name: string
}

/**
 * One destination's data points within a DashboardBucketedStatsDTO. data
 * always has the same length as the enclosing labels, zero-filled for
 * buckets with no runs for this destination.
 */
export interface DashboardDestinationSeriesDTO {
  storageTargetId: string | null
  data: number[]
}

export interface DashboardBucketedStatsDTO {
  since?: string
  until?: string
  year?: number
  labels: string[]
  countSeries: DashboardDestinationSeriesDTO[]
  bytesSeries: DashboardDestinationSeriesDTO[]
}

/**
 * Response from GET /api/dashboard/backup-stats — per-destination backup
 * counts and byte sums, bucketed daily over the last 30 days and monthly
 * over the current year (RN-BACKUP-031: destination reflects the server's
 * CURRENT storage_target_id, not the target in effect when each run
 * happened).
 */
export interface DashboardBackupStatsDTO {
  destinations: DashboardDestinationDTO[]
  dailyLast30Days: DashboardBucketedStatsDTO
  monthlyThisYear: DashboardBucketedStatsDTO
}
