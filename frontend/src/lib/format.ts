/**
 * Format ISO 8601 date string to locale-friendly format.
 * Returns "—" if the date is missing.
 */
export function formatDate(dateStr?: string): string {
  if (!dateStr) return '—'

  try {
    const date = new Date(dateStr)
    return date.toLocaleString('pt-BR', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return dateStr
  }
}

/**
 * Format bytes to human-readable size (B, KB, MB, GB).
 * Returns "—" if the value is missing.
 */
export function formatSize(bytes?: number): string {
  if (bytes === undefined || bytes === null) return '—'

  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unitIndex = 0

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }

  return `${size.toFixed(2)} ${units[unitIndex]}`
}

/**
 * Format a duration in milliseconds to a human-readable string (ms or s).
 * Returns "—" if the value is missing.
 */
export function formatDurationMs(ms?: number): string {
  if (ms === undefined || ms === null) return '—'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

/**
 * Human-readable (pt-BR) label for a backup run status.
 */
export function formatBackupStatus(status: string): string {
  return (
    {
      queued: 'Enfileirado',
      running: 'Executando',
      success: 'Sucesso',
      failed: 'Falhou',
    }[status] || status
  )
}

/**
 * Format a server UUID for display: last 8 chars, uppercased, for brevity.
 */
export function formatServerId(serverId: string): string {
  return serverId.slice(-8).toUpperCase()
}

/**
 * Format an 11-digit CPF (already validated/normalized by the backend) as
 * "000.000.000-00" for display. Returns the raw value unchanged if it isn't
 * exactly 11 digits — display-only, never used for validation.
 */
export function formatCPF(cpf?: string): string {
  if (!cpf) return '—'
  const digits = cpf.replace(/\D/g, '')
  if (digits.length !== 11) return cpf
  return `${digits.slice(0, 3)}.${digits.slice(3, 6)}.${digits.slice(6, 9)}-${digits.slice(9)}`
}
