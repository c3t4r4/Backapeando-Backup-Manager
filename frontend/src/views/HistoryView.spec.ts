import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  mount,
  flushPromises,
  DOMWrapper,
  type VueWrapper,
} from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import HistoryView from './HistoryView.vue'
import type { BackupRunDTO, BackupRunListResponse, ServerDTO } from '@/api'

vi.mock('@/api', () => ({
  getServers: vi.fn(),
  getAllBackupRuns: vi.fn(),
}))

function makeBackups(count: number, serverId = 'server-1'): BackupRunDTO[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `backup-${i + 1}`,
    serverId,
    status: 'success',
    startedAt: '2026-09-14T10:00:00Z',
    finishedAt: '2026-09-14T10:15:00Z',
    blobName: `backup-${i + 1}.sql`,
    blobSizeBytes: 1048576,
    createdAt: '2026-09-14T10:00:00Z',
  }))
}

function pageResponse(
  items: BackupRunDTO[],
  overrides: Partial<BackupRunListResponse> = {}
): BackupRunListResponse {
  return { items, total: items.length, page: 1, pageSize: 50, ...overrides }
}

const mockServers: ServerDTO[] = [
  {
    id: 'server-1',
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
]

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/history', component: HistoryView }],
  })
}

let activeWrapper: VueWrapper | null = null

/**
 * BackupRunDetailsDialog teleports its content to document.body (see its
 * own spec for why), so this view must mount attached to the real DOM for
 * that content to render. The wrapper is unmounted after each test to avoid
 * leaking teleported content into the next test's body.
 */
async function mountHistoryView(query = '') {
  const router = createTestRouter()
  await router.push(`/history${query}`)
  await router.isReady()
  const wrapper = mount(HistoryView, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  activeWrapper = wrapper
  await flushPromises()
  return wrapper
}

function findInBody(selector: string): DOMWrapper<Element> {
  return new DOMWrapper(document.body).find(selector)
}

afterEach(() => {
  activeWrapper?.unmount()
  activeWrapper = null
})

describe('HistoryView.vue', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    const api = await import('@/api')
    vi.mocked(api.getServers).mockResolvedValue(mockServers)
    // Default: every test starts from an empty "all servers" page unless it
    // overrides this — the view now fetches on mount regardless of whether
    // a server is selected (RN-BACKUP-026).
    vi.mocked(api.getAllBackupRuns).mockResolvedValue(pageResponse([]))
  })

  it('should render history view on mount and fetch the default (all servers) page', async () => {
    const api = await import('@/api')

    const wrapper = await mountHistoryView()

    expect(wrapper.text()).toContain('Histórico de Backups')
    expect(wrapper.find('[data-testid="refresh-btn"]').exists()).toBe(true)
    expect(api.getAllBackupRuns).toHaveBeenCalledWith({
      page: 1,
      pageSize: 50,
      status: undefined,
      serverId: undefined,
    })
  })

  it('should populate the server dropdown from getServers', async () => {
    const wrapper = await mountHistoryView()

    const options = wrapper.findAll('[data-testid="server-select"] option')
    expect(options.some((o) => o.text() === 'Production DB')).toBe(true)
  })

  it('should not disable the refresh button just because no server is selected', async () => {
    const wrapper = await mountHistoryView()

    const refreshBtn = wrapper.find('[data-testid="refresh-btn"]')
    expect(refreshBtn.attributes('disabled')).toBeUndefined()
  })

  it('should show the last 50 backups across every server by default', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockResolvedValue(
      pageResponse(makeBackups(3, 'server-1'), { total: 3 })
    )

    const wrapper = await mountHistoryView()

    expect(api.getAllBackupRuns).toHaveBeenCalledWith({
      page: 1,
      pageSize: 50,
      status: undefined,
      serverId: undefined,
    })
    expect(wrapper.find('[data-testid="backup-table"]').exists()).toBe(true)
    expect((wrapper.vm as any).backups).toHaveLength(3)
  })

  it('should fetch backups filtered by the selected server', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockResolvedValue(
      pageResponse(makeBackups(1))
    )

    const wrapper = await mountHistoryView()
    await wrapper.find('[data-testid="server-select"]').setValue('server-1')
    await flushPromises()

    expect(api.getAllBackupRuns).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 50,
      status: undefined,
      serverId: 'server-1',
    })
    expect(wrapper.find('[data-testid="backup-table"]').exists()).toBe(true)
  })

  it('should preselect the server from the ?server= query param and filter by it', async () => {
    const api = await import('@/api')

    await mountHistoryView('?server=server-1')

    expect(api.getAllBackupRuns).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 50,
      status: undefined,
      serverId: 'server-1',
    })
  })

  it('should show the empty state when there are no backups to show', async () => {
    const wrapper = await mountHistoryView()

    expect(wrapper.find('[data-testid="empty-state"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="backup-table"]').exists()).toBe(false)
  })

  it('should show an error alert when fetching backups fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockRejectedValue(new Error('boom'))

    const wrapper = await mountHistoryView()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Erro ao carregar histórico')
  })

  it('should refetch when the refresh button is clicked', async () => {
    const api = await import('@/api')

    await mountHistoryView()
    const callsAfterMount = vi.mocked(api.getAllBackupRuns).mock.calls.length

    await activeWrapper!
      .find('[data-testid="refresh-btn"]')
      .trigger('click')
    await flushPromises()

    expect(api.getAllBackupRuns).toHaveBeenCalledTimes(callsAfterMount + 1)
  })

  it('should request page/pageSize=50 from the server and render whatever page it returns', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockResolvedValue(
      pageResponse(makeBackups(10), { total: 25 })
    )

    const wrapper = await mountHistoryView()

    expect((wrapper.vm as any).backups).toHaveLength(10)
    expect((wrapper.vm as any).backups[0].id).toBe('backup-1')
    expect((wrapper.vm as any).totalBackups).toBe(25)
  })

  it('should refetch page 2 from the server when pagination updates', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockResolvedValueOnce(
      pageResponse(makeBackups(50), { total: 60 })
    )

    const wrapper = await mountHistoryView()

    expect((wrapper.vm as any).page).toBe(1)

    vi.mocked(api.getAllBackupRuns).mockResolvedValueOnce(
      pageResponse(makeBackups(10), { total: 60, page: 2 })
    )
    ;(wrapper.vm as any).handlePageChange(2)
    await flushPromises()

    expect((wrapper.vm as any).page).toBe(2)
    expect(api.getAllBackupRuns).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 50,
      status: undefined,
      serverId: undefined,
    })
  })

  it('should reset to page 1 and pass the status filter when it changes', async () => {
    const api = await import('@/api')

    const wrapper = await mountHistoryView()

    ;(wrapper.vm as any).handlePageChange(3)
    await flushPromises()
    expect((wrapper.vm as any).page).toBe(3)

    await wrapper.find('[data-testid="status-select"]').setValue('failed')
    await flushPromises()

    expect((wrapper.vm as any).page).toBe(1)
    expect(api.getAllBackupRuns).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 50,
      status: 'failed',
      serverId: undefined,
    })
  })

  it('should calculate correct total items from the server response, not the page length', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockResolvedValue(
      pageResponse(makeBackups(23), { total: 23 })
    )

    const wrapper = await mountHistoryView()

    expect((wrapper.vm as any).totalBackups).toBe(23)
  })

  it('should show the server name (not a raw id) in the table when servers are loaded', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockResolvedValue(
      pageResponse(makeBackups(1, 'server-1'))
    )

    const wrapper = await mountHistoryView()

    expect(wrapper.find('[data-testid="server-backup-1"]').text()).toBe(
      'Production DB'
    )
  })

  it('should open the details dialog with the clicked run data', async () => {
    const api = await import('@/api')
    vi.mocked(api.getAllBackupRuns).mockResolvedValue(
      pageResponse(makeBackups(1))
    )

    const wrapper = await mountHistoryView()

    expect(findInBody('[data-testid="backup-details-dialog"]').exists()).toBe(
      false
    )

    const detailsBtn = wrapper.find('[data-testid="details-btn-backup-1"]')
    await detailsBtn.trigger('click')
    await flushPromises()

    expect(findInBody('[data-testid="backup-details-dialog"]').exists()).toBe(
      true
    )
    expect(findInBody('[data-testid="detail-blob-name"]').text()).toBe(
      'backup-1.sql'
    )
  })
})
