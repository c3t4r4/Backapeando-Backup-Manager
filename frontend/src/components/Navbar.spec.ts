/**
 * Tests for Navbar component
 * Coverage:
 * - Renders user email when user is set
 * - Renders "Not logged in" when user is null
 * - Logout button calls callback on click
 */

import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import Navbar from './Navbar.vue'
import type { AuthResponse } from '@/api'

describe('Navbar', () => {
  it('should render user email when user is set', () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Navbar, {
      props: {
        user,
      },
    })

    expect(wrapper.text()).toContain('test@example.com')
    expect(wrapper.text()).toContain('Backapeando')
  })

  it('should render "Not logged in" when user is null', () => {
    const wrapper = mount(Navbar, {
      props: {
        user: null,
      },
    })

    expect(wrapper.text()).toContain('Not logged in')
  })

  it('should emit logout event when logout button is clicked', async () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Navbar, {
      props: {
        user,
      },
    })

    const logoutButton = wrapper.find('[data-testid="logout-button"]')
    expect(logoutButton.exists()).toBe(true)

    await logoutButton.trigger('click')

    expect(wrapper.emitted('logout')).toBeDefined()
    expect(wrapper.emitted('logout')).toHaveLength(1)
  })

  it('should display company name in header', () => {
    const wrapper = mount(Navbar, {
      props: {
        user: null,
      },
    })

    const header = wrapper.find('header')
    expect(header.exists()).toBe(true)
    expect(header.text()).toContain('Backapeando')
  })

  it('should have proper styling classes', () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Navbar, {
      props: {
        user,
      },
    })

    const header = wrapper.find('header')
    expect(header.classes()).toContain('bg-gray-900')
    expect(header.classes()).toContain('text-white')
  })
})
