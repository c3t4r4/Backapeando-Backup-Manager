/**
 * Tests for ServersView component
 * Coverage:
 * - Renders servers layout with title and buttons
 * - Loads servers on mount
 * - Displays ServerList when servers loaded
 * - Shows empty state when no servers
 * - Handles API errors gracefully
 * - Refresh button functionality
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import ServersView from './ServersView.vue'
import type { ServerDTO } from '@/api'

// Mock the API module at module level
vi.mock('@/api', () => ({
  getServers: vi.fn(),
  deleteServer: vi.fn(),
}))

// Mock the useAuth composable
vi.mock('@/composables/useAuth', () => ({
  useAuth: () => ({
    user: { email: 'user@example.com', role: 'admin' },
    isLoggedIn: true,
    loading: false,
    error: null,
    login: vi.fn(),
    logout: vi.fn(),
    checkSession: vi.fn(),
  }),
}))

/**
 * Test router with the routes ServersView navigates to — a real router
 * instance is required so `useRouter()`/`router.push()` inside the
 * component work instead of warning "injection Symbol(router) not found".
 */
function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/servers', component: ServersView },
      { path: '/servers/new', component: { template: '<div>New</div>' } },
      {
        path: '/servers/:id/edit',
        component: { template: '<div>Edit</div>' },
      },
      { path: '/history', component: { template: '<div>History</div>' } },
    ],
  })
}

async function mountServersView() {
  const router = createTestRouter()
  await router.push('/servers')
  await router.isReady()
  const wrapper = mount(ServersView, { global: { plugins: [router] } })
  return { wrapper, router }
}

describe('ServersView', () => {
  const mockServers: ServerDTO[] = [
    {
      id: '1',
      name: 'Production DB',
      host: '10.0.1.100',
      port: 5432,
      sshUser: 'postgres',
      containerName: 'postgres',
      dbName: 'production',
      dbUser: 'backup_user',
      pgDumpExtraArgs: '',
      cronExpression: '0 3 * * *',
      enabled: true,
      status: 'ready',
      createdAt: '2026-09-14T10:00:00Z',
      updatedAt: '2026-09-14T10:00:00Z',
    },
    {
      id: '2',
      name: 'Staging DB',
      host: '10.0.2.50',
      port: 5432,
      sshUser: 'postgres',
      containerName: 'postgres',
      dbName: 'staging',
      dbUser: 'backup_user',
      pgDumpExtraArgs: '',
      cronExpression: '0 4 * * *',
      enabled: false,
      status: 'awaiting_authorization',
      createdAt: '2026-09-14T11:00:00Z',
      updatedAt: '2026-09-14T11:00:00Z',
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render dashboard on mount with title and buttons', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue([])

    const { wrapper } = await mountServersView()
    await flushPromises()

    // Check title
    expect(wrapper.find('h1').text()).toBe('Servidores')

    // Check buttons exist
    expect(wrapper.find('[data-testid="create-server-btn"]').exists()).toBe(
      true
    )
    expect(wrapper.find('[data-testid="refresh-btn"]').exists()).toBe(true)

    // Check button texts
    expect(wrapper.text()).toContain('+ Novo Servidor')
    expect(wrapper.text()).toContain('Atualizar')
  })

  it('should load servers on mount', async () => {
    const api = await import('@/api')
    const mock = vi.mocked(api.getServers)
    mock.mockResolvedValue(mockServers)

    await mountServersView()
    await flushPromises()

    expect(mock).toHaveBeenCalled()
    expect(mock).toHaveBeenCalledTimes(1)
  })

  it('should render ServerList when servers loaded', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)

    const { wrapper } = await mountServersView()
    await flushPromises()

    // Check ServerList component is rendered
    expect(wrapper.find('[data-testid="server-list"]').exists()).toBe(true)

    // Check empty state is not shown
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(false)
  })

  it('should show empty state when no servers', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue([])

    const { wrapper } = await mountServersView()
    await flushPromises()

    // Check empty state is shown
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Nenhum servidor cadastrado')
    expect(
      wrapper.find('[data-testid="create-first-server-btn"]').exists()
    ).toBe(true)

    // Check ServerList is not shown
    expect(wrapper.find('[data-testid="server-list"]').exists()).toBe(false)
  })

  it('should handle errors gracefully', async () => {
    const api = await import('@/api')
    const errorMessage = 'Network error: Failed to fetch servers'
    vi.mocked(api.getServers).mockRejectedValue(new Error(errorMessage))

    const { wrapper } = await mountServersView()
    await flushPromises()

    // Check error alert is shown with generic message (not sensitive details)
    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Erro ao carregar servidores')
    expect(wrapper.text()).toContain('Tente novamente')

    // Raw error message should NOT be exposed
    expect(wrapper.text()).not.toContain('Network error')

    // Check empty list
    expect(wrapper.find('[data-testid="server-list"]').exists()).toBe(false)

    // Check empty state is not shown when error exists
    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(false)
  })

  it('should refetch on refresh button click', async () => {
    const api = await import('@/api')
    const mock = vi.mocked(api.getServers)
    mock.mockResolvedValue(mockServers)

    const { wrapper } = await mountServersView()
    await flushPromises()

    expect(mock).toHaveBeenCalledTimes(1)

    // Click refresh button
    const refreshBtn = wrapper.find('[data-testid="refresh-btn"]')
    await refreshBtn.trigger('click')
    await flushPromises()

    // Should have been called twice (initial + refresh)
    expect(mock).toHaveBeenCalledTimes(2)
  })

  it('should disable refresh button while loading', async () => {
    const api = await import('@/api')

    // Create a promise that we can control
    let resolveFetch: any
    const fetchPromise = new Promise((resolve) => {
      resolveFetch = resolve
    })
    vi.mocked(api.getServers).mockReturnValue(fetchPromise as any)

    const { wrapper } = await mountServersView()
    await wrapper.vm.$nextTick()

    const refreshBtn = wrapper.find<HTMLButtonElement>(
      '[data-testid="refresh-btn"]'
    )
    expect(refreshBtn.element.disabled).toBe(true)

    // Resolve the fetch
    resolveFetch([])
    await flushPromises()

    // Button should no longer be disabled
    expect(refreshBtn.element.disabled).toBe(false)
  })

  it('should display loading spinner while fetching', async () => {
    const api = await import('@/api')

    // Create a promise that we can control
    let resolveFetch: any
    const fetchPromise = new Promise((resolve) => {
      resolveFetch = resolve
    })
    vi.mocked(api.getServers).mockReturnValue(fetchPromise as any)

    const { wrapper } = await mountServersView()
    await wrapper.vm.$nextTick()

    // Spinner should be visible
    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(true)

    // Resolve the fetch
    resolveFetch([])
    await flushPromises()

    // Spinner should no longer be visible
    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(false)
  })

  it('should navigate to /servers/new on create button click', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue([])

    const { wrapper, router } = await mountServersView()
    await flushPromises()

    const createBtn = wrapper.find('[data-testid="create-server-btn"]')
    await createBtn.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/servers/new')
  })

  it('should navigate to /servers/:id/edit on edit button click', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)

    const { wrapper, router } = await mountServersView()
    await flushPromises()

    const editBtn = wrapper.find('[data-testid="edit-btn-1"]')
    await editBtn.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/servers/1/edit')
  })

  it('should navigate to /history with server filter on view-backups button click', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)

    const { wrapper, router } = await mountServersView()
    await flushPromises()

    const backupsBtn = wrapper.find('[data-testid="backups-btn-1"]')
    await backupsBtn.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/history')
    expect(router.currentRoute.value.query.server).toBe('1')
  })

  it('should show confirmation dialog on delete button click without deleting', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)

    const { wrapper } = await mountServersView()
    await flushPromises()

    const deleteBtn = wrapper.find('[data-testid="delete-btn-1"]')
    await deleteBtn.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(true)
    expect(vi.mocked(api.deleteServer)).not.toHaveBeenCalled()
  })

  it('should delete the server and remove it from the list on confirm', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)
    vi.mocked(api.deleteServer).mockResolvedValue(undefined)

    const { wrapper } = await mountServersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-1"]').trigger('click')
    await flushPromises()

    await wrapper
      .find('[data-testid="confirm-dialog-confirm"]')
      .trigger('click')
    await flushPromises()

    expect(api.deleteServer).toHaveBeenCalledWith('1')
    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="server-row-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="server-row-2"]').exists()).toBe(true)
  })

  it('should not call deleteServer twice on a double-click while the request is in flight', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)

    let resolveDelete: () => void = () => {}
    const deletePromise = new Promise<void>((resolve) => {
      resolveDelete = resolve
    })
    vi.mocked(api.deleteServer).mockReturnValue(deletePromise)

    const { wrapper } = await mountServersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-1"]').trigger('click')
    await flushPromises()

    const confirmBtn = wrapper.find<HTMLButtonElement>(
      '[data-testid="confirm-dialog-confirm"]'
    )
    await confirmBtn.trigger('click')
    await confirmBtn.trigger('click')
    await confirmBtn.trigger('click')

    expect(api.deleteServer).toHaveBeenCalledTimes(1)
    expect(confirmBtn.element.disabled).toBe(true)

    resolveDelete()
    await flushPromises()

    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)
  })

  it('should not delete the server when cancelling the dialog', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)

    const { wrapper } = await mountServersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-1"]').trigger('click')
    await flushPromises()

    await wrapper.find('[data-testid="confirm-dialog-cancel"]').trigger('click')
    await flushPromises()

    expect(api.deleteServer).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="server-row-1"]').exists()).toBe(true)
  })

  it('should show an error alert when delete fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)
    vi.mocked(api.deleteServer).mockRejectedValue(new Error('network error'))

    const { wrapper } = await mountServersView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-1"]').trigger('click')
    await flushPromises()

    await wrapper
      .find('[data-testid="confirm-dialog-confirm"]')
      .trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Erro ao deletar servidor')
    expect(wrapper.find('[data-testid="server-row-1"]').exists()).toBe(true)
  })
})
