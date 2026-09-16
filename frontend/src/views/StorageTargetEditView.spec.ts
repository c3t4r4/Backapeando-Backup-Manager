/**
 * Tests for StorageTargetEditView component
 * Coverage:
 * - Loads the existing target and populates the form (secret stays blank)
 * - Shows a loading spinner while fetching
 * - Submits the form and redirects to /storage-targets on success
 * - Omits the secret from the request when left blank ("keep current")
 * - Shows an error alert when loading or saving fails
 * - Cancel button navigates back without calling the API
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import StorageTargetEditView from './StorageTargetEditView.vue'
import type { StorageTargetDTO } from '@/api'

vi.mock('@/api', () => ({
  getStorageTarget: vi.fn(),
  putStorageTarget: vi.fn(),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/storage-targets/:id/edit', component: StorageTargetEditView },
      {
        path: '/storage-targets',
        component: { template: '<div>List</div>' },
      },
    ],
  })
}

async function mountView(id = 'st-1') {
  const router = createTestRouter()
  await router.push(`/storage-targets/${id}/edit`)
  await router.isReady()
  const wrapper = mount(StorageTargetEditView, {
    global: { plugins: [router] },
  })
  return { wrapper, router }
}

describe('StorageTargetEditView', () => {
  const mockTarget: StorageTargetDTO = {
    id: 'st-1',
    name: 'Primary Storage',
    type: 'azure',
    accountName: 'backapeandoprod',
    containerName: 'backups',
    createdAt: '2026-09-14T10:00:00Z',
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should show a loading spinner while fetching the target', async () => {
    const api = await import('@/api')
    let resolveFetch: (value: StorageTargetDTO) => void = () => {}
    vi.mocked(api.getStorageTarget).mockReturnValue(
      new Promise((resolve) => {
        resolveFetch = resolve
      })
    )

    const { wrapper } = await mountView()

    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(true)

    resolveFetch(mockTarget)
    await flushPromises()

    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(false)
  })

  it('should populate the form with the loaded target, leaving the secret blank', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTarget).mockResolvedValue(mockTarget)

    const { wrapper } = await mountView()
    await flushPromises()

    expect(
      wrapper.find<HTMLInputElement>('[data-testid="name-input"]').element.value
    ).toBe('Primary Storage')
    expect(
      wrapper.find<HTMLSelectElement>('[data-testid="type-select"]').element
        .value
    ).toBe('azure')
    expect(
      wrapper.find<HTMLInputElement>('[data-testid="account-name-input"]')
        .element.value
    ).toBe('backapeandoprod')
    expect(
      wrapper.find<HTMLInputElement>('[data-testid="sas-token-input"]').element
        .value
    ).toBe('')
    expect(api.getStorageTarget).toHaveBeenCalledWith('st-1')
  })

  it('should show an error alert when loading fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTarget).mockRejectedValue(new Error('not found'))

    const { wrapper } = await mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="storage-target-form"]').exists()).toBe(
      false
    )
  })

  it('should submit without a secret when left blank (keep current)', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTarget).mockResolvedValue(mockTarget)
    vi.mocked(api.putStorageTarget).mockResolvedValue(mockTarget)

    const { wrapper, router } = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="storage-target-form"]').trigger('submit')
    await flushPromises()

    expect(api.putStorageTarget).toHaveBeenCalledWith(
      'st-1',
      expect.objectContaining({ sasToken: undefined })
    )
    expect(router.currentRoute.value.path).toBe('/storage-targets')
  })

  it('should include the secret when provided', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTarget).mockResolvedValue(mockTarget)
    vi.mocked(api.putStorageTarget).mockResolvedValue(mockTarget)

    const { wrapper } = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="sas-token-input"]').setValue('new-sas')
    await wrapper.find('[data-testid="storage-target-form"]').trigger('submit')
    await flushPromises()

    expect(api.putStorageTarget).toHaveBeenCalledWith(
      'st-1',
      expect.objectContaining({ sasToken: 'new-sas' })
    )
  })

  it('should show an error alert when saving fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTarget).mockResolvedValue(mockTarget)
    vi.mocked(api.putStorageTarget).mockRejectedValue(new Error('conflict'))

    const { wrapper, router } = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="storage-target-form"]').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(router.currentRoute.value.path).toBe('/storage-targets/st-1/edit')
  })

  it('should navigate to /storage-targets on cancel without calling the API', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTarget).mockResolvedValue(mockTarget)

    const { wrapper, router } = await mountView()
    await flushPromises()

    await wrapper.find('[data-testid="cancel-button"]').trigger('click')
    await flushPromises()

    expect(api.putStorageTarget).not.toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/storage-targets')
  })
})
