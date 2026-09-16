import axios from 'axios'
import type { ErrorResponse } from '@/api'

/**
 * Extracts the backend's { error: string } message from an Axios error,
 * falling back to a generic message for anything else (network error,
 * unexpected shape). Used where the exact backend message matters to the
 * user — e.g. the self-delete / last-admin guards on admin_users, or a
 * duplicate email/cpf on create.
 */
export function apiErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError<ErrorResponse>(err) && err.response?.data?.error) {
    return err.response.data.error
  }
  return fallback
}

/**
 * Dev-only console logging for a failed API call, safe to use even on forms
 * that submit a password: an Axios error's `.config.data` is the raw request
 * body, so logging the full error object (as the generic
 * `console.error('[DEBUG] API Error:', err)` pattern used elsewhere does)
 * would print a plaintext password to the browser console on admin_users
 * create/update failures. This logs only the response status and message.
 */
export function logApiError(context: string, err: unknown): void {
  if (!import.meta.env.DEV) return
  if (axios.isAxiosError<ErrorResponse>(err)) {
    console.error(`[DEBUG] ${context}:`, {
      status: err.response?.status,
      error: err.response?.data?.error ?? err.message,
    })
    return
  }
  console.error(`[DEBUG] ${context}:`, err instanceof Error ? err.message : err)
}
