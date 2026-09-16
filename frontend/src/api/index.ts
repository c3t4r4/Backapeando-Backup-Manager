/**
 * API functions for Backapeando
 * All functions use the axios client with auto-injected CSRF tokens
 * Exported for use in components, composables, and stores
 */

import client from './client'
import type {
  LoginRequest,
  AuthResponse,
  ServerDTO,
  UpsertServerRequest,
  BackupRunDTO,
  BackupRunListResponse,
  CheckResult,
  TestConnectionChecks,
  TestConnectionResult,
  RunNowResponse,
  BackupNowResponse,
  StorageTargetDTO,
  UpsertStorageTargetRequest,
  RetentionPolicyDTO,
  DashboardSummaryDTO,
  DashboardBackupStatsDTO,
  DashboardDestinationDTO,
  DashboardDestinationSeriesDTO,
  AdminUserDTO,
  CreateAdminUserRequest,
  UpdateAdminUserRequest,
  ErrorResponse,
} from './types'

// ============================================================================
// Auth Endpoints
// ============================================================================

/**
 * POST /api/auth/login
 * Authenticate with email and password
 * Rate limit: 5 attempts per minute per IP
 */
export async function login(
  email: string,
  password: string
): Promise<AuthResponse> {
  const { data } = await client.post<AuthResponse>('/auth/login', {
    email,
    password,
  } as LoginRequest)
  return data
}

/**
 * POST /api/auth/logout
 * Revoke the current session on the server
 */
export async function logout(): Promise<void> {
  await client.post('/auth/logout')
}

/**
 * GET /api/auth/me
 * Get the current authenticated user's info
 * Requires valid session
 */
export async function getMe(): Promise<AuthResponse> {
  const { data } = await client.get<AuthResponse>('/auth/me')
  return data
}

// ============================================================================
// Server Endpoints
// ============================================================================

/**
 * GET /api/servers
 * Retrieve all configured servers
 */
export async function getServers(): Promise<ServerDTO[]> {
  const { data } = await client.get<ServerDTO[]>('/servers')
  return data
}

/**
 * POST /api/servers
 * Create a new server
 * Returns status="pending_key" (awaits SSH key generation)
 */
export async function postServer(req: UpsertServerRequest): Promise<ServerDTO> {
  const { data } = await client.post<ServerDTO>('/servers', req)
  return data
}

/**
 * GET /api/servers/{id}
 * Retrieve a specific server by ID
 */
export async function getServer(id: string): Promise<ServerDTO> {
  const { data } = await client.get<ServerDTO>(`/servers/${id}`)
  return data
}

/**
 * PUT /api/servers/{id}
 * Update an existing server
 */
export async function putServer(
  id: string,
  req: UpsertServerRequest
): Promise<ServerDTO> {
  const { data } = await client.put<ServerDTO>(`/servers/${id}`, req)
  return data
}

/**
 * DELETE /api/servers/{id}
 * Delete a server (and its associated records)
 */
export async function deleteServer(id: string): Promise<void> {
  await client.delete(`/servers/${id}`)
}

// ============================================================================
// Server SSH Operations (Fase 2)
// ============================================================================

/**
 * POST /api/servers/{id}/test-connection
 * Combined readiness check (RN-BACKUP-013): SSH+TOFU, the target Docker
 * container's running state, and — when storageTargetId is configured — the
 * storage target's credentials/container access.
 * On success (all applicable checks pass): transitions status to "ready"
 * and auto-enables if first time. On any failure: status is left
 * unchanged and lastTestConnectionError is populated with the composed
 * per-check reason. The `checks` field always reflects this call's result.
 */
export async function postTestConnection(
  id: string
): Promise<TestConnectionResult> {
  const { data } = await client.post<TestConnectionResult>(
    `/servers/${id}/test-connection`
  )
  return data
}

/**
 * POST /api/servers/{id}/regenerate-key
 * Generate a new SSH key pair
 * Resets status to "awaiting_authorization" and disables the server
 */
export async function postRegenerateKey(id: string): Promise<ServerDTO> {
  const { data } = await client.post<ServerDTO>(`/servers/${id}/regenerate-key`)
  return data
}

/**
 * POST /api/servers/{id}/reset-host-key
 * Clear the TOFU host key fingerprint (resets trust)
 * Requires re-testing the connection to re-pin the host key
 */
export async function postResetHostKey(id: string): Promise<ServerDTO> {
  const { data } = await client.post<ServerDTO>(`/servers/${id}/reset-host-key`)
  return data
}

/**
 * POST /api/servers/{id}/enable
 * Manually re-enables the server for automatic scheduling.
 */
export async function postEnableServer(id: string): Promise<ServerDTO> {
  const { data } = await client.post<ServerDTO>(`/servers/${id}/enable`)
  return data
}

/**
 * POST /api/servers/{id}/disable
 * Manually disables the server: the scheduler stops claiming it for
 * automatic runs, but manual backups (run-now/backup-now) remain available.
 */
export async function postDisableServer(id: string): Promise<ServerDTO> {
  const { data } = await client.post<ServerDTO>(`/servers/${id}/disable`)
  return data
}

// ============================================================================
// Backup Execution Endpoints
// ============================================================================

/**
 * POST /api/servers/{id}/run-now (Fase 4 — Async)
 * Enqueue a backup for async execution by the scheduler worker
 * Responds immediately with 202 Accepted
 * Prerequisites: status="ready" && enabled=true && storageTargetId configured
 */
export async function postRunNow(serverId: string): Promise<RunNowResponse> {
  const { data } = await client.post<RunNowResponse>(
    `/servers/${serverId}/run-now`
  )
  return data
}

/**
 * POST /api/servers/{id}/backup-now (Fase 3 — Sync)
 * Execute a backup synchronously
 * Responds with result (success or failure) once complete
 * Prerequisites: status="ready" && enabled=true && storageTargetId configured
 * dryRun: if true, retention plan is returned but not executed
 */
export async function postBackupNow(
  serverId: string,
  dryRun?: boolean
): Promise<BackupNowResponse> {
  const { data } = await client.post<BackupNowResponse>(
    `/servers/${serverId}/backup-now`,
    {
      dryRun: dryRun ?? false,
    }
  )
  return data
}

/**
 * GET /api/servers/{id}/backup-runs
 * Retrieve a paginated page of backup runs (executions) for a server, most
 * recent first, optionally filtered by status (RN-BACKUP-025).
 */
export async function getBackupRuns(
  serverId: string,
  opts?: {
    page?: number
    pageSize?: number
    status?: BackupRunDTO['status']
  }
): Promise<BackupRunListResponse> {
  const { data } = await client.get<BackupRunListResponse>(
    `/servers/${serverId}/backup-runs`,
    { params: opts }
  )
  return data
}

/**
 * GET /api/backup-runs
 * Retrieve a paginated page of backup runs across every server (or a single
 * one, when serverId is passed), most recent first, defaulting to page 1 /
 * 50 items with no server filter — the History screen's default view
 * (RN-BACKUP-026), which previously showed nothing until a server was
 * selected.
 */
export async function getAllBackupRuns(opts?: {
  page?: number
  pageSize?: number
  status?: BackupRunDTO['status']
  serverId?: string
}): Promise<BackupRunListResponse> {
  const { data } = await client.get<BackupRunListResponse>('/backup-runs', {
    params: opts,
  })
  return data
}

// ============================================================================
// Retention Policy Endpoints
// ============================================================================

/**
 * GET /api/retention-policy/default
 * Retrieve the global retention policy
 */
export async function getDefaultRetentionPolicy(): Promise<RetentionPolicyDTO> {
  const { data } = await client.get<RetentionPolicyDTO>(
    '/retention-policy/default'
  )
  return data
}

/**
 * PUT /api/retention-policy/default
 * Update the global retention policy
 * Both recentCount and monthlyCount cannot be zero
 */
export async function putDefaultRetentionPolicy(
  policy: Omit<RetentionPolicyDTO, 'id' | 'serverId'>
): Promise<RetentionPolicyDTO> {
  const { data } = await client.put<RetentionPolicyDTO>(
    '/retention-policy/default',
    policy
  )
  return data
}

/**
 * GET /api/servers/{id}/retention-policy
 * Retrieve the effective retention policy for a server
 * Returns server override if set, otherwise global default
 */
export async function getServerRetentionPolicy(
  serverId: string
): Promise<RetentionPolicyDTO> {
  const { data } = await client.get<RetentionPolicyDTO>(
    `/servers/${serverId}/retention-policy`
  )
  return data
}

/**
 * PUT /api/servers/{id}/retention-policy
 * Set a per-server retention policy override
 * Omit both to use the global policy
 */
export async function putServerRetentionPolicy(
  serverId: string,
  policy: Omit<RetentionPolicyDTO, 'id' | 'serverId'>
): Promise<RetentionPolicyDTO> {
  const { data } = await client.put<RetentionPolicyDTO>(
    `/servers/${serverId}/retention-policy`,
    policy
  )
  return data
}

/**
 * DELETE /api/servers/{id}/retention-policy
 * Remove the per-server retention policy override
 * Falls back to global default
 */
export async function deleteServerRetentionPolicy(
  serverId: string
): Promise<void> {
  await client.delete(`/servers/${serverId}/retention-policy`)
}

// ============================================================================
// Storage Targets Endpoints
// ============================================================================

/**
 * GET /api/storage-targets
 * Retrieve all storage targets (Azure Blob Storage, S3-compatible, or
 * local/NFS filesystem)
 */
export async function getStorageTargets(): Promise<StorageTargetDTO[]> {
  const { data } = await client.get<StorageTargetDTO[]>('/storage-targets')
  return data
}

/**
 * POST /api/storage-targets
 * Create a new storage target
 * Secrets (SAS token / S3 secret access key) are encrypted and never returned
 */
export async function postStorageTarget(
  req: UpsertStorageTargetRequest
): Promise<StorageTargetDTO> {
  const { data } = await client.post<StorageTargetDTO>('/storage-targets', req)
  return data
}

/**
 * GET /api/storage-targets/{id}
 * Retrieve a specific storage target by ID
 */
export async function getStorageTarget(id: string): Promise<StorageTargetDTO> {
  const { data } = await client.get<StorageTargetDTO>(`/storage-targets/${id}`)
  return data
}

/**
 * PUT /api/storage-targets/{id}
 * Update a storage target
 * Secrets can be omitted to keep the current one
 */
export async function putStorageTarget(
  id: string,
  req: UpsertStorageTargetRequest
): Promise<StorageTargetDTO> {
  const { data } = await client.put<StorageTargetDTO>(
    `/storage-targets/${id}`,
    req
  )
  return data
}

/**
 * DELETE /api/storage-targets/{id}
 * Delete a storage target (and any server references to it)
 */
export async function deleteStorageTarget(id: string): Promise<void> {
  await client.delete(`/storage-targets/${id}`)
}

// ============================================================================
// Admin User Endpoints
// ============================================================================

/**
 * GET /api/admin-users
 * Retrieve all admin (operator) accounts
 */
export async function getAdminUsers(): Promise<AdminUserDTO[]> {
  const { data } = await client.get<AdminUserDTO[]>('/admin-users')
  return data
}

/**
 * POST /api/admin-users
 * Create a new admin account. Password must be at least 12 characters.
 */
export async function postAdminUser(
  req: CreateAdminUserRequest
): Promise<AdminUserDTO> {
  const { data } = await client.post<AdminUserDTO>('/admin-users', req)
  return data
}

/**
 * GET /api/admin-users/{id}
 * Retrieve a specific admin account by ID
 */
export async function getAdminUser(id: string): Promise<AdminUserDTO> {
  const { data } = await client.get<AdminUserDTO>(`/admin-users/${id}`)
  return data
}

/**
 * PUT /api/admin-users/{id}
 * Update an admin account. Password can be omitted to keep the current one.
 */
export async function putAdminUser(
  id: string,
  req: UpdateAdminUserRequest
): Promise<AdminUserDTO> {
  const { data } = await client.put<AdminUserDTO>(`/admin-users/${id}`, req)
  return data
}

/**
 * DELETE /api/admin-users/{id}
 * Delete an admin account. Rejected with 409 for self-deletion or when
 * deleting the last remaining admin (RN-AUTH-002, RN-AUTH-003).
 */
export async function deleteAdminUser(id: string): Promise<void> {
  await client.delete(`/admin-users/${id}`)
}

// ============================================================================
// Dashboard Endpoints
// ============================================================================

/**
 * GET /api/dashboard/summary
 * Retrieve aggregate KPIs, a rolling backup-runs window, and attention
 * points (servers awaiting authorization, servers with a failed connection
 * test, recent failed backup runs) for the analytics dashboard.
 */
export async function getDashboardSummary(): Promise<DashboardSummaryDTO> {
  const { data } = await client.get<DashboardSummaryDTO>('/dashboard/summary')
  return data
}

/**
 * GET /api/dashboard/backup-stats
 * Retrieve per-destination backup counts and byte sums, bucketed daily over
 * the last 30 days and monthly over the current year, feeding the
 * dashboard's 4 stacked bar charts.
 */
export async function getDashboardBackupStats(): Promise<DashboardBackupStatsDTO> {
  const { data } = await client.get<DashboardBackupStatsDTO>(
    '/dashboard/backup-stats'
  )
  return data
}

// ============================================================================
// Re-exports for convenient importing
// ============================================================================

export type {
  LoginRequest,
  AuthResponse,
  ServerDTO,
  UpsertServerRequest,
  BackupRunDTO,
  BackupRunListResponse,
  CheckResult,
  TestConnectionChecks,
  TestConnectionResult,
  RunNowResponse,
  BackupNowResponse,
  StorageTargetDTO,
  UpsertStorageTargetRequest,
  RetentionPolicyDTO,
  DashboardSummaryDTO,
  DashboardBackupStatsDTO,
  DashboardDestinationDTO,
  DashboardDestinationSeriesDTO,
  AdminUserDTO,
  CreateAdminUserRequest,
  UpdateAdminUserRequest,
  ErrorResponse,
}
