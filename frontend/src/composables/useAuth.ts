/**
 * useAuth composable
 * Manages authentication state and session lifecycle
 *
 * Features:
 * - Reactive auth state (user, isLoggedIn, loading, error)
 * - Login/logout/checkSession functions
 * - Auto-logout on 401 (listens to 'auth:logout' event from axios interceptor)
 *
 * Usage:
 * ```ts
 * const auth = useAuth()
 * const success = await auth.login('user@example.com', 'password')
 * if (success) {
 *   console.log('Logged in as:', auth.user?.email)
 * }
 * ```
 */

import { ref } from 'vue'
import * as api from '@/api'
import type { AuthResponse } from '@/api'

/**
 * Shared reactive state (singleton pattern)
 * All useAuth() calls share the same state instance
 */
const user = ref<AuthResponse | null>(null)
const isLoggedIn = ref(false)
const loading = ref(false)
const error = ref<string | null>(null)

/**
 * Auto-logout listener registration (once per app lifetime)
 */
let autoLogoutListenerRegistered = false

/**
 * useAuth composable
 * Returns reactive auth state and functions
 */
export function useAuth() {
  /**
   * POST /api/auth/login
   * Authenticate user with email/password
   * Sets user and isLoggedIn on success
   * Sets error on failure
   *
   * @param email User email
   * @param password User password
   * @returns true if login successful, false otherwise
   */
  async function login(
    emailParam: string,
    passwordParam: string
  ): Promise<boolean> {
    loading.value = true
    error.value = null

    try {
      const responseUser = await api.login(emailParam, passwordParam)
      user.value = responseUser
      isLoggedIn.value = true
      return true
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Login failed'
      error.value = errorMessage
      isLoggedIn.value = false
      user.value = null
      return false
    } finally {
      loading.value = false
    }
  }

  /**
   * POST /api/auth/logout
   * Revoke the current session on server
   * Clears user and isLoggedIn state
   *
   * Idempotent: safe to call multiple times
   */
  async function logout(): Promise<void> {
    loading.value = true

    try {
      await api.logout()
    } catch (err) {
      // Log but don't throw — logout should always clear local state
      // even if server fails (network error, session already expired, etc.)
      console.error('Logout API call failed:', err)
    } finally {
      user.value = null
      isLoggedIn.value = false
      error.value = null
      loading.value = false
    }
  }

  /**
   * GET /api/auth/me
   * Check if user has valid session
   * Restores user if session is valid
   * Clears state if session is invalid (401)
   *
   * @returns true if session valid, false otherwise
   */
  async function checkSession(): Promise<boolean> {
    loading.value = true

    try {
      const responseUser = await api.getMe()
      user.value = responseUser
      isLoggedIn.value = true
      error.value = null
      return true
    } catch (err) {
      // Session expired or invalid
      isLoggedIn.value = false
      user.value = null
      // Don't set error state for checkSession — it's not a user action
      return false
    } finally {
      loading.value = false
    }
  }

  /**
   * Handle auto-logout event from axios interceptor on 401
   * CSRF token lives only in the cookie (never cached client-side), so
   * there's nothing to clear here beyond calling logout()
   */
  async function handleAutoLogout(): Promise<void> {
    await logout()
  }

  /**
   * Register auto-logout listener (once per app)
   * Listens for 'auth:logout' custom event dispatched by axios interceptor on 401
   * Calls logout() to clear local state
   */
  function registerAutoLogoutListener(): void {
    if (autoLogoutListenerRegistered || typeof window === 'undefined') {
      return
    }

    window.addEventListener('auth:logout', handleAutoLogout)
    autoLogoutListenerRegistered = true
  }

  // Register listener on first useAuth call
  registerAutoLogoutListener()

  return {
    // Reactive state (refs)
    user,
    isLoggedIn,
    loading,
    error,

    // Methods
    login,
    logout,
    checkSession,
  }
}
