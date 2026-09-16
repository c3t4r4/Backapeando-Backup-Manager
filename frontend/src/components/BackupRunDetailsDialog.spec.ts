import { describe, it, expect, afterEach } from 'vitest'
import {
  mount,
  flushPromises,
  DOMWrapper,
  type VueWrapper,
} from '@vue/test-utils'
import BackupRunDetailsDialog from './BackupRunDetailsDialog.vue'
import { Dialog } from '@/components/ui/dialog'
import type { BackupRunDTO } from '@/api'

const mockBackup: BackupRunDTO = {
  id: 'backup-1',
  serverId: 'server-1234567890',
  status: 'success',
  startedAt: '2026-09-14T10:00:00Z',
  finishedAt: '2026-09-14T10:15:00Z',
  blobName: 'backup-1.sql',
  blobSizeBytes: 1048576,
  dumpDurationMs: 12345,
  uploadDurationMs: 6789,
  createdAt: '2026-09-14T10:00:00Z',
}

let activeWrapper: VueWrapper | null = null

/**
 * DialogContent renders via <Teleport> to document.body, moving its content
 * out of the component tree Vue Test Utils tracks (`wrapper.find` can't see
 * it). Mounting with `attachTo: document.body` makes the teleport actually
 * land in the real DOM, so assertions query `document.body` instead of the
 * wrapper. The wrapper is unmounted after each test to remove that content
 * again — otherwise it would leak into the next test's body.
 */
async function mountDialog(backup: BackupRunDTO | null) {
  activeWrapper = mount(BackupRunDetailsDialog, {
    props: { backup },
    attachTo: document.body,
  })
  await flushPromises()
  return activeWrapper
}

function findInBody(selector: string): DOMWrapper<Element> {
  return new DOMWrapper(document.body).find(selector)
}

afterEach(() => {
  activeWrapper?.unmount()
  activeWrapper = null
})

describe('BackupRunDetailsDialog', () => {
  it('does not render dialog content when backup is null', async () => {
    await mountDialog(null)

    expect(findInBody('[data-testid="backup-details-dialog"]').exists()).toBe(
      false
    )
  })

  it('renders run details when backup is provided', async () => {
    await mountDialog(mockBackup)

    expect(findInBody('[data-testid="backup-details-dialog"]').exists()).toBe(
      true
    )
    expect(findInBody('[data-testid="detail-status"]').text()).toBe('Sucesso')
    expect(findInBody('[data-testid="detail-blob-name"]').text()).toBe(
      'backup-1.sql'
    )
    expect(findInBody('[data-testid="detail-size"]').text()).toBe('1.00 MB')
  })

  it('shows the error message when the run failed', async () => {
    const failedBackup: BackupRunDTO = {
      ...mockBackup,
      status: 'failed',
      errorMessage: 'backup failed during SSH connection',
    }
    await mountDialog(failedBackup)

    expect(findInBody('[data-testid="detail-error-message"]').text()).toBe(
      'backup failed during SSH connection'
    )
  })

  it('does not show error message or log sections when absent', async () => {
    await mountDialog(mockBackup)

    expect(findInBody('[data-testid="detail-error-message"]').exists()).toBe(
      false
    )
    expect(findInBody('[data-testid="detail-log-output"]').exists()).toBe(false)
  })

  it('emits close when the underlying dialog reports it closed', async () => {
    const wrapper = await mountDialog(mockBackup)

    await wrapper.findComponent(Dialog).vm.$emit('update:open', false)

    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
