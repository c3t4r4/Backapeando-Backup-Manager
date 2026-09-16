import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import BackupTable from './BackupTable.vue'
import type { BackupRunDTO } from '@/api'

describe('BackupTable.vue', () => {
  const mockBackups: BackupRunDTO[] = [
    {
      id: 'backup-1',
      serverId: '550e8400-e29b-41d4-a716-446655440000',
      status: 'success',
      startedAt: '2026-09-14T10:00:00Z',
      finishedAt: '2026-09-14T10:15:00Z',
      blobName: 'backup-2026-09-14.sql',
      blobSizeBytes: 1048576, // 1 MB
      createdAt: '2026-09-14T10:00:00Z',
    },
    {
      id: 'backup-2',
      serverId: '550e8400-e29b-41d4-a716-446655440001',
      status: 'failed',
      startedAt: '2026-09-13T10:00:00Z',
      finishedAt: undefined,
      blobName: undefined,
      blobSizeBytes: undefined,
      errorMessage: 'SSH connection timeout',
      createdAt: '2026-09-13T10:00:00Z',
    },
    {
      id: 'backup-3',
      serverId: '550e8400-e29b-41d4-a716-446655440002',
      status: 'queued',
      startedAt: undefined,
      finishedAt: undefined,
      createdAt: '2026-09-14T09:00:00Z',
    },
  ]

  it('should render table with backup rows', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: mockBackups,
      },
    })

    expect(wrapper.find('[data-testid="backups-table"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="backup-row-backup-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="backup-row-backup-2"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="backup-row-backup-3"]').exists()).toBe(true)
  })

  it('should display backup properties (server, dates, status, size)', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: mockBackups,
      },
    })

    // Check first backup (success)
    expect(wrapper.find('[data-testid="server-backup-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="started-backup-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="finished-backup-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="status-badge-backup-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="size-backup-1"]').exists()).toBe(true)

    // Check that content is rendered
    const statusBadge = wrapper.find('[data-testid="status-badge-backup-1"]')
    expect(statusBadge.text()).toContain('Sucesso')
  })

  it('should show status badge with correct styling for all 4 statuses', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: mockBackups,
      },
    })

    // Success badge (green)
    const successBadge = wrapper.find('[data-testid="status-badge-backup-1"]')
    expect(successBadge.classes()).toContain('bg-green-100')
    expect(successBadge.classes()).toContain('text-green-800')
    expect(successBadge.text()).toContain('Sucesso')

    // Failed badge (red)
    const failedBadge = wrapper.find('[data-testid="status-badge-backup-2"]')
    expect(failedBadge.classes()).toContain('bg-red-100')
    expect(failedBadge.classes()).toContain('text-red-800')
    expect(failedBadge.text()).toContain('Falhou')

    // Queued badge (yellow)
    const queuedBadge = wrapper.find('[data-testid="status-badge-backup-3"]')
    expect(queuedBadge.classes()).toContain('bg-yellow-100')
    expect(queuedBadge.classes()).toContain('text-yellow-800')
    expect(queuedBadge.text()).toContain('Enfileirado')
  })

  it('should format dates to locale PT-BR format', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: [mockBackups[0]], // success backup with dates
      },
    })

    const startedCell = wrapper.find('[data-testid="started-backup-1"]')
    expect(startedCell.text()).toContain('14/09/2026')
  })

  it('should format size from bytes to human-readable format', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: [mockBackups[0]], // 1MB backup
      },
    })

    const sizeCell = wrapper.find('[data-testid="size-backup-1"]')
    expect(sizeCell.text()).toContain('1.00 MB')
  })

  it('should show dash (—) for missing dates and sizes', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: [mockBackups[2]], // queued backup without dates/size
      },
    })

    const startedCell = wrapper.find('[data-testid="started-backup-3"]')
    const finishedCell = wrapper.find('[data-testid="finished-backup-3"]')
    const sizeCell = wrapper.find('[data-testid="size-backup-3"]')

    expect(startedCell.text()).toBe('—')
    expect(finishedCell.text()).toBe('—')
    expect(sizeCell.text()).toBe('—')
  })

  it('should emit view-details event on details button click', async () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: [mockBackups[0]],
      },
    })

    const detailsBtn = wrapper.find('[data-testid="details-btn-backup-1"]')
    await detailsBtn.trigger('click')

    expect(wrapper.emitted('view-details')).toBeTruthy()
    expect(wrapper.emitted('view-details')?.[0]).toEqual(['backup-1'])
  })

  it('should render empty state when no backups', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: [],
      },
    })

    expect(wrapper.find('[data-testid="empty-backups"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Nenhum backup encontrado')
  })

  it('should display correct table headers', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: mockBackups,
      },
    })

    const headers = ['Servidor', 'Iniciado', 'Concluído', 'Status', 'Tamanho', 'Ações']
    headers.forEach((header) => {
      expect(wrapper.text()).toContain(header)
    })
  })

  it('should format server ID to show last 8 chars in uppercase', () => {
    const wrapper = mount(BackupTable, {
      props: {
        backups: [mockBackups[0]],
      },
    })

    const serverCell = wrapper.find('[data-testid="server-backup-1"]')
    // UUID: 550e8400-e29b-41d4-a716-446655440000
    // Last 8 chars: 55440000
    expect(serverCell.text()).toContain('55440000')
  })

  it('should handle large file sizes (GB range)', () => {
    const largeBackup: BackupRunDTO = {
      ...mockBackups[0],
      blobSizeBytes: 5368709120, // 5 GB
    }

    const wrapper = mount(BackupTable, {
      props: {
        backups: [largeBackup],
      },
    })

    const sizeCell = wrapper.find('[data-testid="size-backup-1"]')
    expect(sizeCell.text()).toContain('5.00 GB')
  })

  it('should show running status with blue badge', () => {
    const runningBackup: BackupRunDTO = {
      ...mockBackups[0],
      status: 'running',
    }

    const wrapper = mount(BackupTable, {
      props: {
        backups: [runningBackup],
      },
    })

    const badge = wrapper.find('[data-testid="status-badge-backup-1"]')
    expect(badge.classes()).toContain('bg-blue-100')
    expect(badge.classes()).toContain('text-blue-800')
    expect(badge.text()).toContain('Executando')
  })
})
