import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import Pagination from './Pagination.vue'

describe('Pagination.vue', () => {
  it('should render pagination controls with current page info', () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 1,
        pageSize: 10,
        totalItems: 50,
      },
    })

    expect(wrapper.text()).toContain('Página 1 de 5')
    expect(wrapper.find('[data-testid="pagination-prev"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pagination-next"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pagination-jump-input"]').exists()).toBe(true)
  })

  it('should emit update:page when previous button is clicked', async () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 2,
        pageSize: 10,
        totalItems: 50,
      },
    })

    const prevBtn = wrapper.find('[data-testid="pagination-prev"]')
    await prevBtn.trigger('click')

    expect(wrapper.emitted('update:page')).toBeTruthy()
    expect(wrapper.emitted('update:page')?.[0]).toEqual([1])
  })

  it('should emit update:page when next button is clicked', async () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 1,
        pageSize: 10,
        totalItems: 50,
      },
    })

    const nextBtn = wrapper.find('[data-testid="pagination-next"]')
    await nextBtn.trigger('click')

    expect(wrapper.emitted('update:page')).toBeTruthy()
    expect(wrapper.emitted('update:page')?.[0]).toEqual([2])
  })

  it('should disable previous button on first page', () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 1,
        pageSize: 10,
        totalItems: 50,
      },
    })

    const prevBtn = wrapper.find('[data-testid="pagination-prev"]') as any
    expect(prevBtn.attributes('disabled')).toBeDefined()
  })

  it('should disable next button on last page', () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 5,
        pageSize: 10,
        totalItems: 50,
      },
    })

    const nextBtn = wrapper.find('[data-testid="pagination-next"]') as any
    expect(nextBtn.attributes('disabled')).toBeDefined()
  })

  it('should emit update:page when jump to page button is clicked', async () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 1,
        pageSize: 10,
        totalItems: 50,
      },
    })

    const jumpBtn = wrapper.find('[data-testid="pagination-jump-btn"]')

    // Set via v-model
    wrapper.vm.jumpPageInput = 3
    await wrapper.vm.$nextTick()
    await jumpBtn.trigger('click')

    expect(wrapper.emitted('update:page')).toBeTruthy()
    expect(wrapper.emitted('update:page')?.[0]).toEqual([3])
  })

  it('should clamp jump-to-page value within valid range', async () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 1,
        pageSize: 10,
        totalItems: 50,
      },
    })

    const jumpBtn = wrapper.find('[data-testid="pagination-jump-btn"]')

    // Try to jump to page 10 when max is 5
    wrapper.vm.jumpPageInput = 10
    await wrapper.vm.$nextTick()
    await jumpBtn.trigger('click')

    // Should clamp to 5
    expect(wrapper.emitted('update:page')?.[0]).toEqual([5])
  })

  it('should disable jump input and button when totalItems is 0', () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 1,
        pageSize: 10,
        totalItems: 0,
      },
    })

    const jumpInput = wrapper.find('[data-testid="pagination-jump-input"]') as any
    const jumpBtn = wrapper.find('[data-testid="pagination-jump-btn"]') as any

    // When totalItems is 0, both input and button should be disabled
    expect(jumpInput.attributes('disabled')).toBeDefined()
    expect(jumpBtn.attributes('disabled')).toBeDefined()
  })

  it('should clear jump input after successful jump', async () => {
    const wrapper = mount(Pagination, {
      props: {
        currentPage: 1,
        pageSize: 10,
        totalItems: 50,
      },
    })

    const jumpBtn = wrapper.find('[data-testid="pagination-jump-btn"]')

    wrapper.vm.jumpPageInput = 3
    await wrapper.vm.$nextTick()
    await jumpBtn.trigger('click')
    await wrapper.vm.$nextTick()

    expect(wrapper.vm.jumpPageInput).toBe(null)
  })
})
