/**
 * Tests for App.vue
 * Coverage:
 * - Passes the real authenticated user down to Layout/Navbar (regression
 *   test for the "Not logged in" bug: App.vue used to render <Layout>
 *   with no `user` prop at all, so Navbar always fell into its logged-out
 *   branch regardless of actual session state)
 * - Renders Layout only on routes with meta.requiresAuth
 * - Wires the logout event through to useAuth().logout() + redirects to /login
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import App from './App.vue'

const mockLogout = vi.fn().mockResolvedValue(undefined)
const mockUser = ref<{ email: string; role: string } | null>({
  email: 'user@example.com',
  role: 'admin',
})
const mockLoading = ref(false)

vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({
    user: mockUser,
    loading: mockLoading,
    logout: mockLogout,
  }),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/dashboard',
        component: { template: '<div data-testid="page">Dashboard</div>' },
        meta: { requiresAuth: true },
      },
      {
        path: '/login',
        component: { template: '<div data-testid="page">Login</div>' },
        meta: { requiresAuth: false },
      },
    ],
  })
}

describe('App', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUser.value = { email: 'user@example.com', role: 'admin' }
    mockLoading.value = false
  })

  it('should pass the real user down to Navbar on a protected route', async () => {
    const router = createTestRouter()
    await router.push('/dashboard')
    await router.isReady()

    const wrapper = mount(App, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('user@example.com')
    expect(wrapper.text()).not.toContain('Not logged in')
  })

  it('should show "Not logged in" when there is no authenticated user', async () => {
    mockUser.value = null

    const router = createTestRouter()
    await router.push('/dashboard')
    await router.isReady()

    const wrapper = mount(App, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.text()).toContain('Not logged in')
  })

  it('should not render Layout/Navbar on a public route', async () => {
    const router = createTestRouter()
    await router.push('/login')
    await router.isReady()

    const wrapper = mount(App, { global: { plugins: [router] } })
    await flushPromises()

    expect(wrapper.text()).not.toContain('Not logged in')
    expect(wrapper.text()).not.toContain('user@example.com')
    expect(wrapper.find('[data-testid="page"]').text()).toBe('Login')
  })

  it('should call logout() and redirect to /login on the logout event', async () => {
    const router = createTestRouter()
    await router.push('/dashboard')
    await router.isReady()

    const wrapper = mount(App, { global: { plugins: [router] } })
    await flushPromises()

    await wrapper.find('[data-testid="logout-button"]').trigger('click')
    await flushPromises()

    expect(mockLogout).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.path).toBe('/login')
  })
})
