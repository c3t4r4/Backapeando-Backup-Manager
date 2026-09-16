/**
 * Vue Router guard tests
 *
 * Tests 8+ scenarios:
 * 1. Public route `/login` accessible without authentication
 * 2. Protected route `/dashboard` redirects to `/login` without authentication
 * 3. Protected route `/dashboard` accessible with authentication
 * 4. Authenticated user visiting `/login` redirects to `/dashboard`
 * 5. Root `/` redirects based on auth state (to `/login` or `/dashboard`)
 * 6. After logout, `/dashboard` is blocked again
 * 7. Router initializes without errors
 * 8. Multiple protected routes block correctly
 */

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { authGuard } from '@/router/index'
import LoginView from '@/views/LoginView.vue'
import DashboardView from '@/views/DashboardView.vue'
import ServersView from '@/views/ServersView.vue'
import HistoryView from '@/views/HistoryView.vue'
import ServerNewView from '@/views/ServerNewView.vue'
import ServerEditView from '@/views/ServerEditView.vue'
import StorageTargetsView from '@/views/StorageTargetsView.vue'
import StorageTargetNewView from '@/views/StorageTargetNewView.vue'
import StorageTargetEditView from '@/views/StorageTargetEditView.vue'

/**
 * Mock useAuth composable
 * Returns a ref-like object that can be controlled in tests
 */
vi.mock('@/composables/useAuth', () => {
  const isLoggedInValue = { value: false }

  return {
    useAuth: () => ({
      isLoggedIn: isLoggedInValue,
      user: { value: null },
      loading: { value: false },
      error: { value: null },
      login: vi.fn(),
      logout: vi.fn(),
      checkSession: vi.fn(),
      _testSetLoggedIn: (value: boolean) => {
        isLoggedInValue.value = value
      },
    }),
  }
})

/**
 * Create test router instance with same routes as production
 */
function createTestRouter() {
  const routes: RouteRecordRaw[] = [
    {
      path: '/',
      redirect: () => {
        const { isLoggedIn } = useAuth()
        return isLoggedIn.value ? '/dashboard' : '/login'
      },
    },
    {
      path: '/login',
      component: LoginView,
      name: 'login',
      meta: {
        requiresAuth: false,
      },
    },
    {
      path: '/dashboard',
      component: DashboardView,
      name: 'dashboard',
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/servers',
      component: ServersView,
      name: 'servers',
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/history',
      component: HistoryView,
      name: 'history',
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/settings',
      component: {
        template: '<div>Settings</div>',
      },
      name: 'settings',
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/servers/new',
      component: ServerNewView,
      name: 'server-new',
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/servers/:id/edit',
      component: ServerEditView,
      name: 'server-edit',
      props: true,
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/storage-targets',
      component: StorageTargetsView,
      name: 'storage-targets',
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/storage-targets/new',
      component: StorageTargetNewView,
      name: 'storage-target-new',
      meta: {
        requiresAuth: true,
      },
    },
    {
      path: '/storage-targets/:id/edit',
      component: StorageTargetEditView,
      name: 'storage-target-edit',
      props: true,
      meta: {
        requiresAuth: true,
      },
    },
  ]

  const testRouter = createRouter({
    history: createMemoryHistory(),
    routes,
  })

  // Real guard from the production router — not a hand-duplicated copy —
  // so this test suite can't silently drift from actual guard behavior.
  testRouter.beforeEach(authGuard)

  return testRouter
}

describe('Vue Router Guards', () => {
  let router: ReturnType<typeof createTestRouter>
  let auth: any

  beforeEach(async () => {
    router = createTestRouter()
    auth = useAuth()
    // Reset to logged out state
    auth._testSetLoggedIn(false)
  })

  /**
   * Test 1: Public route `/login` accessible without authentication
   */
  it('should allow access to /login without authentication', async () => {
    await router.push('/login')
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.name).toBe('login')
  })

  /**
   * Test 2: Protected route `/dashboard` redirects to `/login` without authentication
   */
  it('should redirect from /dashboard to /login without authentication', async () => {
    await router.push('/dashboard')
    // Guard should redirect to login
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.name).toBe('login')
  })

  /**
   * Test 3: Protected route `/dashboard` accessible with authentication
   */
  it('should allow access to /dashboard with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  /**
   * Test 4: Authenticated user visiting `/login` redirects to `/dashboard`
   */
  it('should redirect from /login to /dashboard when authenticated', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/login')
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(router.currentRoute.value.name).toBe('dashboard')
  })

  /**
   * Test 5: Root `/` redirects to `/login` or `/dashboard` based on auth state
   */
  it('should redirect from / to /login when not authenticated', async () => {
    auth._testSetLoggedIn(false)
    await router.push('/')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('should redirect from / to /dashboard when authenticated', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  /**
   * Test 6: After logout, protected routes are inaccessible
   * Test the sequence: authenticate → access → logout → block
   */
  it('should require new auth after logout for protected routes', async () => {
    // Authenticate and access
    auth._testSetLoggedIn(true)
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')

    // Logout (clear auth)
    auth._testSetLoggedIn(false)

    // Try to navigate to another protected route while logged out
    // This simulates the logout cleanup flow
    await router.push('/history')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  /**
   * Test 7: Router initializes without errors
   */
  it('should initialize router without errors', () => {
    expect(router).toBeDefined()
    const routes = router.getRoutes()
    expect(routes).toBeDefined()
    expect(routes.length).toBeGreaterThan(0)
    // Should have at least: /, /login, /dashboard, /history, /settings
    expect(routes.length).toBeGreaterThanOrEqual(5)
  })

  /**
   * Test: /servers (dedicated server management screen) is protected
   */
  it('should redirect from /servers to /login without authentication', async () => {
    auth._testSetLoggedIn(false)
    await router.push('/servers')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('should allow access to /servers with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/servers')
    expect(router.currentRoute.value.path).toBe('/servers')
    expect(router.currentRoute.value.name).toBe('servers')
  })

  /**
   * Test 8: Multiple protected routes (`/history`, `/settings`) block correctly
   */
  it('should redirect from /history to /login without authentication', async () => {
    auth._testSetLoggedIn(false)
    await router.push('/history')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('should allow access to /history with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/history')
    expect(router.currentRoute.value.path).toBe('/history')
  })

  it('should redirect from /settings to /login without authentication', async () => {
    auth._testSetLoggedIn(false)
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('should allow access to /settings with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/settings')
  })

  /**
   * Test 9: /servers/new and /servers/:id/edit are protected routes
   */
  it('should redirect from /servers/new to /login without authentication', async () => {
    auth._testSetLoggedIn(false)
    await router.push('/servers/new')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('should allow access to /servers/new with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/servers/new')
    expect(router.currentRoute.value.path).toBe('/servers/new')
    expect(router.currentRoute.value.name).toBe('server-new')
  })

  it('should redirect from /servers/:id/edit to /login without authentication', async () => {
    auth._testSetLoggedIn(false)
    await router.push('/servers/abc-123/edit')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('should allow access to /servers/:id/edit with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/servers/abc-123/edit')
    expect(router.currentRoute.value.path).toBe('/servers/abc-123/edit')
    expect(router.currentRoute.value.name).toBe('server-edit')
    expect(router.currentRoute.value.params.id).toBe('abc-123')
  })

  /**
   * Test 10: /storage-targets, /storage-targets/new, /storage-targets/:id/edit are protected routes
   */
  it('should redirect from /storage-targets to /login without authentication', async () => {
    auth._testSetLoggedIn(false)
    await router.push('/storage-targets')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('should allow access to /storage-targets with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/storage-targets')
    expect(router.currentRoute.value.path).toBe('/storage-targets')
    expect(router.currentRoute.value.name).toBe('storage-targets')
  })

  it('should allow access to /storage-targets/new with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/storage-targets/new')
    expect(router.currentRoute.value.path).toBe('/storage-targets/new')
    expect(router.currentRoute.value.name).toBe('storage-target-new')
  })

  it('should allow access to /storage-targets/:id/edit with authentication', async () => {
    auth._testSetLoggedIn(true)
    await router.push('/storage-targets/xyz-789/edit')
    expect(router.currentRoute.value.path).toBe('/storage-targets/xyz-789/edit')
    expect(router.currentRoute.value.name).toBe('storage-target-edit')
    expect(router.currentRoute.value.params.id).toBe('xyz-789')
  })

  /**
   * Additional test: Multiple route transitions work correctly
   */
  it('should handle multiple route transitions correctly', async () => {
    // Navigate to login first (not authenticated)
    await router.push('/login')
    expect(router.currentRoute.value.path).toBe('/login')

    // Try to access protected route (should fail and redirect to login)
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/login')

    // Authenticate
    auth._testSetLoggedIn(true)

    // Now access dashboard (should succeed)
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/dashboard')

    // Access history (should succeed)
    await router.push('/history')
    expect(router.currentRoute.value.path).toBe('/history')

    // Try to access login (should redirect to dashboard)
    await router.push('/login')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })
})
