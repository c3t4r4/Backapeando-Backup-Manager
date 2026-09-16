/**
 * Tests for Sidebar component
 * Coverage:
 * - Renders list of menu items
 * - Each item has router-link with correct href
 * - Hover styling classes applied
 */

import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import Sidebar from './Sidebar.vue'

// Create a mock router for testing router-link
const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
    { path: '/servers', component: { template: '<div>Servers</div>' } },
    {
      path: '/storage-targets',
      component: { template: '<div>Destinos de Backup</div>' },
    },
    {
      path: '/admin-users',
      component: { template: '<div>Administradores</div>' },
    },
    { path: '/history', component: { template: '<div>History</div>' } },
    { path: '/settings', component: { template: '<div>Settings</div>' } },
  ],
})

describe('Sidebar', () => {
  it('should render list of default menu items', () => {
    const wrapper = mount(Sidebar, {
      global: {
        plugins: [router],
      },
    })

    const links = wrapper.findAll('a')
    expect(links).toHaveLength(6)
  })

  it('should render each item with router-link and correct href', () => {
    const wrapper = mount(Sidebar, {
      global: {
        plugins: [router],
      },
    })

    const links = wrapper.findAll('a')

    expect(links[0].attributes('href')).toBe('/dashboard')
    expect(links[0].text()).toBe('Dashboard')

    expect(links[1].attributes('href')).toBe('/servers')
    expect(links[1].text()).toBe('Servers')

    expect(links[2].attributes('href')).toBe('/storage-targets')
    expect(links[2].text()).toBe('Destinos de Backup')

    expect(links[3].attributes('href')).toBe('/admin-users')
    expect(links[3].text()).toBe('Administradores')

    expect(links[4].attributes('href')).toBe('/history')
    expect(links[4].text()).toBe('History')

    expect(links[5].attributes('href')).toBe('/settings')
    expect(links[5].text()).toBe('Settings')
  })

  it('should apply hover styling classes', () => {
    const wrapper = mount(Sidebar, {
      global: {
        plugins: [router],
      },
    })

    const link = wrapper.find('a')
    expect(link.classes()).toContain('hover:bg-gray-700')
    expect(link.classes()).toContain('rounded')
    expect(link.classes()).toContain('px-4')
  })

  it('should accept custom menu items', () => {
    const customItems = [
      { label: 'Custom 1', href: '/custom1' },
      { label: 'Custom 2', href: '/custom2' },
    ]

    const wrapper = mount(Sidebar, {
      props: {
        items: customItems,
      },
      global: {
        plugins: [router],
      },
    })

    const links = wrapper.findAll('a')
    expect(links).toHaveLength(2)
    expect(links[0].text()).toBe('Custom 1')
    expect(links[1].text()).toBe('Custom 2')
  })

  it('should have nav element with proper styling', () => {
    const wrapper = mount(Sidebar, {
      global: {
        plugins: [router],
      },
    })

    const nav = wrapper.find('nav')
    expect(nav.classes()).toContain('bg-gray-800')
    expect(nav.classes()).toContain('text-white')
    expect(nav.classes()).toContain('w-64')
  })
})
