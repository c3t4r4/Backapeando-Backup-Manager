/**
 * Tests for ServerList component
 * Coverage:
 * - Renders table with server rows
 * - Displays server properties correctly
 * - Shows status badges with correct styling
 * - Emits events on button clicks
 */

import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ServerList from './ServerList.vue'
import type { ServerDTO } from '@/api'

describe('ServerList', () => {
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
    {
      id: '3',
      name: 'Dev DB',
      host: '10.0.3.10',
      port: 5432,
      sshUser: 'postgres',
      containerName: 'postgres',
      dbName: 'development',
      dbUser: 'backup_user',
      pgDumpExtraArgs: '',
      cronExpression: '0 5 * * *',
      enabled: true,
      status: 'pending_key',
      createdAt: '2026-09-14T12:00:00Z',
      updatedAt: '2026-09-14T12:00:00Z',
    },
  ]

  it('should render table with server rows', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: mockServers,
      },
    })

    // Check table exists
    expect(wrapper.find('[data-testid="servers-table"]').exists()).toBe(true)

    // Check all rows are rendered
    const rows = wrapper.findAll('[data-testid^="server-row-"]')
    expect(rows).toHaveLength(3)

    // Check specific row
    expect(wrapper.find('[data-testid="server-row-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="server-row-2"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="server-row-3"]').exists()).toBe(true)
  })

  it('should display server properties correctly', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: mockServers,
      },
    })

    const firstRow = wrapper.find('[data-testid="server-row-1"]')
    const firstRowText = firstRow.text()

    expect(firstRowText).toContain('Production DB')
    expect(firstRowText).toContain('10.0.1.100')
    expect(firstRowText).toContain('5432')

    const secondRow = wrapper.find('[data-testid="server-row-2"]')
    const secondRowText = secondRow.text()

    expect(secondRowText).toContain('Staging DB')
    expect(secondRowText).toContain('10.0.2.50')
  })

  it('should show status badge with correct styling for ready status', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: [mockServers[0]], // ready status
      },
    })

    const badge = wrapper.find('[data-testid="status-badge-1"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('Pronto')
    expect(badge.classes()).toContain('bg-green-100')
    expect(badge.classes()).toContain('text-green-800')
  })

  it('should show status badge with correct styling for awaiting_authorization status', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: [mockServers[1]], // awaiting_authorization status
      },
    })

    const badge = wrapper.find('[data-testid="status-badge-2"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('Aguardando Autorização')
    expect(badge.classes()).toContain('bg-orange-100')
    expect(badge.classes()).toContain('text-orange-800')
  })

  it('should show status badge with correct styling for pending_key status', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: [mockServers[2]], // pending_key status
      },
    })

    const badge = wrapper.find('[data-testid="status-badge-3"]')
    expect(badge.exists()).toBe(true)
    expect(badge.text()).toBe('Aguardando Chave')
    expect(badge.classes()).toContain('bg-yellow-100')
    expect(badge.classes()).toContain('text-yellow-800')
  })

  it('should emit edit event on edit button click', async () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: mockServers,
      },
    })

    const editButton = wrapper.find('[data-testid="edit-btn-1"]')
    await editButton.trigger('click')

    expect(wrapper.emitted('edit')).toBeTruthy()
    expect(wrapper.emitted('edit')?.[0]).toEqual(['1'])
  })

  it('should emit delete event on delete button click', async () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: mockServers,
      },
    })

    const deleteButton = wrapper.find('[data-testid="delete-btn-2"]')
    await deleteButton.trigger('click')

    expect(wrapper.emitted('delete')).toBeTruthy()
    expect(wrapper.emitted('delete')?.[0]).toEqual(['2'])
  })

  it('should emit view-backups event on backups button click', async () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: mockServers,
      },
    })

    const backupsButton = wrapper.find('[data-testid="backups-btn-3"]')
    await backupsButton.trigger('click')

    expect(wrapper.emitted('view-backups')).toBeTruthy()
    expect(wrapper.emitted('view-backups')?.[0]).toEqual(['3'])
  })

  it('should render empty table when servers list is empty', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: [],
      },
    })

    // Table should still render but with no body rows
    expect(wrapper.find('[data-testid="servers-table"]').exists()).toBe(true)

    const rows = wrapper.findAll('[data-testid^="server-row-"]')
    expect(rows).toHaveLength(0)
  })

  it('should render all action buttons for each row', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: [mockServers[0]],
      },
    })

    expect(wrapper.find('[data-testid="edit-btn-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="delete-btn-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="backups-btn-1"]').exists()).toBe(true)
  })

  it('should have table header with all columns', () => {
    const wrapper = mount(ServerList, {
      props: {
        servers: mockServers,
      },
    })

    const headerText = wrapper.find('thead').text()
    expect(headerText).toContain('Nome')
    expect(headerText).toContain('Host')
    expect(headerText).toContain('Porta')
    expect(headerText).toContain('Status')
    expect(headerText).toContain('Ações')
  })

  it('should log security warning for unknown server status', () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    // Create a server with unknown status
    const unknownServer = {
      ...mockServers[0],
      id: '999',
      name: 'Corrupted Server',
      status: 'corrupted_status', // Invalid status that doesn't match any case
    } as any

    const wrapper = mount(ServerList, {
      props: {
        servers: [unknownServer],
      },
    })

    // Verify: console.warn was called with [SECURITY] prefix
    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining('[SECURITY] Unknown server status from API: corrupted_status')
    )

    // Verify: Badge renders with fallback gray styling for unknown status
    const badge = wrapper.find('[data-testid="status-badge-999"]')
    expect(badge.exists()).toBe(true)
    expect(badge.classes()).toContain('bg-gray-100')
    expect(badge.classes()).toContain('text-gray-800')
    // Badge should display the raw status value as fallback
    expect(badge.text()).toBe('corrupted_status')

    warnSpy.mockRestore()
  })
})
