/**
 * Tests for StorageTargetNewView component
 * Coverage:
 * - Renders the creation form (defaults to type=azure)
 * - Submits the azure form and redirects to /storage-targets on success
 * - Switching type renders the s3 / filesystem field groups instead
 * - Submits the s3 and filesystem forms with the right payload shape
 * - Shows an error alert on failure
 * - Cancel button navigates back without calling the API
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import StorageTargetNewView from './StorageTargetNewView.vue'
import type { StorageTargetDTO } from '@/api'

vi.mock('@/api', () => ({
  postStorageTarget: vi.fn(),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/storage-targets/new', component: StorageTargetNewView },
      {
        path: '/storage-targets',
        component: { template: '<div>List</div>' },
      },
    ],
  })
}

async function mountView() {
  const router = createTestRouter()
  await router.push('/storage-targets/new')
  await router.isReady()
  const wrapper = mount(StorageTargetNewView, {
    global: { plugins: [router] },
  })
  return { wrapper, router }
}

describe('StorageTargetNewView', () => {
  const mockCreated: StorageTargetDTO = {
    id: 'st-1',
    name: 'Primary Storage',
    type: 'azure',
    accountName: 'backapeandoprod',
    containerName: 'backups',
    createdAt: '2026-09-15T00:00:00Z',
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the form fields for the default type (azure)', async () => {
    const { wrapper } = await mountView()

    expect(
      wrapper.find('[data-testid="storage-target-form"]').exists()
    ).toBe(true)
    expect(wrapper.find('[data-testid="name-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="type-select"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="account-name-input"]').exists()).toBe(
      true
    )
    expect(wrapper.find('[data-testid="container-name-input"]').exists()).toBe(
      true
    )
    expect(wrapper.find('[data-testid="sas-token-input"]').exists()).toBe(true)
  })

  it('should call postStorageTarget with azure fields and redirect on submit', async () => {
    const api = await import('@/api')
    vi.mocked(api.postStorageTarget).mockResolvedValue(mockCreated)

    const { wrapper, router } = await mountView()

    await wrapper.find('[data-testid="name-input"]').setValue('Primary Storage')
    await wrapper
      .find('[data-testid="account-name-input"]')
      .setValue('backapeandoprod')
    await wrapper
      .find('[data-testid="container-name-input"]')
      .setValue('backups')
    await wrapper.find('[data-testid="sas-token-input"]').setValue('sv=...')

    await wrapper.find('[data-testid="storage-target-form"]').trigger('submit')
    await flushPromises()

    expect(api.postStorageTarget).toHaveBeenCalledWith({
      name: 'Primary Storage',
      type: 'azure',
      accountName: 'backapeandoprod',
      containerName: 'backups',
      sasToken: 'sv=...',
    })
    expect(router.currentRoute.value.path).toBe('/storage-targets')
  })

  it('should render s3 fields when type is switched to s3', async () => {
    const { wrapper } = await mountView()

    await wrapper.find('[data-testid="type-select"]').setValue('s3')

    expect(wrapper.find('[data-testid="bucket-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="access-key-id-input"]').exists()).toBe(
      true
    )
    expect(
      wrapper.find('[data-testid="secret-access-key-input"]').exists()
    ).toBe(true)
    expect(wrapper.find('[data-testid="account-name-input"]').exists()).toBe(
      false
    )
  })

  it('should call postStorageTarget with s3 fields', async () => {
    const api = await import('@/api')
    vi.mocked(api.postStorageTarget).mockResolvedValue({
      ...mockCreated,
      type: 's3',
    })

    const { wrapper } = await mountView()

    await wrapper.find('[data-testid="name-input"]').setValue('S3 Storage')
    await wrapper.find('[data-testid="type-select"]').setValue('s3')
    await wrapper.find('[data-testid="bucket-input"]').setValue('my-bucket')
    await wrapper
      .find('[data-testid="access-key-id-input"]')
      .setValue('AKIAEXAMPLE')
    await wrapper
      .find('[data-testid="secret-access-key-input"]')
      .setValue('secret123')

    await wrapper.find('[data-testid="storage-target-form"]').trigger('submit')
    await flushPromises()

    expect(api.postStorageTarget).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'S3 Storage',
        type: 's3',
        bucket: 'my-bucket',
        accessKeyId: 'AKIAEXAMPLE',
        secretAccessKey: 'secret123',
        usePathStyle: false,
      })
    )
  })

  it('should render filesystem fields when type is switched to filesystem', async () => {
    const { wrapper } = await mountView()

    await wrapper.find('[data-testid="type-select"]').setValue('filesystem')

    expect(wrapper.find('[data-testid="root-path-input"]').exists()).toBe(
      true
    )
    expect(wrapper.find('[data-testid="account-name-input"]').exists()).toBe(
      false
    )
    expect(wrapper.find('[data-testid="bucket-input"]').exists()).toBe(false)
  })

  it('should call postStorageTarget with filesystem fields', async () => {
    const api = await import('@/api')
    vi.mocked(api.postStorageTarget).mockResolvedValue({
      ...mockCreated,
      type: 'filesystem',
    })

    const { wrapper } = await mountView()

    await wrapper.find('[data-testid="name-input"]').setValue('Local Storage')
    await wrapper.find('[data-testid="type-select"]').setValue('filesystem')
    await wrapper
      .find('[data-testid="root-path-input"]')
      .setValue('/mnt/backups')

    await wrapper.find('[data-testid="storage-target-form"]').trigger('submit')
    await flushPromises()

    expect(api.postStorageTarget).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Local Storage',
        type: 'filesystem',
        rootPath: '/mnt/backups',
      })
    )
  })

  it('should show an error alert when postStorageTarget fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.postStorageTarget).mockRejectedValue(
      new Error('bad request')
    )

    const { wrapper, router } = await mountView()

    await wrapper.find('[data-testid="name-input"]').setValue('Primary Storage')
    await wrapper
      .find('[data-testid="account-name-input"]')
      .setValue('backapeandoprod')
    await wrapper
      .find('[data-testid="container-name-input"]')
      .setValue('backups')
    await wrapper.find('[data-testid="sas-token-input"]').setValue('sv=...')

    await wrapper.find('[data-testid="storage-target-form"]').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(router.currentRoute.value.path).toBe('/storage-targets/new')
  })

  it('should navigate to /storage-targets on cancel without calling the API', async () => {
    const api = await import('@/api')
    const { wrapper, router } = await mountView()

    await wrapper.find('[data-testid="cancel-button"]').trigger('click')
    await flushPromises()

    expect(api.postStorageTarget).not.toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/storage-targets')
  })
})
