/**
 * Tests for DashboardView component (analytics)
 * Coverage:
 * - Loads summary + backup-stats on mount and renders KPIs
 * - Renders the status-distribution doughnut and the 4 per-destination
 *   stacked bar charts (vue-chartjs mocked — Chart.js needs a real <canvas>
 *   context that happy-dom doesn't provide)
 * - Renders attention points (awaiting authorization, connection errors,
 *   recent failures)
 * - Shows empty state when there are no attention points
 * - Handles API errors gracefully, including partial Promise.all failure
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import DashboardView from './DashboardView.vue'
import type { DashboardSummaryDTO, DashboardBackupStatsDTO } from '@/api'

vi.mock('@/api', () => ({
  getDashboardSummary: vi.fn(),
  getDashboardBackupStats: vi.fn(),
}))

vi.mock('vue-chartjs', () => ({
  Doughnut: { template: '<div data-testid="doughnut-stub" />' },
  Bar: { template: '<div data-testid="bar-stub" />' },
}))

import * as api from '@/api'

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard', component: DashboardView },
      { path: '/servers/:id/edit', component: { template: '<div>Edit</div>' } },
    ],
  })
}

async function mountDashboardView() {
  const router = createTestRouter()
  await router.push('/dashboard')
  await router.isReady()

  const wrapper = mount(DashboardView, {
    global: { plugins: [router] },
  })
  await flushPromises()
  return wrapper
}

const emptySummary: DashboardSummaryDTO = {
  servers: {
    total: 3,
    byStatus: {
      pendingKey: 0,
      awaitingAuthorization: 0,
      ready: 3,
      disabled: 0,
    },
    withConnectionError: 0,
  },
  backupRunsLast30Days: {
    since: '2026-08-16T00:00:00Z',
    total: 10,
    success: 9,
    failed: 1,
    successRatePercent: 90,
  },
  attentionPoints: {
    serversAwaitingAuthorization: [],
    serversWithConnectionError: [],
    recentFailures: [],
  },
}

const emptyBackupStats: DashboardBackupStatsDTO = {
  destinations: [],
  dailyLast30Days: { labels: [], countSeries: [], bytesSeries: [] },
  monthlyThisYear: { labels: [], countSeries: [], bytesSeries: [] },
}

describe('DashboardView', () => {
  beforeEach(() => {
    vi.mocked(api.getDashboardSummary).mockReset()
    vi.mocked(api.getDashboardBackupStats).mockReset()
    vi.mocked(api.getDashboardBackupStats).mockResolvedValue(emptyBackupStats)
  })

  it('should fetch the summary and backup stats on mount and render KPIs', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue(emptySummary)

    const wrapper = await mountDashboardView()

    expect(api.getDashboardSummary).toHaveBeenCalledTimes(1)
    expect(api.getDashboardBackupStats).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="kpi-total-servers"]').text()).toContain(
      '3'
    )
    expect(wrapper.find('[data-testid="kpi-ready-servers"]').text()).toContain(
      '3'
    )
    expect(
      wrapper.find('[data-testid="kpi-connection-error"]').text()
    ).toContain('0')
    expect(wrapper.find('[data-testid="kpi-backups-30d"]').text()).toContain(
      '10'
    )
    expect(wrapper.find('[data-testid="kpi-success-rate"]').text()).toContain(
      '90.0%'
    )
  })

  it('should render the status doughnut and the 4 per-destination charts in their empty state', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue(emptySummary)

    const wrapper = await mountDashboardView()

    expect(wrapper.find('[data-testid="doughnut-stub"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="bar-stub"]').exists()).toBe(false)

    expect(wrapper.find('[data-testid="chart-daily-count"]').text()).toContain(
      'Nenhum backup nos últimos 30 dias.'
    )
    expect(wrapper.find('[data-testid="chart-daily-bytes"]').text()).toContain(
      'Nenhum backup nos últimos 30 dias.'
    )
    expect(
      wrapper.find('[data-testid="chart-monthly-count"]').text()
    ).toContain('Nenhum backup no ano corrente.')
    expect(
      wrapper.find('[data-testid="chart-monthly-bytes"]').text()
    ).toContain('Nenhum backup no ano corrente.')
  })

  it('should render the 4 per-destination charts when there is backup data', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue(emptySummary)
    vi.mocked(api.getDashboardBackupStats).mockResolvedValue({
      destinations: [
        { storageTargetId: 'target-1', name: 'Azure Produção' },
        { storageTargetId: null, name: 'Sem destino' },
      ],
      dailyLast30Days: {
        since: '2026-08-18T00:00:00Z',
        until: '2026-09-17T00:00:00Z',
        labels: ['2026-09-16'],
        countSeries: [
          { storageTargetId: 'target-1', data: [3] },
          { storageTargetId: null, data: [0] },
        ],
        bytesSeries: [
          { storageTargetId: 'target-1', data: [1024 * 1024 * 5] },
          { storageTargetId: null, data: [0] },
        ],
      },
      monthlyThisYear: {
        year: 2026,
        labels: ['2026-09'],
        countSeries: [
          { storageTargetId: 'target-1', data: [3] },
          { storageTargetId: null, data: [0] },
        ],
        bytesSeries: [
          { storageTargetId: 'target-1', data: [1024 * 1024 * 5] },
          { storageTargetId: null, data: [0] },
        ],
      },
    })

    const wrapper = await mountDashboardView()

    const barStubs = wrapper.findAll('[data-testid="bar-stub"]')
    expect(barStubs.length).toBe(4)
    expect(
      wrapper.find('[data-testid="chart-daily-count"]').text()
    ).not.toContain('Nenhum backup')
  })

  it('should show an error message when only the backup-stats call fails', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue(emptySummary)
    vi.mocked(api.getDashboardBackupStats).mockRejectedValue(
      new Error('network error')
    )

    const wrapper = await mountDashboardView()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
  })

  it('should show the empty state when there are no attention points', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue(emptySummary)

    const wrapper = await mountDashboardView()

    expect(wrapper.find('[data-testid="attention-empty"]').exists()).toBe(true)
  })

  it('should render servers awaiting authorization as an attention point', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue({
      ...emptySummary,
      attentionPoints: {
        ...emptySummary.attentionPoints,
        serversAwaitingAuthorization: [
          {
            id: 'srv-1',
            name: 'db-prod',
            host: '10.0.0.1',
            port: 22,
            sshUser: 'postgres',
            containerName: 'pg',
            dbName: 'db',
            dbUser: 'user',
            pgDumpExtraArgs: '',
            cronExpression: '0 3 * * *',
            enabled: false,
            status: 'awaiting_authorization',
            createdAt: '2026-09-01T00:00:00Z',
            updatedAt: '2026-09-01T00:00:00Z',
          },
        ],
      },
    })

    const wrapper = await mountDashboardView()

    const section = wrapper.find('[data-testid="attention-awaiting-auth"]')
    expect(section.exists()).toBe(true)
    expect(section.text()).toContain('db-prod')
    expect(wrapper.find('[data-testid="attention-empty"]').exists()).toBe(false)
  })

  it('should render servers with connection error as an attention point', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue({
      ...emptySummary,
      attentionPoints: {
        ...emptySummary.attentionPoints,
        serversWithConnectionError: [
          {
            id: 'srv-2',
            name: 'db-staging',
            host: '10.0.0.2',
            port: 22,
            sshUser: 'postgres',
            containerName: 'pg',
            dbName: 'db',
            dbUser: 'user',
            pgDumpExtraArgs: '',
            cronExpression: '0 3 * * *',
            enabled: true,
            status: 'ready',
            lastTestConnectionError: 'connection refused',
            createdAt: '2026-09-01T00:00:00Z',
            updatedAt: '2026-09-01T00:00:00Z',
          },
        ],
      },
    })

    const wrapper = await mountDashboardView()

    const section = wrapper.find('[data-testid="attention-connection-error"]')
    expect(section.exists()).toBe(true)
    expect(section.text()).toContain('db-staging')
    expect(section.text()).toContain('connection refused')
  })

  it('should render recent backup failures as an attention point', async () => {
    vi.mocked(api.getDashboardSummary).mockResolvedValue({
      ...emptySummary,
      attentionPoints: {
        ...emptySummary.attentionPoints,
        recentFailures: [
          {
            id: 'run-2',
            serverId: 'srv-3',
            status: 'failed',
            errorMessage:
              'backup failed during backup; see server logs for details',
            createdAt: '2026-09-10T08:00:00Z',
          },
        ],
      },
    })

    const wrapper = await mountDashboardView()

    const section = wrapper.find('[data-testid="attention-recent-failures"]')
    expect(section.exists()).toBe(true)
    expect(section.text()).toContain('backup failed during backup')
  })

  it('should show an error message when the API call fails', async () => {
    vi.mocked(api.getDashboardSummary).mockRejectedValue(
      new Error('network error')
    )

    const wrapper = await mountDashboardView()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="kpi-grid"]').exists()).toBe(false)
  })
})
