/**
 * Tests for StorageTargetList component
 * Coverage:
 * - Renders table with target rows
 * - Displays target properties correctly per type (azure/s3/filesystem)
 * - Emits events on button clicks
 * - Empty table when no targets
 */

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import StorageTargetList from './StorageTargetList.vue'
import type { StorageTargetDTO } from '@/api'

describe('StorageTargetList', () => {
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
    {
      id: '3',
      name: 'Local Storage',
      type: 'filesystem',
      rootPath: '/mnt/backups',
      createdAt: '2026-09-14T12:00:00Z',
    },
  ]

  it('should render table with target rows', () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: mockTargets },
    })

    expect(
      wrapper.find('[data-testid="storage-targets-table"]').exists()
    ).toBe(true)

    const rows = wrapper.findAll('[data-testid^="storage-target-row-"]')
    expect(rows).toHaveLength(3)
    expect(
      wrapper.find('[data-testid="storage-target-row-1"]').exists()
    ).toBe(true)
  })

  it('should display azure target properties correctly', () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: mockTargets },
    })

    const firstRow = wrapper.find('[data-testid="storage-target-row-1"]')
    expect(firstRow.text()).toContain('Primary Storage')
    expect(firstRow.text()).toContain('Azure')
    expect(firstRow.text()).toContain('backapeandoprod')
    expect(firstRow.text()).toContain('backups')
  })

  it('should display s3 target properties correctly', () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: mockTargets },
    })

    const secondRow = wrapper.find('[data-testid="storage-target-row-2"]')
    expect(secondRow.text()).toContain('Secondary Storage')
    expect(secondRow.text()).toContain('S3')
    expect(secondRow.text()).toContain('backups-dev')
    expect(secondRow.text()).toContain('us-east-1')
  })

  it('should display filesystem target properties correctly', () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: mockTargets },
    })

    const thirdRow = wrapper.find('[data-testid="storage-target-row-3"]')
    expect(thirdRow.text()).toContain('Local Storage')
    expect(thirdRow.text()).toContain('Local/NFS')
    expect(thirdRow.text()).toContain('/mnt/backups')
  })

  it('should emit edit event on edit button click', async () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: mockTargets },
    })

    await wrapper.find('[data-testid="edit-btn-1"]').trigger('click')

    expect(wrapper.emitted('edit')).toBeTruthy()
    expect(wrapper.emitted('edit')?.[0]).toEqual(['1'])
  })

  it('should emit delete event on delete button click', async () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: mockTargets },
    })

    await wrapper.find('[data-testid="delete-btn-2"]').trigger('click')

    expect(wrapper.emitted('delete')).toBeTruthy()
    expect(wrapper.emitted('delete')?.[0]).toEqual(['2'])
  })

  it('should render empty table when targets list is empty', () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: [] },
    })

    expect(
      wrapper.find('[data-testid="storage-targets-table"]').exists()
    ).toBe(true)
    expect(
      wrapper.findAll('[data-testid^="storage-target-row-"]')
    ).toHaveLength(0)
  })

  it('should have table header with all columns', () => {
    const wrapper = mount(StorageTargetList, {
      props: { targets: mockTargets },
    })

    const headerText = wrapper.find('thead').text()
    expect(headerText).toContain('Nome')
    expect(headerText).toContain('Tipo')
    expect(headerText).toContain('Detalhes')
    expect(headerText).toContain('Ações')
  })
})
