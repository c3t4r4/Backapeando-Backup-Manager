/**
 * Tests for AdminUsersView component
 * Coverage:
 * - Renders list layout with title and buttons
 * - Loads admin users on mount
 * - Empty state / error handling
 * - Create/edit navigation
 * - Delete confirmation flow (including double-submit guard)
 * - Self-deletion is disabled in the UI
 * - Backend-provided error messages surface literally (e.g. last-admin guard)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import AdminUsersView from './AdminUsersView.vue'
import type { AdminUserDTO } from '@/api'

vi.mock('@/api', () => ({
  getAdminUsers: vi.fn(),
  deleteAdminUser: vi.fn(),
}))

vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({
    user: ref({ id: 'self-1', email: 'self@example.com', role: 'admin' }),
    isLoggedIn: ref(true),
    loading: ref(false),
    error: ref(null),
    login: vi.fn(),
    logout: vi.fn(),
    checkSession: vi.fn(),
  }),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin-users', component: AdminUsersView },
      { path: '/admin-users/new', component: { template: '<div>New</div>' } },
      {
        path: '/admin-users/:id/edit',
        component: { template: '<div>Edit</div>' },
      },
    ],
  })
}

async function mountAdminUsersView() {
  const router = createTestRouter()
  await router.push('/admin-users')
  await router.isReady()
  const wrapper = mount(AdminUsersView, { global: { plugins: [router] } })
  return { wrapper, router }
}

describe('AdminUsersView', () => {
  const mockUsers: AdminUserDTO[] = [
    {
      id: 'self-1',
      email: 'self@example.com',
      cpf: '11144477735',
      role: 'admin',
      createdAt: '2026-09-14T10:00:00Z',
      updatedAt: '2026-09-14T10:00:00Z',
    },
    {
      id: 'other-2',
      email: 'other@example.com',
      cpf: '52998224725',
      role: 'admin',
      createdAt: '2026-09-14T11:00:00Z',
      updatedAt: '2026-09-14T11:00:00Z',
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render title and buttons', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue([])

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    expect(wrapper.find('h1').text()).toBe('Administradores')
    expect(wrapper.find('[data-testid="create-admin-user-btn"]').exists()).toBe(
      true
    )
  })

  it('should load admin users on mount', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue(mockUsers)

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    expect(api.getAdminUsers).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="admin-users-table"]').exists()).toBe(
      true
    )
    expect(
      wrapper.find('[data-testid="admin-user-row-other-2"]').exists()
    ).toBe(true)
  })

  it('should show empty state when no admin users', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue([])

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
  })

  it('should show error alert on fetch failure', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockRejectedValue(new Error('boom'))

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
  })

  it('should navigate to /admin-users/new on create button click', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue([])

    const { wrapper, router } = await mountAdminUsersView()
    await flushPromises()

    await wrapper.find('[data-testid="create-admin-user-btn"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/admin-users/new')
  })

  it('should navigate to /admin-users/:id/edit on edit button click', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue(mockUsers)

    const { wrapper, router } = await mountAdminUsersView()
    await flushPromises()

    await wrapper.find('[data-testid="edit-btn-other-2"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/admin-users/other-2/edit')
  })

  it('should disable the delete button for the currently logged-in user', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue(mockUsers)

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    expect(
      wrapper.find('[data-testid="delete-btn-self-1"]').attributes('disabled')
    ).toBeDefined()
    expect(
      wrapper.find('[data-testid="delete-btn-other-2"]').attributes('disabled')
    ).toBeUndefined()
  })

  it('should delete the user and remove it from the list on confirm', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue(mockUsers)
    vi.mocked(api.deleteAdminUser).mockResolvedValue(undefined)

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-other-2"]').trigger('click')
    await flushPromises()
    await wrapper
      .find('[data-testid="confirm-dialog-confirm"]')
      .trigger('click')
    await flushPromises()

    expect(api.deleteAdminUser).toHaveBeenCalledWith('other-2')
    expect(
      wrapper.find('[data-testid="admin-user-row-other-2"]').exists()
    ).toBe(false)
    expect(wrapper.find('[data-testid="admin-user-row-self-1"]').exists()).toBe(
      true
    )
  })

  it('should not delete twice on rapid repeated confirm clicks', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue(mockUsers)
    let resolveDelete: () => void = () => {}
    vi.mocked(api.deleteAdminUser).mockReturnValue(
      new Promise((resolve) => {
        resolveDelete = resolve
      })
    )

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-other-2"]').trigger('click')
    await flushPromises()

    const confirmBtn = wrapper.find('[data-testid="confirm-dialog-confirm"]')
    await confirmBtn.trigger('click')
    await confirmBtn.trigger('click')

    expect(api.deleteAdminUser).toHaveBeenCalledTimes(1)

    resolveDelete()
    await flushPromises()
  })

  it('should cancel deletion without calling the API', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue(mockUsers)

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-other-2"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="confirm-dialog-cancel"]').trigger('click')
    await flushPromises()

    expect(api.deleteAdminUser).not.toHaveBeenCalled()
    expect(
      wrapper.find('[data-testid="admin-user-row-other-2"]').exists()
    ).toBe(true)
  })

  it('should show the backend error message when deletion is rejected (e.g. last admin)', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUsers).mockResolvedValue(mockUsers)
    vi.mocked(api.deleteAdminUser).mockRejectedValue({
      isAxiosError: true,
      response: { data: { error: 'cannot delete the last remaining admin' } },
    })

    const { wrapper } = await mountAdminUsersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-other-2"]').trigger('click')
    await flushPromises()
    await wrapper
      .find('[data-testid="confirm-dialog-confirm"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').text()).toContain(
      'cannot delete the last remaining admin'
    )
  })
})
