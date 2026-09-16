/**
 * Tests for ServerNewView component
 * Coverage:
 * - Renders the creation form
 * - Loads storage targets for the optional dropdown
 * - Submits the form and redirects to /servers on success
 * - Shows an error alert on failure
 * - Cancel button navigates back without calling the API
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import ServerNewView from './ServerNewView.vue'
import type { StorageTargetDTO, ServerDTO } from '@/api'
import { Select } from '@/components/ui/select'

vi.mock('@/api', () => ({
  postServer: vi.fn(),
  getStorageTargets: vi.fn(),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/servers/new', component: ServerNewView },
      { path: '/servers', component: { template: '<div>Servers</div>' } },
    ],
  })
}

async function mountServerNewView() {
  const router = createTestRouter()
  await router.push('/servers/new')
  await router.isReady()
  const wrapper = mount(ServerNewView, { global: { plugins: [router] } })
  return { wrapper, router }
}

/**
 * Reka-ui's Select relies on real pointer capture/focus semantics that
 * happy-dom does not implement, so tests interact with it the same way the
 * template's v-model does: by emitting `update:modelValue` directly on the
 * `Select` component instance, rather than simulating a pointer-driven
 * open+click sequence through its teleported popover. `index` follows DOM
 * order: 0 = motor do banco (dbEngine), 1 = onde o banco roda (deploymentMode).
 */
async function setSelectValue(
  wrapper: VueWrapper,
  index: number,
  value: string
): Promise<void> {
  const selects = wrapper.findAllComponents(Select as never)
  await selects[index].vm.$emit('update:modelValue', value)
  await flushPromises()
}

describe('ServerNewView', () => {
  const mockStorageTargets: StorageTargetDTO[] = [
    {
      id: 'az-1',
      name: 'Primary Storage',
      type: 'azure',
      accountName: 'acct',
      containerName: 'backups',
      createdAt: '2026-09-14T10:00:00Z',
    },
  ]

  const mockCreatedServer: ServerDTO = {
    id: 'srv-1',
    name: 'New Server',
    host: '10.0.0.1',
    port: 22,
    sshUser: 'ubuntu',
    containerName: 'postgres',
    dbName: 'app',
    dbUser: 'backup_user',
    pgDumpExtraArgs: '',
    cronExpression: '0 3 * * *',
    enabled: false,
    status: 'pending_key',
    createdAt: '2026-09-15T00:00:00Z',
    updatedAt: '2026-09-15T00:00:00Z',
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the form fields', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue([])

    const { wrapper } = await mountServerNewView()
    await flushPromises()

    expect(wrapper.find('[data-testid="server-form"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="name-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="host-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="port-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="ssh-user-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="container-name-input"]').exists()).toBe(
      true
    )
    expect(wrapper.find('[data-testid="db-name-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="db-user-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="cron-expression-input"]').exists()).toBe(
      true
    )
  })

  it('should populate the storage target dropdown from the API', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

    const { wrapper } = await mountServerNewView()
    await flushPromises()

    const options = wrapper.findAll(
      '[data-testid="storage-target-select"] option'
    )
    expect(options.some((o) => o.text() === 'Primary Storage (Azure)')).toBe(
      true
    )
  })

  it('should call postServer and redirect to /servers on submit', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue([])
    vi.mocked(api.postServer).mockResolvedValue(mockCreatedServer)

    const { wrapper, router } = await mountServerNewView()
    await flushPromises()

    await wrapper.find('[data-testid="name-input"]').setValue('New Server')
    await wrapper.find('[data-testid="host-input"]').setValue('10.0.0.1')
    await wrapper.find('[data-testid="ssh-user-input"]').setValue('ubuntu')
    await wrapper
      .find('[data-testid="container-name-input"]')
      .setValue('postgres')
    await wrapper.find('[data-testid="db-name-input"]').setValue('app')
    await wrapper.find('[data-testid="db-user-input"]').setValue('backup_user')

    await wrapper.find('[data-testid="server-form"]').trigger('submit')
    await flushPromises()

    expect(api.postServer).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'New Server',
        host: '10.0.0.1',
        sshUser: 'ubuntu',
        containerName: 'postgres',
        dbName: 'app',
        dbUser: 'backup_user',
      })
    )
    expect(router.currentRoute.value.path).toBe('/servers')
  })

  it('should show an error alert when postServer fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue([])
    vi.mocked(api.postServer).mockRejectedValue(new Error('bad request'))

    const { wrapper, router } = await mountServerNewView()
    await flushPromises()

    await wrapper.find('[data-testid="name-input"]').setValue('New Server')
    await wrapper.find('[data-testid="host-input"]').setValue('10.0.0.1')
    await wrapper.find('[data-testid="ssh-user-input"]').setValue('ubuntu')
    await wrapper
      .find('[data-testid="container-name-input"]')
      .setValue('postgres')
    await wrapper.find('[data-testid="db-name-input"]').setValue('app')
    await wrapper.find('[data-testid="db-user-input"]').setValue('backup_user')

    await wrapper.find('[data-testid="server-form"]').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(router.currentRoute.value.path).toBe('/servers/new')
  })

  it('should navigate to /servers on cancel without calling the API', async () => {
    const api = await import('@/api')
    vi.mocked(api.getStorageTargets).mockResolvedValue([])

    const { wrapper, router } = await mountServerNewView()
    await flushPromises()

    await wrapper.find('[data-testid="cancel-button"]').trigger('click')
    await flushPromises()

    expect(api.postServer).not.toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/servers')
  })

  describe('engine and deployment mode selectors', () => {
    it('defaults to postgres + docker, showing pg-dump extra args and containerName', async () => {
      const api = await import('@/api')
      vi.mocked(api.getStorageTargets).mockResolvedValue([])

      const { wrapper } = await mountServerNewView()
      await flushPromises()

      expect(
        wrapper.find('[data-testid="container-name-input"]').exists()
      ).toBe(true)
      expect(
        wrapper.find('[data-testid="pg-dump-extra-args-input"]').exists()
      ).toBe(true)
      expect(
        wrapper.find('[data-testid="mysql-dump-extra-args-input"]').exists()
      ).toBe(false)
      expect(
        wrapper.find('[data-testid="sqlcmd-extra-args-input"]').exists()
      ).toBe(false)
    })

    it('switching to mysql shows the mysql extra-args field and hides the postgres one', async () => {
      const api = await import('@/api')
      vi.mocked(api.getStorageTargets).mockResolvedValue([])

      const { wrapper } = await mountServerNewView()
      await flushPromises()

      await setSelectValue(wrapper, 0, 'mysql')

      expect(
        wrapper.find('[data-testid="pg-dump-extra-args-input"]').exists()
      ).toBe(false)
      expect(
        wrapper.find('[data-testid="mysql-dump-extra-args-input"]').exists()
      ).toBe(true)
      expect(
        wrapper.find('[data-testid="sqlcmd-extra-args-input"]').exists()
      ).toBe(false)
    })

    it('switching to sqlserver shows the sqlcmd extra-args field', async () => {
      const api = await import('@/api')
      vi.mocked(api.getStorageTargets).mockResolvedValue([])

      const { wrapper } = await mountServerNewView()
      await flushPromises()

      await setSelectValue(wrapper, 0, 'sqlserver')

      expect(
        wrapper.find('[data-testid="sqlcmd-extra-args-input"]').exists()
      ).toBe(true)
    })

    it('switching deploymentMode to host hides the containerName field', async () => {
      const api = await import('@/api')
      vi.mocked(api.getStorageTargets).mockResolvedValue([])

      const { wrapper } = await mountServerNewView()
      await flushPromises()

      await setSelectValue(wrapper, 1, 'host')

      expect(
        wrapper.find('[data-testid="container-name-input"]').exists()
      ).toBe(false)
    })

    it('omits containerName and dbPassword from the payload in host mode with no password', async () => {
      const api = await import('@/api')
      vi.mocked(api.getStorageTargets).mockResolvedValue([])
      vi.mocked(api.postServer).mockResolvedValue(mockCreatedServer)

      const { wrapper } = await mountServerNewView()
      await flushPromises()

      await setSelectValue(wrapper, 1, 'host')
      await wrapper.find('[data-testid="name-input"]').setValue('New Server')
      await wrapper.find('[data-testid="host-input"]').setValue('10.0.0.1')
      await wrapper.find('[data-testid="ssh-user-input"]').setValue('ubuntu')
      await wrapper.find('[data-testid="db-name-input"]').setValue('app')
      await wrapper
        .find('[data-testid="db-user-input"]')
        .setValue('backup_user')

      await wrapper.find('[data-testid="server-form"]').trigger('submit')
      await flushPromises()

      const payload = vi.mocked(api.postServer).mock.calls[0][0]
      expect(payload.deploymentMode).toBe('host')
      expect(payload.containerName).toBeUndefined()
      expect(payload.dbPassword).toBeUndefined()
    })

    it('sends dbPassword when configured for a mysql server', async () => {
      const api = await import('@/api')
      vi.mocked(api.getStorageTargets).mockResolvedValue([])
      vi.mocked(api.postServer).mockResolvedValue(mockCreatedServer)

      const { wrapper } = await mountServerNewView()
      await flushPromises()

      await setSelectValue(wrapper, 0, 'mysql')
      await wrapper.find('[data-testid="name-input"]').setValue('New Server')
      await wrapper.find('[data-testid="host-input"]').setValue('10.0.0.1')
      await wrapper.find('[data-testid="ssh-user-input"]').setValue('ubuntu')
      await wrapper
        .find('[data-testid="container-name-input"]')
        .setValue('mysql-container')
      await wrapper.find('[data-testid="db-name-input"]').setValue('app')
      await wrapper
        .find('[data-testid="db-user-input"]')
        .setValue('backup_user')
      await wrapper.find('[data-testid="db-password-input"]').setValue('s3cret')
      await wrapper
        .find('[data-testid="mysql-dump-extra-args-input"]')
        .setValue('--single-transaction')

      await wrapper.find('[data-testid="server-form"]').trigger('submit')
      await flushPromises()

      expect(api.postServer).toHaveBeenCalledWith(
        expect.objectContaining({
          dbEngine: 'mysql',
          dbPassword: 's3cret',
          mysqlDumpExtraArgs: '--single-transaction',
        })
      )
    })
  })

  describe('dump command preview', () => {
    it('updates reactively as fields change and never shows the real password', async () => {
      const api = await import('@/api')
      vi.mocked(api.getStorageTargets).mockResolvedValue([])

      const { wrapper } = await mountServerNewView()
      await flushPromises()

      await wrapper
        .find('[data-testid="container-name-input"]')
        .setValue('pg-container')
      await wrapper.find('[data-testid="db-user-input"]').setValue('app_user')
      await wrapper.find('[data-testid="db-name-input"]').setValue('app_db')
      await flushPromises()

      const previewText = wrapper
        .findAll('[data-testid="dump-command-preview-line"]')
        .map((el) => el.text())
        .join('\n')
      expect(previewText).toContain('pg_dump')
      expect(previewText).toContain('pg-container')
      expect(previewText).toContain('app_user')
      expect(previewText).toContain('app_db')

      await wrapper
        .find('[data-testid="db-password-input"]')
        .setValue('super-secret-value')
      await flushPromises()

      const previewWithPassword = wrapper
        .findAll('[data-testid="dump-command-preview-line"]')
        .map((el) => el.text())
        .join('\n')
      expect(previewWithPassword).not.toContain('super-secret-value')
      expect(previewWithPassword).toContain('***')
    })
  })
})
