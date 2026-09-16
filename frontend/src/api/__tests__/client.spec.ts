/**
 * Tests for API client interceptors
 * Verifies:
 * 1. CSRF token injection from the `backapeando_backup_csrf` cookie into the
 *    X-CSRF-Token request header
 * 2. Malformed CSRF cookie values are ignored (not injected)
 * 3. 401 logout event dispatch (excluding the auth endpoints themselves)
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import client from '../client'

const VALID_TOKEN = 'a'.repeat(43) // matches /^[A-Za-z0-9_-]{43}$/

function setDocumentCookie(value: string): void {
  document.cookie = value
}

describe('API Client', () => {
  beforeEach(() => {
    // `document.cookie` is redefined as a plain writable data property so
    // each test controls its full value directly, instead of relying on
    // the test environment's real cookie-jar append/expiry semantics.
    Object.defineProperty(document, 'cookie', {
      value: '',
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    delete (document as unknown as { cookie?: string }).cookie
    vi.restoreAllMocks()
  })

  describe('CSRF Token Injection (request interceptor)', () => {
    it('should inject a valid CSRF token from the cookie into the X-CSRF-Token header', async () => {
      setDocumentCookie(`backapeando_backup_csrf=${VALID_TOKEN}`)

      const requestInterceptor = client.interceptors.request.handlers?.[0]
      expect(requestInterceptor).toBeDefined()

      const config = await requestInterceptor!.fulfilled({
        headers: {},
      } as any)

      expect(config.headers['X-CSRF-Token']).toBe(VALID_TOKEN)
    })

    it('should not add the header when there is no CSRF cookie', async () => {
      setDocumentCookie('')

      const requestInterceptor = client.interceptors.request.handlers?.[0]
      const config = await requestInterceptor!.fulfilled({
        headers: {},
      } as any)

      expect(config.headers['X-CSRF-Token']).toBeUndefined()
    })

    it('should ignore a malformed CSRF cookie value instead of injecting it', async () => {
      setDocumentCookie('backapeando_backup_csrf=not-a-valid-token')
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

      const requestInterceptor = client.interceptors.request.handlers?.[0]
      const config = await requestInterceptor!.fulfilled({
        headers: {},
      } as any)

      expect(config.headers['X-CSRF-Token']).toBeUndefined()
      expect(warnSpy).toHaveBeenCalled()
    })

    it('should pick the CSRF cookie out from among other cookies', async () => {
      setDocumentCookie(
        `other=1; backapeando_backup_csrf=${VALID_TOKEN}; another=2`
      )

      const requestInterceptor = client.interceptors.request.handlers?.[0]
      const config = await requestInterceptor!.fulfilled({
        headers: {},
      } as any)

      expect(config.headers['X-CSRF-Token']).toBe(VALID_TOKEN)
    })
  })

  describe('401 Logout Event (response interceptor)', () => {
    it('should dispatch auth:logout event on a 401 from a non-auth endpoint', async () => {
      const dispatchSpy = vi.spyOn(window, 'dispatchEvent')
      const responseInterceptor = client.interceptors.response.handlers?.[0]
      expect(responseInterceptor).toBeDefined()

      const error = {
        isAxiosError: true,
        response: { status: 401 },
        config: { url: '/servers' },
      }

      await expect(responseInterceptor!.rejected!(error)).rejects.toBe(error)
      expect(dispatchSpy).toHaveBeenCalledWith(expect.any(CustomEvent))
      const dispatched = dispatchSpy.mock.calls.find(
        (call) => (call[0] as CustomEvent).type === 'auth:logout'
      )
      expect(dispatched).toBeDefined()
    })

    it('should not dispatch auth:logout on a 401 from /auth/login', async () => {
      const dispatchSpy = vi.spyOn(window, 'dispatchEvent')
      const responseInterceptor = client.interceptors.response.handlers?.[0]

      const error = {
        isAxiosError: true,
        response: { status: 401 },
        config: { url: '/auth/login' },
      }

      await expect(responseInterceptor!.rejected!(error)).rejects.toBe(error)
      expect(dispatchSpy).not.toHaveBeenCalledWith(
        expect.objectContaining({ type: 'auth:logout' })
      )
    })

    it('should not dispatch auth:logout on a 401 from /auth/logout', async () => {
      const dispatchSpy = vi.spyOn(window, 'dispatchEvent')
      const responseInterceptor = client.interceptors.response.handlers?.[0]

      const error = {
        isAxiosError: true,
        response: { status: 401 },
        config: { url: '/auth/logout' },
      }

      await expect(responseInterceptor!.rejected!(error)).rejects.toBe(error)
      expect(dispatchSpy).not.toHaveBeenCalledWith(
        expect.objectContaining({ type: 'auth:logout' })
      )
    })

    it('should not dispatch auth:logout on non-401 errors', async () => {
      const dispatchSpy = vi.spyOn(window, 'dispatchEvent')
      const responseInterceptor = client.interceptors.response.handlers?.[0]

      const error = {
        isAxiosError: true,
        response: { status: 500 },
        config: { url: '/servers' },
      }

      await expect(responseInterceptor!.rejected!(error)).rejects.toBe(error)
      expect(dispatchSpy).not.toHaveBeenCalledWith(
        expect.objectContaining({ type: 'auth:logout' })
      )
    })
  })

  describe('Client Configuration', () => {
    it('should have withCredentials enabled', () => {
      expect(client.defaults.withCredentials).toBe(true)
    })

    it('should have correct baseURL with /api path', () => {
      const baseURL = client.defaults.baseURL
      expect(baseURL).toBeDefined()
      expect(baseURL).toContain('/api')
    })

    it('should have proper timeout set', () => {
      expect(client.defaults.timeout).toBe(30000)
    })
  })

  describe('Interceptor Setup', () => {
    it('should have exactly one request interceptor configured', () => {
      const handlers = client.interceptors.request.handlers
      expect(handlers).toBeDefined()
      if (handlers) {
        expect(handlers.length).toBe(1)
      }
    })

    it('should have exactly one response interceptor configured', () => {
      const handlers = client.interceptors.response.handlers
      expect(handlers).toBeDefined()
      if (handlers) {
        expect(handlers.length).toBe(1)
      }
    })
  })
})
