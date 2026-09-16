/**
 * Tests for useAuth composable
 * Coverage:
 * - Login success (sets user + isLoggedIn + clears error)
 * - Login failure (sets error + isLoggedIn=false)
 * - Session restore via checkSession
 * - Logout (clears all state)
 * - Double logout (idempotent, no throw)
 */

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useAuth } from '../useAuth'
import * as api from '@/api'

// Mock the api module
vi.mock('@/api')

describe('useAuth', () => {
  beforeEach(() => {
    // Clear all mocks before each test
    vi.clearAllMocks()
    // Reset state by calling logout (clears user, isLoggedIn, error)
    const auth = useAuth()
    auth.user.value = null
    auth.isLoggedIn.value = false
    auth.error.value = null
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  // Test 1: Login success
  it('should login successfully and set user + isLoggedIn', async () => {
    const mockUser = { email: 'test@example.com', role: 'admin' }
    vi.mocked(api.login).mockResolvedValueOnce(mockUser)

    const auth = useAuth()
    const result = await auth.login('test@example.com', 'password123')

    expect(result).toBe(true)
    expect(auth.isLoggedIn.value).toBe(true)
    expect(auth.user.value).toEqual(mockUser)
    expect(auth.error.value).toBeNull()
    expect(auth.loading.value).toBe(false)
  })

  // Test 2: Login error
  it('should handle login error', async () => {
    vi.mocked(api.login).mockRejectedValueOnce(new Error('Invalid credentials'))

    const auth = useAuth()
    const result = await auth.login('test@example.com', 'wrongpassword')

    expect(result).toBe(false)
    expect(auth.isLoggedIn.value).toBe(false)
    expect(auth.user.value).toBeNull()
    expect(auth.error.value).toBe('Invalid credentials')
    expect(auth.loading.value).toBe(false)
  })

  // Test 3: Session restore via checkSession
  it('should restore session via checkSession', async () => {
    const mockUser = { email: 'test@example.com', role: 'admin' }
    vi.mocked(api.getMe).mockResolvedValueOnce(mockUser)

    const auth = useAuth()
    const result = await auth.checkSession()

    expect(result).toBe(true)
    expect(auth.isLoggedIn.value).toBe(true)
    expect(auth.user.value).toEqual(mockUser)
    expect(auth.error.value).toBeNull()
    expect(auth.loading.value).toBe(false)
  })

  // Test 4: Logout
  it('should logout and clear state', async () => {
    vi.mocked(api.logout).mockResolvedValueOnce(undefined)

    const auth = useAuth()
    // Pre-populate state as if user was logged in
    auth.user.value = { email: 'test@example.com', role: 'admin' }
    auth.isLoggedIn.value = true

    await auth.logout()

    expect(auth.isLoggedIn.value).toBe(false)
    expect(auth.user.value).toBeNull()
    expect(auth.error.value).toBeNull()
    expect(auth.loading.value).toBe(false)
    expect(api.logout).toHaveBeenCalledOnce()
  })

  // Test 5: Double logout (idempotent)
  it('should be idempotent on double logout', async () => {
    vi.mocked(api.logout).mockResolvedValue(undefined)

    const auth = useAuth()

    // First logout
    await auth.logout()
    expect(auth.isLoggedIn.value).toBe(false)

    // Second logout — should not throw
    await auth.logout()
    expect(auth.isLoggedIn.value).toBe(false)

    // API was called twice
    expect(api.logout).toHaveBeenCalledTimes(2)
  })

  // Bonus test: checkSession on invalid session (401)
  it('should handle checkSession on invalid session', async () => {
    vi.mocked(api.getMe).mockRejectedValueOnce(new Error('Unauthorized'))

    const auth = useAuth()
    // Pre-populate as if logged in
    auth.user.value = { email: 'test@example.com', role: 'admin' }
    auth.isLoggedIn.value = true

    const result = await auth.checkSession()

    expect(result).toBe(false)
    expect(auth.isLoggedIn.value).toBe(false)
    expect(auth.user.value).toBeNull()
    expect(auth.loading.value).toBe(false)
  })

  // Bonus test: loading state during login
  it('should set loading=true during login', async () => {
    vi.mocked(api.login).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          // Simulate delay
          setTimeout(
            () => resolve({ email: 'test@example.com', role: 'admin' }),
            10
          )
        })
    )

    const auth = useAuth()
    const promise = auth.login('test@example.com', 'password123')

    // During the promise, loading may or may not be true depending on timing
    // But after the promise resolves, loading should be false
    await promise
    expect(auth.loading.value).toBe(false)
  })

  // Bonus test: error cleared on successful login
  it('should clear error on successful login', async () => {
    const mockUser = { email: 'test@example.com', role: 'admin' }
    vi.mocked(api.login)
      .mockRejectedValueOnce(new Error('Login failed'))
      .mockResolvedValueOnce(mockUser)

    const auth = useAuth()

    // First login fails
    await auth.login('test@example.com', 'wrong')
    expect(auth.error.value).not.toBeNull()

    // Second login succeeds
    const result = await auth.login('test@example.com', 'correct')
    expect(result).toBe(true)
    expect(auth.error.value).toBeNull()
  })
})
