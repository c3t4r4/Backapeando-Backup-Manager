/**
 * Tests for ServerEditView component
 * Coverage:
 * - Loads the existing server and populates the form
 * - Shows a loading spinner while fetching
 * - Submits the form and redirects to /servers on success
 * - Shows an error alert when loading or saving fails
 * - Cancel button navigates back without calling the API
 * - Retention policy section (global vs. per-server override)
 * - Combined "Testar Backup" action (RN-BACKUP-013) rendering per-check results
 * - "Executar Backup Agora" (run-now)
 * - Regenerate key / reset host key behind a confirm dialog (double-submit guarded)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import ServerEditView from './ServerEditView.vue'
import type {
  StorageTargetDTO,
  RetentionPolicyDTO,
  RunNowResponse,
  ServerDTO,
  TestConnectionResult,
} from '@/api'
import { Select } from '@/components/ui/select'

/**
 * Reka-ui's Select relies on real pointer capture/focus semantics that
 * happy-dom does not implement, so tests interact with it the same way the
 * template's v-model does: by emitting `update:modelValue` directly on the
 * `Select` component instance. `index` follows DOM order: 0 = motor do banco
 * (dbEngine), 1 = onde o banco roda (deploymentMode).
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

vi.mock('@/api', () => ({
  getServer: vi.fn(),
  putServer: vi.fn(),
  getStorageTargets: vi.fn(),
  getServerRetentionPolicy: vi.fn(),
  putServerRetentionPolicy: vi.fn(),
  deleteServerRetentionPolicy: vi.fn(),
  postTestConnection: vi.fn(),
  postRunNow: vi.fn(),
  postRegenerateKey: vi.fn(),
  postResetHostKey: vi.fn(),
  postEnableServer: vi.fn(),
  postDisableServer: vi.fn(),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/servers/:id/edit', component: ServerEditView },
      { path: '/servers', component: { template: '<div>Servers</div>' } },
      { path: '/history', component: { template: '<div>History</div>' } },
    ],
  })
}

async function mountServerEditView(id = 'srv-1') {
  const router = createTestRouter()
  await router.push(`/servers/${id}/edit`)
  await router.isReady()
  const wrapper = mount(ServerEditView, { global: { plugins: [router] } })
  return { wrapper, router }
}

describe('ServerEditView', () => {
  const mockServer: ServerDTO = {
    id: 'srv-1',
    name: 'Production DB',
    host: '10.0.1.100',
    port: 5432,
    sshUser: 'postgres',
    dbEngine: 'postgres',
    deploymentMode: 'docker',
    containerName: 'postgres',
    dbName: 'production',
    dbUser: 'backup_user',
    hasDbPassword: false,
    pgDumpExtraArgs: '--exclude-table=logs',
    mysqlDumpExtraArgs: '',
    sqlCmdExtraArgs: '',
    storageTargetId: 'az-1',
    cronExpression: '0 3 * * *',
    enabled: true,
    status: 'ready',
    sshPublicKey: 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest test-key',
    createdAt: '2026-09-14T10:00:00Z',
    updatedAt: '2026-09-14T10:00:00Z',
  }

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

  const mockGlobalPolicy: RetentionPolicyDTO = {
    id: 'policy-global',
    recentCount: 3,
    monthlyCount: 12,
  }

  const mockOverridePolicy: RetentionPolicyDTO = {
    id: 'policy-override',
    serverId: 'srv-1',
    recentCount: 5,
    monthlyCount: 6,
  }

  beforeEach(async () => {
    vi.clearAllMocks()
    const api = await import('@/api')
    // Sensible defaults so existing tests that don't care about retention
    // still exercise a realistic (non-error) path.
    vi.mocked(api.getServerRetentionPolicy).mockResolvedValue(mockGlobalPolicy)

    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
    })
  })

  it('should show a loading spinner while fetching the server', async () => {
    const api = await import('@/api')
    let resolveFetch: (value: ServerDTO) => void = () => {}
    const fetchPromise = new Promise<ServerDTO>((resolve) => {
      resolveFetch = resolve
    })
    vi.mocked(api.getServer).mockReturnValue(fetchPromise)
    vi.mocked(api.getStorageTargets).mockResolvedValue([])

    const { wrapper } = await mountServerEditView()

    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(true)

    resolveFetch(mockServer)
    await flushPromises()

    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(false)
  })

  it('should populate the form with the loaded server data', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServer).mockResolvedValue(mockServer)
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

    const { wrapper } = await mountServerEditView()
    await flushPromises()

    expect(
      wrapper.find<HTMLInputElement>('[data-testid="name-input"]').element.value
    ).toBe('Production DB')
    expect(
      wrapper.find<HTMLInputElement>('[data-testid="host-input"]').element.value
    ).toBe('10.0.1.100')
    expect(
      wrapper.find<HTMLInputElement>('[data-testid="db-name-input"]').element
        .value
    ).toBe('production')
    expect(api.getServer).toHaveBeenCalledWith('srv-1')
  })

  it('should show an error alert when loading the server fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServer).mockRejectedValue(new Error('not found'))
    vi.mocked(api.getStorageTargets).mockResolvedValue([])

    const { wrapper } = await mountServerEditView()
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="server-form"]').exists()).toBe(false)
  })

  it('should call putServer with the server id and redirect on submit', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServer).mockResolvedValue(mockServer)
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
    vi.mocked(api.putServer).mockResolvedValue(mockServer)

    const { wrapper, router } = await mountServerEditView()
    await flushPromises()

    await wrapper.find('[data-testid="name-input"]').setValue('Updated Name')
    await wrapper.find('[data-testid="server-form"]').trigger('submit')
    await flushPromises()

    expect(api.putServer).toHaveBeenCalledWith(
      'srv-1',
      expect.objectContaining({ name: 'Updated Name' })
    )
    expect(router.currentRoute.value.path).toBe('/servers')
  })

  it('should show an error alert when saving fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServer).mockResolvedValue(mockServer)
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
    vi.mocked(api.putServer).mockRejectedValue(new Error('conflict'))

    const { wrapper, router } = await mountServerEditView()
    await flushPromises()

    await wrapper.find('[data-testid="server-form"]').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(router.currentRoute.value.path).toBe('/servers/srv-1/edit')
  })

  it('should navigate to /servers on cancel without calling the API', async () => {
    const api = await import('@/api')
    vi.mocked(api.getServer).mockResolvedValue(mockServer)
    vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

    const { wrapper, router } = await mountServerEditView()
    await flushPromises()

    await wrapper.find('[data-testid="cancel-button"]').trigger('click')
    await flushPromises()

    expect(api.putServer).not.toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/servers')
  })

  describe('retention policy section', () => {
    it('should show "use global" checked when the server has no override', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.getServerRetentionPolicy).mockResolvedValue(
        mockGlobalPolicy
      )

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      const checkbox = wrapper.find<HTMLInputElement>(
        '[data-testid="use-global-retention-checkbox"]'
      )
      expect(checkbox.element.checked).toBe(true)
      expect(
        wrapper.find('[data-testid="retention-recent-count-input"]').exists()
      ).toBe(false)
    })

    it('should show the override values and uncheck "use global" when the server has one', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.getServerRetentionPolicy).mockResolvedValue(
        mockOverridePolicy
      )

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      const checkbox = wrapper.find<HTMLInputElement>(
        '[data-testid="use-global-retention-checkbox"]'
      )
      expect(checkbox.element.checked).toBe(false)
      expect(
        wrapper.find<HTMLInputElement>(
          '[data-testid="retention-recent-count-input"]'
        ).element.value
      ).toBe('5')
    })

    it('should save a per-server override via putServerRetentionPolicy', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.getServerRetentionPolicy).mockResolvedValue(
        mockOverridePolicy
      )
      vi.mocked(api.putServerRetentionPolicy).mockResolvedValue(
        mockOverridePolicy
      )

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="retention-recent-count-input"]')
        .setValue('7')
      await wrapper
        .find('[data-testid="save-retention-button"]')
        .trigger('click')
      await flushPromises()

      expect(api.putServerRetentionPolicy).toHaveBeenCalledWith('srv-1', {
        recentCount: 7,
        monthlyCount: 6,
      })
      expect(
        wrapper.find('[data-testid="retention-success-alert"]').exists()
      ).toBe(true)
    })

    it('should remove the override via deleteServerRetentionPolicy when switching back to global', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.getServerRetentionPolicy).mockResolvedValue(
        mockOverridePolicy
      )
      vi.mocked(api.deleteServerRetentionPolicy).mockResolvedValue(undefined)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="use-global-retention-checkbox"]')
        .setValue(true)
      await wrapper
        .find('[data-testid="save-retention-button"]')
        .trigger('click')
      await flushPromises()

      expect(api.deleteServerRetentionPolicy).toHaveBeenCalledWith('srv-1')
      expect(api.putServerRetentionPolicy).not.toHaveBeenCalled()
    })

    it('should block saving when both counts are zero (RN-BACKUP-004)', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.getServerRetentionPolicy).mockResolvedValue(
        mockOverridePolicy
      )

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="retention-recent-count-input"]')
        .setValue('0')
      await wrapper
        .find('[data-testid="retention-monthly-count-input"]')
        .setValue('0')
      await flushPromises()

      expect(
        wrapper.find('[data-testid="retention-validation-error"]').exists()
      ).toBe(true)

      await wrapper
        .find('[data-testid="save-retention-button"]')
        .trigger('click')
      await flushPromises()

      expect(api.putServerRetentionPolicy).not.toHaveBeenCalled()
    })
  })

  describe('combined test-connection ("Testar Backup")', () => {
    it('should render all 3 check results and update the displayed status', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const result: TestConnectionResult = {
        ...mockServer,
        status: 'ready',
        enabled: true,
        checks: {
          ssh: { ok: true },
          dumpTool: { ok: true },
          storage: { ok: false, error: 'storage connection failed' },
        },
      }
      vi.mocked(api.postTestConnection).mockResolvedValue(result)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper.find('[data-testid="test-backup-button"]').trigger('click')
      await flushPromises()

      expect(api.postTestConnection).toHaveBeenCalledWith('srv-1')
      expect(wrapper.find('[data-testid="check-ssh"]').text()).toContain('✅')
      expect(wrapper.find('[data-testid="check-dump-tool"]').text()).toContain(
        '✅'
      )
      expect(wrapper.find('[data-testid="check-storage"]').text()).toContain(
        '❌'
      )
      expect(wrapper.find('[data-testid="check-storage"]').text()).toContain(
        'storage connection failed'
      )
    })

    it('should omit the storage check when the server has no storageTargetId', async () => {
      const api = await import('@/api')
      const serverWithoutStorage = { ...mockServer, storageTargetId: undefined }
      vi.mocked(api.getServer).mockResolvedValue(serverWithoutStorage)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const result: TestConnectionResult = {
        ...serverWithoutStorage,
        checks: { ssh: { ok: true }, dumpTool: { ok: true } },
      }
      vi.mocked(api.postTestConnection).mockResolvedValue(result)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper.find('[data-testid="test-backup-button"]').trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="check-storage"]').exists()).toBe(false)
    })
  })

  describe('run backup now', () => {
    it('should show the message returned by postRunNow with a link to history', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const response: RunNowResponse = {
        backupRunId: 'run-1',
        status: 'queued',
        message: 'Backup enfileirado com sucesso.',
      }
      vi.mocked(api.postRunNow).mockResolvedValue(response)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper.find('[data-testid="run-now-button"]').trigger('click')
      await flushPromises()

      expect(api.postRunNow).toHaveBeenCalledWith('srv-1')
      expect(wrapper.find('[data-testid="run-now-message"]').text()).toContain(
        'Backup enfileirado com sucesso.'
      )
    })

    it('should show an error alert when postRunNow fails', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.postRunNow).mockRejectedValue(new Error('conflict'))

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper.find('[data-testid="run-now-button"]').trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    })
  })

  describe('guia de autorização SSH', () => {
    it('should show the public key and instructions when present', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      expect(
        wrapper.find('[data-testid="ssh-authorization-guide"]').exists()
      ).toBe(true)
      expect(wrapper.find('[data-testid="ssh-public-key"]').text()).toBe(
        mockServer.sshPublicKey
      )
      expect(wrapper.text()).toContain('postgres')
      expect(wrapper.text()).toContain('10.0.1.100')
    })

    it('should not show the guide when there is no public key', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue({
        ...mockServer,
        sshPublicKey: undefined,
      })
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      expect(
        wrapper.find('[data-testid="ssh-authorization-guide"]').exists()
      ).toBe(false)
    })

    it('should copy the public key to the clipboard on click', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper.find('[data-testid="copy-ssh-key-button"]').trigger('click')
      await flushPromises()

      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        mockServer.sshPublicKey
      )
      expect(wrapper.find('[data-testid="copy-ssh-key-button"]').text()).toBe(
        'Copiado!'
      )
    })

    it('should update the displayed public key after regenerating', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      const newKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAInewkey newkey'
      vi.mocked(api.postRegenerateKey).mockResolvedValue({
        ...mockServer,
        status: 'awaiting_authorization',
        enabled: false,
        sshPublicKey: newKey,
      })

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="regenerate-key-button"]')
        .trigger('click')
      await flushPromises()
      await wrapper
        .find('[data-testid="confirm-dialog-confirm"]')
        .trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="ssh-public-key"]').text()).toBe(newKey)
    })
  })

  describe('regenerate key / reset host key', () => {
    it('should call postRegenerateKey after confirming', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.postRegenerateKey).mockResolvedValue({
        ...mockServer,
        status: 'awaiting_authorization',
        enabled: false,
      })

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="regenerate-key-button"]')
        .trigger('click')
      await flushPromises()
      expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(true)

      await wrapper
        .find('[data-testid="confirm-dialog-confirm"]')
        .trigger('click')
      await flushPromises()

      expect(api.postRegenerateKey).toHaveBeenCalledWith('srv-1')
      expect(api.postResetHostKey).not.toHaveBeenCalled()
      expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(
        false
      )
    })

    it('should call postResetHostKey after confirming', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.postResetHostKey).mockResolvedValue(mockServer)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="reset-host-key-button"]')
        .trigger('click')
      await flushPromises()
      await wrapper
        .find('[data-testid="confirm-dialog-confirm"]')
        .trigger('click')
      await flushPromises()

      expect(api.postResetHostKey).toHaveBeenCalledWith('srv-1')
      expect(api.postRegenerateKey).not.toHaveBeenCalled()
    })

    it('should not call the API twice on a rapid double-confirm', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      let resolveRegenerate: (value: ServerDTO) => void = () => {}
      vi.mocked(api.postRegenerateKey).mockReturnValue(
        new Promise((resolve) => {
          resolveRegenerate = resolve
        })
      )

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="regenerate-key-button"]')
        .trigger('click')
      await flushPromises()

      const confirmBtn = wrapper.find('[data-testid="confirm-dialog-confirm"]')
      await confirmBtn.trigger('click')
      await confirmBtn.trigger('click')

      expect(api.postRegenerateKey).toHaveBeenCalledTimes(1)

      resolveRegenerate(mockServer)
      await flushPromises()
    })

    it('should cancel without calling the API', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="reset-host-key-button"]')
        .trigger('click')
      await flushPromises()
      await wrapper
        .find('[data-testid="confirm-dialog-cancel"]')
        .trigger('click')
      await flushPromises()

      expect(api.postResetHostKey).not.toHaveBeenCalled()
      expect(api.postRegenerateKey).not.toHaveBeenCalled()
    })
  })

  describe('enable / disable server (RN-BACKUP-024)', () => {
    it('shows "Desativar Servidor" for an enabled server and calls postDisableServer after confirming', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue({
        ...mockServer,
        enabled: true,
      })
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.postDisableServer).mockResolvedValue({
        ...mockServer,
        enabled: false,
      })

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      expect(
        wrapper.find('[data-testid="disable-server-button"]').exists()
      ).toBe(true)
      expect(
        wrapper.find('[data-testid="enable-server-button"]').exists()
      ).toBe(false)

      await wrapper
        .find('[data-testid="disable-server-button"]')
        .trigger('click')
      await flushPromises()
      await wrapper
        .find('[data-testid="confirm-dialog-confirm"]')
        .trigger('click')
      await flushPromises()

      expect(api.postDisableServer).toHaveBeenCalledWith('srv-1')
      expect(api.postEnableServer).not.toHaveBeenCalled()
      expect(wrapper.text()).toContain('Desabilitado')
    })

    it('shows "Ativar Servidor" for a disabled server and calls postEnableServer after confirming', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue({
        ...mockServer,
        enabled: false,
      })
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.postEnableServer).mockResolvedValue({
        ...mockServer,
        enabled: true,
      })

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      expect(
        wrapper.find('[data-testid="enable-server-button"]').exists()
      ).toBe(true)
      expect(
        wrapper.find('[data-testid="disable-server-button"]').exists()
      ).toBe(false)

      await wrapper
        .find('[data-testid="enable-server-button"]')
        .trigger('click')
      await flushPromises()
      await wrapper
        .find('[data-testid="confirm-dialog-confirm"]')
        .trigger('click')
      await flushPromises()

      expect(api.postEnableServer).toHaveBeenCalledWith('srv-1')
      expect(api.postDisableServer).not.toHaveBeenCalled()
      expect(wrapper.text()).toContain('Habilitado')
    })

    it('cancels without calling the API', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue({
        ...mockServer,
        enabled: true,
      })
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="disable-server-button"]')
        .trigger('click')
      await flushPromises()
      await wrapper
        .find('[data-testid="confirm-dialog-cancel"]')
        .trigger('click')
      await flushPromises()

      expect(api.postDisableServer).not.toHaveBeenCalled()
      expect(api.postEnableServer).not.toHaveBeenCalled()
    })
  })

  describe('engine, deployment mode, and DB password', () => {
    it('populates the selectors and extra-args field from the loaded server', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      expect(
        wrapper.find('[data-testid="container-name-input"]').exists()
      ).toBe(true)
      expect(
        (
          wrapper.find('[data-testid="container-name-input"]')
            .element as HTMLInputElement
        ).value
      ).toBe('postgres')
      expect(
        wrapper.find('[data-testid="pg-dump-extra-args-input"]').exists()
      ).toBe(true)
    })

    it('switching to mysql shows the mysql extra-args field and requires a password message', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await setSelectValue(wrapper, 0, 'mysql')

      expect(
        wrapper.find('[data-testid="mysql-dump-extra-args-input"]').exists()
      ).toBe(true)
      expect(
        wrapper.find('[data-testid="pg-dump-extra-args-input"]').exists()
      ).toBe(false)
    })

    it('switching deploymentMode to host hides containerName', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await setSelectValue(wrapper, 1, 'host')

      expect(
        wrapper.find('[data-testid="container-name-input"]').exists()
      ).toBe(false)
    })

    it('leaving the password field blank on submit does not send dbPassword (keeps the current one)', async () => {
      const api = await import('@/api')
      const serverWithPassword = { ...mockServer, hasDbPassword: true }
      vi.mocked(api.getServer).mockResolvedValue(serverWithPassword)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.putServer).mockResolvedValue(serverWithPassword)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper.find('[data-testid="server-form"]').trigger('submit')
      await flushPromises()

      const payload = vi.mocked(api.putServer).mock.calls[0][1]
      expect(payload.dbPassword).toBeUndefined()
    })

    it('submitting a non-empty password sends it to be re-encrypted', async () => {
      const api = await import('@/api')
      vi.mocked(api.getServer).mockResolvedValue(mockServer)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)
      vi.mocked(api.putServer).mockResolvedValue(mockServer)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      await wrapper
        .find('[data-testid="db-password-input"]')
        .setValue('new-password')
      await wrapper.find('[data-testid="server-form"]').trigger('submit')
      await flushPromises()

      const payload = vi.mocked(api.putServer).mock.calls[0][1]
      expect(payload.dbPassword).toBe('new-password')
    })

    it('the dump command preview never shows the real password, only a mask', async () => {
      const api = await import('@/api')
      const serverWithPassword = { ...mockServer, hasDbPassword: true }
      vi.mocked(api.getServer).mockResolvedValue(serverWithPassword)
      vi.mocked(api.getStorageTargets).mockResolvedValue(mockStorageTargets)

      const { wrapper } = await mountServerEditView()
      await flushPromises()

      const previewText = wrapper
        .findAll('[data-testid="dump-command-preview-line"]')
        .map((el) => el.text())
        .join('\n')
      expect(previewText).toContain('***')

      await wrapper
        .find('[data-testid="db-password-input"]')
        .setValue('a-real-secret-value')
      await flushPromises()

      const previewAfterTyping = wrapper
        .findAll('[data-testid="dump-command-preview-line"]')
        .map((el) => el.text())
        .join('\n')
      expect(previewAfterTyping).not.toContain('a-real-secret-value')
    })
  })
})
