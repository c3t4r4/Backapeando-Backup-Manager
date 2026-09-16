import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import AdminUserNewView from './AdminUserNewView.vue'
import type { AdminUserDTO } from '@/api'

vi.mock('@/api', () => ({
  postAdminUser: vi.fn(),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin-users', component: { template: '<div>List</div>' } },
      { path: '/admin-users/new', component: AdminUserNewView },
    ],
  })
}

async function mountAdminUserNewView() {
  const router = createTestRouter()
  await router.push('/admin-users/new')
  await router.isReady()
  const wrapper = mount(AdminUserNewView, { global: { plugins: [router] } })
  return { wrapper, router }
}

describe('AdminUserNewView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the form fields', async () => {
    const { wrapper } = await mountAdminUserNewView()

    expect(wrapper.find('[data-testid="email-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="cpf-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="password-input"]').exists()).toBe(true)
  })

  it('should submit the form and navigate to /admin-users on success', async () => {
    const api = await import('@/api')
    const created: AdminUserDTO = {
      id: '1',
      email: 'new@example.com',
      cpf: '11144477735',
      role: 'admin',
      createdAt: '2026-09-15T10:00:00Z',
      updatedAt: '2026-09-15T10:00:00Z',
    }
    vi.mocked(api.postAdminUser).mockResolvedValue(created)

    const { wrapper, router } = await mountAdminUserNewView()

    await wrapper
      .find('[data-testid="email-input"]')
      .setValue('new@example.com')
    await wrapper.find('[data-testid="cpf-input"]').setValue('11144477735')
    await wrapper
      .find('[data-testid="password-input"]')
      .setValue('supersecretpw12')
    await wrapper.find('[data-testid="admin-user-form"]').trigger('submit')
    await flushPromises()

    expect(api.postAdminUser).toHaveBeenCalledWith({
      email: 'new@example.com',
      cpf: '11144477735',
      password: 'supersecretpw12',
    })
    expect(router.currentRoute.value.path).toBe('/admin-users')
  })

  it('should show the backend error message on failure (e.g. duplicate email)', async () => {
    const api = await import('@/api')
    vi.mocked(api.postAdminUser).mockRejectedValue({
      isAxiosError: true,
      response: { data: { error: 'email already registered' } },
    })

    const { wrapper } = await mountAdminUserNewView()

    await wrapper
      .find('[data-testid="email-input"]')
      .setValue('dup@example.com')
    await wrapper.find('[data-testid="cpf-input"]').setValue('11144477735')
    await wrapper
      .find('[data-testid="password-input"]')
      .setValue('supersecretpw12')
    await wrapper.find('[data-testid="admin-user-form"]').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').text()).toContain(
      'email already registered'
    )
  })

  it('should navigate back to /admin-users on cancel', async () => {
    const { wrapper, router } = await mountAdminUserNewView()

    await wrapper.find('[data-testid="cancel-button"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/admin-users')
  })
})
