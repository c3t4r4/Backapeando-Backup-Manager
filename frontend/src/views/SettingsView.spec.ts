/**
 * Tests for SettingsView component (global retention policy)
 * Coverage:
 * - Loads and populates the global retention policy
 * - Shows a loading spinner while fetching
 * - Saves changes via putDefaultRetentionPolicy
 * - Client-side validation mirroring RN-BACKUP-004 (not both zero)
 * - Error handling for load/save failures
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SettingsView from './SettingsView.vue'
import type { RetentionPolicyDTO } from '@/api'

vi.mock('@/api', () => ({
  getDefaultRetentionPolicy: vi.fn(),
  putDefaultRetentionPolicy: vi.fn(),
}))

describe('SettingsView', () => {
  const mockPolicy: RetentionPolicyDTO = {
    id: 'policy-1',
    recentCount: 3,
    monthlyCount: 12,
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should show a loading spinner while fetching the policy', async () => {
    const api = await import('@/api')
    let resolveFetch: (value: RetentionPolicyDTO) => void = () => {}
    vi.mocked(api.getDefaultRetentionPolicy).mockReturnValue(
      new Promise((resolve) => {
        resolveFetch = resolve
      })
    )

    const wrapper = mount(SettingsView)

    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(true)

    resolveFetch(mockPolicy)
    await flushPromises()

    expect(wrapper.find('[data-testid="loading-spinner"]').exists()).toBe(false)
  })

  it('should populate the form with the loaded policy', async () => {
    const api = await import('@/api')
    vi.mocked(api.getDefaultRetentionPolicy).mockResolvedValue(mockPolicy)

    const wrapper = mount(SettingsView)
    await flushPromises()

    expect(
      wrapper.find<HTMLInputElement>('[data-testid="recent-count-input"]')
        .element.value
    ).toBe('3')
    expect(
      wrapper.find<HTMLInputElement>('[data-testid="monthly-count-input"]')
        .element.value
    ).toBe('12')
  })

  it('should show an error alert when loading fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getDefaultRetentionPolicy).mockRejectedValue(
      new Error('boom')
    )

    const wrapper = mount(SettingsView)
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
  })

  it('should save the policy and show a success message', async () => {
    const api = await import('@/api')
    vi.mocked(api.getDefaultRetentionPolicy).mockResolvedValue(mockPolicy)
    vi.mocked(api.putDefaultRetentionPolicy).mockResolvedValue(mockPolicy)

    const wrapper = mount(SettingsView)
    await flushPromises()

    await wrapper.find('[data-testid="recent-count-input"]').setValue('5')
    await wrapper.find('[data-testid="retention-form"]').trigger('submit')
    await flushPromises()

    expect(api.putDefaultRetentionPolicy).toHaveBeenCalledWith({
      recentCount: 5,
      monthlyCount: 12,
    })
    expect(wrapper.find('[data-testid="success-alert"]').exists()).toBe(true)
  })

  it('should show a validation error and not submit when both counts are zero', async () => {
    const api = await import('@/api')
    vi.mocked(api.getDefaultRetentionPolicy).mockResolvedValue(mockPolicy)

    const wrapper = mount(SettingsView)
    await flushPromises()

    await wrapper.find('[data-testid="recent-count-input"]').setValue('0')
    await wrapper.find('[data-testid="monthly-count-input"]').setValue('0')
    await flushPromises()

    expect(wrapper.find('[data-testid="validation-error"]').exists()).toBe(true)

    await wrapper.find('[data-testid="retention-form"]').trigger('submit')
    await flushPromises()

    expect(api.putDefaultRetentionPolicy).not.toHaveBeenCalled()
  })

  it('should show an error alert when saving fails', async () => {
    const api = await import('@/api')
    vi.mocked(api.getDefaultRetentionPolicy).mockResolvedValue(mockPolicy)
    vi.mocked(api.putDefaultRetentionPolicy).mockRejectedValue(
      new Error('conflict')
    )

    const wrapper = mount(SettingsView)
    await flushPromises()

    await wrapper.find('[data-testid="retention-form"]').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
  })
})
