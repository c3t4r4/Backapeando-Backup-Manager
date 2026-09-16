/**
 * Tests for Layout component
 * Coverage:
 * - Renders Navbar, Sidebar, and main slot
 * - LoadingSpinner visible when loading=true
 * - Slot content visible when loading=false
 * - Logout event propagation
 */

import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import Layout from './Layout.vue'
import type { AuthResponse } from '@/api'

// Create a mock router for testing router-link in Sidebar
const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
    { path: '/servers', component: { template: '<div>Servers</div>' } },
    {
      path: '/storage-targets',
      component: { template: '<div>Destinos de Backup</div>' },
    },
    { path: '/history', component: { template: '<div>History</div>' } },
    { path: '/settings', component: { template: '<div>Settings</div>' } },
  ],
})

describe('Layout', () => {
  it('should render Navbar, Sidebar, and slot content', () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Layout, {
      props: {
        user,
        loading: false,
      },
      slots: {
        default: '<div data-testid="slot-content">Main Content</div>',
      },
      global: {
        plugins: [router],
      },
    })

    // Check Navbar is rendered
    expect(wrapper.text()).toContain('Backapeando')
    expect(wrapper.text()).toContain('test@example.com')

    // Check Sidebar is rendered
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).toContain('History')

    // Check slot content is rendered
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(true)
  })

  it('should show LoadingSpinner when loading=true', () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Layout, {
      props: {
        user,
        loading: true,
      },
      slots: {
        default: '<div data-testid="slot-content">Main Content</div>',
      },
      global: {
        plugins: [router],
      },
    })

    const spinner = wrapper.find('[data-testid="layout-loading"]')
    expect(spinner.exists()).toBe(true)

    // Slot content should not be visible
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(false)
  })

  it('should show slot content when loading=false', () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Layout, {
      props: {
        user,
        loading: false,
      },
      slots: {
        default: '<div data-testid="slot-content">Main Content</div>',
      },
      global: {
        plugins: [router],
      },
    })

    const content = wrapper.find('[data-testid="slot-content"]')
    expect(content.exists()).toBe(true)
    expect(content.text()).toBe('Main Content')
  })

  it('should emit logout event from Navbar', async () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Layout, {
      props: {
        user,
        loading: false,
      },
      global: {
        plugins: [router],
      },
    })

    // Find and click logout button
    const logoutButton = wrapper.find('[data-testid="logout-button"]')
    expect(logoutButton.exists()).toBe(true)

    await logoutButton.trigger('click')

    expect(wrapper.emitted('logout')).toBeDefined()
    expect(wrapper.emitted('logout')).toHaveLength(1)
  })

  it('should have proper layout structure with flexbox', () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Layout, {
      props: {
        user,
        loading: false,
      },
      global: {
        plugins: [router],
      },
    })

    const outerDiv = wrapper.find('div')
    expect(outerDiv.classes()).toContain('flex')
    expect(outerDiv.classes()).toContain('h-screen')
    expect(outerDiv.classes()).toContain('bg-gray-100')
  })

  it('should toggle loading state dynamically', async () => {
    const user: AuthResponse = { email: 'test@example.com', role: 'admin' }

    const wrapper = mount(Layout, {
      props: {
        user,
        loading: false,
      },
      slots: {
        default: '<div data-testid="slot-content">Main Content</div>',
      },
      global: {
        plugins: [router],
      },
    })

    // Initially should show content
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(true)

    // Update to loading=true
    await wrapper.setProps({ loading: true })

    // Should show spinner
    expect(wrapper.find('[data-testid="layout-loading"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(false)

    // Update back to loading=false
    await wrapper.setProps({ loading: false })

    // Should show content again
    expect(wrapper.find('[data-testid="slot-content"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="layout-loading"]').exists()).toBe(false)
  })
})
