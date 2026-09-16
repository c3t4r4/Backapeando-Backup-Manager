/**
 * Tests for LoadingSpinner component
 * Coverage:
 * - Renders with correct size classes
 * - Renders message when provided
 * - Animate-spin class present
 * - Default size is md
 */

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LoadingSpinner from './LoadingSpinner.vue'

describe('LoadingSpinner', () => {
  it('should render spinner with md size by default', () => {
    const wrapper = mount(LoadingSpinner)

    const spinner = wrapper.find('[data-testid="spinner"]')
    expect(spinner.exists()).toBe(true)
    expect(spinner.classes()).toContain('w-12')
    expect(spinner.classes()).toContain('h-12')
  })

  it('should render spinner with sm size when size prop is set', () => {
    const wrapper = mount(LoadingSpinner, {
      props: {
        size: 'sm',
      },
    })

    const spinner = wrapper.find('[data-testid="spinner"]')
    expect(spinner.classes()).toContain('w-8')
    expect(spinner.classes()).toContain('h-8')
  })

  it('should render spinner with lg size when size prop is set', () => {
    const wrapper = mount(LoadingSpinner, {
      props: {
        size: 'lg',
      },
    })

    const spinner = wrapper.find('[data-testid="spinner"]')
    expect(spinner.classes()).toContain('w-16')
    expect(spinner.classes()).toContain('h-16')
  })

  it('should render message when message prop is provided', () => {
    const wrapper = mount(LoadingSpinner, {
      props: {
        message: 'Loading data...',
      },
    })

    const message = wrapper.find('[data-testid="spinner-message"]')
    expect(message.exists()).toBe(true)
    expect(message.text()).toBe('Loading data...')
  })

  it('should not render message when message prop is empty', () => {
    const wrapper = mount(LoadingSpinner, {
      props: {
        message: '',
      },
    })

    const message = wrapper.find('[data-testid="spinner-message"]')
    expect(message.exists()).toBe(false)
  })

  it('should have animate-spin class', () => {
    const wrapper = mount(LoadingSpinner)

    const spinner = wrapper.find('[data-testid="spinner"]')
    expect(spinner.classes()).toContain('animate-spin')
  })

  it('should have border styling classes', () => {
    const wrapper = mount(LoadingSpinner)

    const spinner = wrapper.find('[data-testid="spinner"]')
    expect(spinner.classes()).toContain('border-4')
    expect(spinner.classes()).toContain('border-gray-300')
    expect(spinner.classes()).toContain('border-t-blue-600')
    expect(spinner.classes()).toContain('rounded-full')
  })
})
