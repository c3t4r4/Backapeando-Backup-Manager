/**
 * Axios HTTP client with interceptors for Backapeando API
 * Handles:
 * - CSRF token injection (request header, read from the CSRF cookie)
 * - 401 logout event dispatch
 */

import axios, {
  AxiosInstance,
  AxiosError,
  AxiosResponse,
  InternalAxiosRequestConfig,
} from 'axios'

// `??` (not `||`) so an explicitly empty VITE_API_URL (Docker build, same-origin
// via the nginx /api/ proxy) is respected instead of falling back — only an
// actually-unset var (local `npm run dev` without VITE_API_URL) uses the default.
const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8081'

/**
 * Main HTTP client instance
 * All requests include credentials (cookies) automatically
 */
const client: AxiosInstance = axios.create({
  baseURL: `${API_URL}/api`,
  withCredentials: true, // Include cookies in every request
  timeout: 30000, // 30s default timeout
})

/**
 * Read the CSRF token straight from the (non-HttpOnly) cookie the server
 * sets on login. This is the double-submit pattern the backend actually
 * implements (`backend/internal/auth/csrf.go`): the server never sends the
 * token in a response header, only as a cookie readable by same-origin JS.
 */
function getCsrfTokenFromCookie(): string | null {
  if (typeof document === 'undefined') {
    return null
  }
  const match = document.cookie.match(/(?:^|; )backapeando_backup_csrf=([^;]*)/)
  return match ? decodeURIComponent(match[1]) : null
}

/**
 * Request interceptor: Inject CSRF token read from the cookie
 * Validates token format before injection (base64url 32-byte = 43 chars)
 * Header: X-CSRF-Token
 * Cookie: backapeando_backup_csrf (set by server, HttpOnly=false by design)
 */
client.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const csrfToken = getCsrfTokenFromCookie()

  if (csrfToken) {
    // Validate token format: base64url-encoded 32-byte value (43 chars)
    if (/^[A-Za-z0-9_-]{43}$/.test(csrfToken)) {
      config.headers['X-CSRF-Token'] = csrfToken
    } else {
      console.warn('[SECURITY] Malformed CSRF token in cookie, ignoring')
    }
  }
  return config
})

/**
 * Endpoints excluded from the 401 auto-logout handling below.
 * A 401 from these is an expected outcome of the auth flow itself
 * (wrong credentials, already-logged-out session) — not a signal that a
 * previously valid session expired mid-request.
 */
const AUTH_ENDPOINTS = ['/auth/login', '/auth/logout']

/**
 * Error interceptor: Handle 401 Unauthorized
 * Dispatches 'auth:logout' event so useAuth composable can clean up
 */
client.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error: unknown) => {
    if (
      axios.isAxiosError(error) &&
      error.response?.status === 401 &&
      !AUTH_ENDPOINTS.includes(error.config?.url ?? '')
    ) {
      // Dispatch custom event for auth composable to listen
      window.dispatchEvent(new CustomEvent('auth:logout'))
    }
    return Promise.reject(error)
  }
)

export default client
