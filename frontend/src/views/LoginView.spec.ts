/**
 * Tests for LoginView component
 * Coverage:
 * - Renders login form with email and password inputs
 * - Form submission structure
 * - Input field properties and accessibility
 * - Layout and styling
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import LoginView from './LoginView.vue'
import * as api from '@/api'
import { useAuth } from '@/composables/useAuth'

// Mock the API module
vi.mock('@/api', async () => {
  const actual = await vi.importActual('@/api')
  return {
    ...actual,
    login: vi.fn(),
  }
})

/**
 * Test router with just the routes LoginView interacts with — a real
 * router instance is required so `useRouter()`/`router.push()` inside
 * LoginView work instead of warning "injection Symbol(router) not found".
 */
function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', component: LoginView },
      { path: '/dashboard', component: { template: '<div>Dashboard</div>' } },
    ],
  })
}

async function mountLoginView() {
  const router = createTestRouter()
  await router.push('/login')
  await router.isReady()
  const wrapper = mount(LoginView, { global: { plugins: [router] } })
  return { wrapper, router }
}

describe('LoginView', () => {
  // useAuth's state (user/isLoggedIn/loading/error) is a module-level
  // singleton shared across every test in this file — reset it directly
  // (no real API call) so a successful login in one test doesn't bleed
  // into the next.
  afterEach(() => {
    const auth = useAuth()
    auth.isLoggedIn.value = false
    auth.user.value = null
    auth.error.value = null
    auth.loading.value = false
  })

  it('should render login form with all required elements', async () => {
    const { wrapper } = await mountLoginView()

    // Check all form elements exist
    expect(wrapper.find('[data-testid="login-form"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="email-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="password-input"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="submit-button"]').exists()).toBe(true)

    // Check company branding is displayed
    expect(wrapper.text()).toContain('Backapeando')
  })

  it('should have input fields with empty values on mount', async () => {
    const { wrapper } = await mountLoginView()

    const emailInput = wrapper.find<HTMLInputElement>(
      '[data-testid="email-input"]'
    )
    const passwordInput = wrapper.find<HTMLInputElement>(
      '[data-testid="password-input"]'
    )

    expect(emailInput.element.value).toBe('')
    expect(passwordInput.element.value).toBe('')
  })

  it('should accept and update email and password input values', async () => {
    const { wrapper } = await mountLoginView()

    const emailInput = wrapper.find<HTMLInputElement>(
      '[data-testid="email-input"]'
    )
    const passwordInput = wrapper.find<HTMLInputElement>(
      '[data-testid="password-input"]'
    )

    // setValue() sets the value and fires the input event atomically per
    // field — setting both DOM values upfront and triggering events after
    // (the previous approach) raced against Vue re-rendering the first
    // field's v-model change, which reset the second field's untracked
    // DOM-only value back to its stale ref value before its own event fired.
    await emailInput.setValue('user@example.com')
    await passwordInput.setValue('password123')

    // Verify values are set
    expect(emailInput.element.value).toBe('user@example.com')
    expect(passwordInput.element.value).toBe('password123')
  })

  it('should have proper form input types and attributes for accessibility', async () => {
    const { wrapper } = await mountLoginView()

    const emailInput = wrapper.find<HTMLInputElement>(
      '[data-testid="email-input"]'
    )
    const passwordInput = wrapper.find<HTMLInputElement>(
      '[data-testid="password-input"]'
    )

    // Check input types
    expect(emailInput.element.type).toBe('email')
    expect(passwordInput.element.type).toBe('password')

    // Check required attributes
    expect(emailInput.element.required).toBe(true)
    expect(passwordInput.element.required).toBe(true)
  })

  it('should have semantic HTML structure with labels', async () => {
    const { wrapper } = await mountLoginView()

    const labels = wrapper.findAll('label')
    expect(labels.length).toBeGreaterThanOrEqual(2)

    const labelTexts = labels.map((l) => l.text())
    expect(labelTexts.some((t) => t.includes('Email'))).toBe(true)
    expect(labelTexts.some((t) => t.includes('Senha'))).toBe(true)

    // Check label for attributes
    const emailLabel = wrapper.find('label[for="email"]')
    const passwordLabel = wrapper.find('label[for="password"]')
    expect(emailLabel.exists()).toBe(true)
    expect(passwordLabel.exists()).toBe(true)
  })

  it('should have responsive layout styling with Tailwind classes', async () => {
    const { wrapper } = await mountLoginView()

    // Check for main container with background
    const mainContainer = wrapper.find('[class*="bg-gray-100"]')
    expect(mainContainer.exists()).toBe(true)

    // Check for card container
    const cardContainer = wrapper.find('[class*="bg-white"]')
    expect(cardContainer.exists()).toBe(true)

    // Check for focus styling on inputs
    const emailInput = wrapper.find('[data-testid="email-input"]')
    expect(emailInput.classes()).toContain('focus:ring-2')
    expect(emailInput.classes()).toContain('focus:ring-blue-500')
  })

  it('should render submit button with accessible structure', async () => {
    const { wrapper } = await mountLoginView()

    const button = wrapper.find<HTMLButtonElement>(
      '[data-testid="submit-button"]'
    )
    expect(button.exists()).toBe(true)
    expect(button.element.type).toBe('submit')

    // Button should be part of form
    const form = wrapper.find('[data-testid="login-form"]')
    expect(form.element.contains(button.element)).toBe(true)
  })

  it('should redirect to /dashboard after a successful login', async () => {
    vi.mocked(api.login).mockResolvedValueOnce({
      email: 'user@example.com',
      role: 'admin',
    })

    const { wrapper, router } = await mountLoginView()

    await wrapper
      .find<HTMLInputElement>('[data-testid="email-input"]')
      .setValue('user@example.com')
    await wrapper
      .find<HTMLInputElement>('[data-testid="password-input"]')
      .setValue('correct-password')
    await wrapper.find('[data-testid="login-form"]').trigger('submit')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('should not redirect when login fails', async () => {
    vi.mocked(api.login).mockRejectedValueOnce(
      new Error('invalid email or password')
    )

    const { wrapper, router } = await mountLoginView()

    await wrapper
      .find<HTMLInputElement>('[data-testid="email-input"]')
      .setValue('user@example.com')
    await wrapper
      .find<HTMLInputElement>('[data-testid="password-input"]')
      .setValue('wrong-password')
    await wrapper.find('[data-testid="login-form"]').trigger('submit')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/login')
    expect(wrapper.find('[data-testid="error-alert"]').exists()).toBe(true)
  })
})
