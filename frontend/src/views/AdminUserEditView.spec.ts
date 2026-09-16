import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import AdminUserEditView from './AdminUserEditView.vue'
import type { AdminUserDTO } from '@/api'

vi.mock('@/api', () => ({
  getAdminUser: vi.fn(),
  putAdminUser: vi.fn(),
  deleteAdminUser: vi.fn(),
}))

let mockCurrentUserId = 'self-1'
vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({
    user: ref({
      id: mockCurrentUserId,
      email: 'self@example.com',
      role: 'admin',
    }),
    isLoggedIn: ref(true),
    loading: ref(false),
    error: ref(null),
    login: vi.fn(),
    logout: vi.fn(),
    checkSession: vi.fn(),
  }),
}))

const existingUser: AdminUserDTO = {
  id: 'other-2',
  email: 'other@example.com',
  cpf: '52998224725',
  role: 'admin',
  createdAt: '2026-09-14T11:00:00Z',
  updatedAt: '2026-09-14T11:00:00Z',
}

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin-users', component: { template: '<div>List</div>' } },
      {
        path: '/admin-users/:id/edit',
        component: AdminUserEditView,
        props: true,
      },
    ],
  })
}

async function mountAdminUserEditView(id = 'other-2') {
  const router = createTestRouter()
  await router.push(`/admin-users/${id}/edit`)
  await router.isReady()
  const wrapper = mount(AdminUserEditView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('AdminUserEditView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockCurrentUserId = 'self-1'
  })

  it('should load existing user data on mount', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUser).mockResolvedValue(existingUser)

    const { wrapper } = await mountAdminUserEditView()

    expect(api.getAdminUser).toHaveBeenCalledWith('other-2')
    expect(
      (wrapper.find('[data-testid="email-input"]').element as HTMLInputElement)
        .value
    ).toBe('other@example.com')
  })

  it('should submit without a password field when left blank', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUser).mockResolvedValue(existingUser)
    vi.mocked(api.putAdminUser).mockResolvedValue(existingUser)

    const { wrapper, router } = await mountAdminUserEditView()

    await wrapper.find('[data-testid="admin-user-form"]').trigger('submit')
    await flushPromises()

    expect(api.putAdminUser).toHaveBeenCalledWith('other-2', {
      email: 'other@example.com',
      cpf: '52998224725',
    })
    expect(router.currentRoute.value.path).toBe('/admin-users')
  })

  it('should include the password when filled in', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUser).mockResolvedValue(existingUser)
    vi.mocked(api.putAdminUser).mockResolvedValue(existingUser)

    const { wrapper } = await mountAdminUserEditView()

    await wrapper
      .find('[data-testid="password-input"]')
      .setValue('brandnewpassword12')
    await wrapper.find('[data-testid="admin-user-form"]').trigger('submit')
    await flushPromises()

    expect(api.putAdminUser).toHaveBeenCalledWith('other-2', {
      email: 'other@example.com',
      cpf: '52998224725',
      password: 'brandnewpassword12',
    })
  })

  it('should disable the delete button when editing your own account', async () => {
    mockCurrentUserId = 'other-2'
    const api = await import('@/api')
    vi.mocked(api.getAdminUser).mockResolvedValue(existingUser)

    const { wrapper } = await mountAdminUserEditView('other-2')

    expect(
      wrapper.find('[data-testid="delete-button"]').attributes('disabled')
    ).toBeDefined()
  })

  it('should delete the account and navigate to the list on confirm', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUser).mockResolvedValue(existingUser)
    vi.mocked(api.deleteAdminUser).mockResolvedValue(undefined)

    const { wrapper, router } = await mountAdminUserEditView()

    await wrapper.find('[data-testid="delete-button"]').trigger('click')
    await flushPromises()
    await wrapper
      .find('[data-testid="confirm-dialog-confirm"]')
      .trigger('click')
    await flushPromises()

    expect(api.deleteAdminUser).toHaveBeenCalledWith('other-2')
    expect(router.currentRoute.value.path).toBe('/admin-users')
  })

  it('should show the backend error message when the update is rejected', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAdminUser).mockResolvedValue(existingUser)
    vi.mocked(api.putAdminUser).mockRejectedValue({
      isAxiosError: true,
      response: { data: { error: 'cpf already registered' } },
    })

    const { wrapper } = await mountAdminUserEditView()

    await wrapper.find('[data-testid="admin-user-form"]').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').text()).toContain(
      'cpf already registered'
    )
  })
})
