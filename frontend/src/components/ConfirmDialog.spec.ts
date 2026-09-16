/**
 * Tests for ConfirmDialog component
 * Coverage:
 * - Renders nothing when closed
 * - Renders title/message when open
 * - Emits confirm/cancel on button click
 */

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ConfirmDialog from './ConfirmDialog.vue'

describe('ConfirmDialog', () => {
  it('should not render when open is false', () => {
    const wrapper = mount(ConfirmDialog, {
      props: {
        open: false,
        title: 'Deletar servidor',
        message: 'Esta ação não pode ser desfeita.',
      },
    })

    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)
  })

  it('should render title and message when open is true', () => {
    const wrapper = mount(ConfirmDialog, {
      props: {
        open: true,
        title: 'Deletar servidor',
        message: 'Esta ação não pode ser desfeita.',
      },
    })

    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="confirm-dialog-title"]').text()).toBe(
      'Deletar servidor'
    )
    expect(wrapper.find('[data-testid="confirm-dialog-message"]').text()).toBe(
      'Esta ação não pode ser desfeita.'
    )
  })

  it('should emit confirm when the confirm button is clicked', async () => {
    const wrapper = mount(ConfirmDialog, {
      props: { open: true, title: 'Title', message: 'Message' },
    })

    await wrapper
      .find('[data-testid="confirm-dialog-confirm"]')
      .trigger('click')

    expect(wrapper.emitted('confirm')).toBeTruthy()
    expect(wrapper.emitted('cancel')).toBeFalsy()
  })

  it('should emit cancel when the cancel button is clicked', async () => {
    const wrapper = mount(ConfirmDialog, {
      props: { open: true, title: 'Title', message: 'Message' },
    })

    await wrapper.find('[data-testid="confirm-dialog-cancel"]').trigger('click')

    expect(wrapper.emitted('cancel')).toBeTruthy()
    expect(wrapper.emitted('confirm')).toBeFalsy()
  })

  it('should disable both buttons when loading is true', () => {
    const wrapper = mount(ConfirmDialog, {
      props: { open: true, title: 'Title', message: 'Message', loading: true },
    })

    const confirmBtn = wrapper.find<HTMLButtonElement>(
      '[data-testid="confirm-dialog-confirm"]'
    )
    const cancelBtn = wrapper.find<HTMLButtonElement>(
      '[data-testid="confirm-dialog-cancel"]'
    )

    expect(confirmBtn.element.disabled).toBe(true)
    expect(cancelBtn.element.disabled).toBe(true)
    expect(confirmBtn.text()).toBe('Confirmando...')
  })

  it('should not disable buttons when loading is false or omitted', () => {
    const wrapper = mount(ConfirmDialog, {
      props: { open: true, title: 'Title', message: 'Message' },
    })

    const confirmBtn = wrapper.find<HTMLButtonElement>(
      '[data-testid="confirm-dialog-confirm"]'
    )
    const cancelBtn = wrapper.find<HTMLButtonElement>(
      '[data-testid="confirm-dialog-cancel"]'
    )

    expect(confirmBtn.element.disabled).toBe(false)
    expect(cancelBtn.element.disabled).toBe(false)
    expect(confirmBtn.text()).toBe('Confirmar')
  })
})
