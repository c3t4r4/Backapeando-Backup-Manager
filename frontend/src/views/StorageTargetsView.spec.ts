/**
 * Tests for StorageTargetsView component
 * Coverage:
 * - Renders list layout with title and buttons
 * - Loads targets on mount
 * - Empty state / error handling
 * - Create/edit navigation
 * - Delete confirmation flow (including double-submit guard)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import StorageTargetsView from './StorageTargetsView.vue'
import type { StorageTargetDTO } from '@/api'

vi.mock('@/api', () => ({
  getStorageTargets: vi.fn(),
  deleteStorageTarget: vi.fn(),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/storage-targets', component: StorageTargetsView },
      {
        path: '/storage-targets/new',
        component: { template: '<div>New</div>' },
      },
      {
        path: '/storage-targets/:id/edit',
        component: { template: '<div>Edit</div>' },
      },
    ],
  })
}

async function mountStorageTargetsView() {
  const router = createTestRouter()
  await router.push('/storage-targets')
  await router.isReady()
  const wrapper = mount(StorageTargetsView, { global: { plugins: [router] } })
  return { wrapper, router }
}

describe('StorageTargetsView', () => {
  const mockTargets: StorageTargetDTO[] = [
    {
      id: '1',
      name: 'Primary Storage',
      type: 'azure',
      accountName: 'backapeandoprod',
      containerName: 'backups',
      createdAt: '2026-09-14T10:00:00Z',
    },
    {
      id: '2',
      name: 'Secondary Storage',
      type: 's3',
      bucket: 'backups-dev',
      region: 'us-east-1',
      createdAt: '2026-09-14T11:00:00Z',
    },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render title and buttons', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue([])

    const { wrapper } = await mountStorageTargetsView()
    await flushPromises()

    expect(wrapper.find('h1').text()).toBe('Destinos de Backup')
    expect(wrapper.find('[data-testid="create-target-btn"]').exists()).toBe(
      true
    )
  })

  it('should load targets on mount', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockTargets)

    const { wrapper } = await mountStorageTargetsView()
    await flushPromises()

    expect(api.getStorageTargets).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="storage-target-list"]').exists()).toBe(
      true
    )
  })

  it('should show empty state when no targets', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue([])

    const { wrapper } = await mountStorageTargetsView()
    await flushPromises()

    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
  })

  it('should show error alert on fetch failure', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockRejectedValue(new Error('boom'))

    const { wrapper } = await mountStorageTargetsView()
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
  })

  it('should navigate to /storage-targets/new on create button click', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue([])

    const { wrapper, router } = await mountStorageTargetsView()
    await flushPromises()

    await wrapper.find('[data-testid="create-target-btn"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/storage-targets/new')
  })

  it('should navigate to /storage-targets/:id/edit on edit button click', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockTargets)

    const { wrapper, router } = await mountStorageTargetsView()
    await flushPromises()

    await wrapper.find('[data-testid="edit-btn-1"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/storage-targets/1/edit')
  })

  it('should delete the target and remove it from the list on confirm', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockTargets)
    vi.mocked(api.deleteStorageTarget).mockResolvedValue(undefined)

    const { wrapper } = await mountStorageTargetsView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-1"]').trigger('click')
    await flushPromises()
    await wrapper
      .find('[data-testid="confirm-dialog-confirm"]')
      .trigger('click')
    await flushPromises()

    expect(api.deleteStorageTarget).toHaveBeenCalledWith('1')
    expect(
      wrapper.find('[data-testid="storage-target-row-1"]').exists()
    ).toBe(false)
    expect(
      wrapper.find('[data-testid="storage-target-row-2"]').exists()
    ).toBe(true)
  })

  it('should not delete twice on rapid repeated confirm clicks', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockTargets)
    let resolveDelete: () => void = () => {}
    vi.mocked(api.deleteStorageTarget).mockReturnValue(
      new Promise((resolve) => {
        resolveDelete = resolve
      })
    )

    const { wrapper } = await mountStorageTargetsView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-1"]').trigger('click')
    await flushPromises()

    const confirmBtn = wrapper.find('[data-testid="confirm-dialog-confirm"]')
    await confirmBtn.trigger('click')
    await confirmBtn.trigger('click')

    expect(api.deleteStorageTarget).toHaveBeenCalledTimes(1)

    resolveDelete()
    await flushPromises()
  })

  it('should cancel deletion without calling the API', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockTargets)

    const { wrapper } = await mountStorageTargetsView()
    await flushPromises()

    await wrapper.find('[data-testid="delete-btn-1"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="confirm-dialog-cancel"]').trigger('click')
    await flushPromises()

    expect(api.deleteStorageTarget).not.toHaveBeenCalled()
    expect(
      wrapper.find('[data-testid="storage-target-row-1"]').exists()
    ).toBe(true)
  })
})
