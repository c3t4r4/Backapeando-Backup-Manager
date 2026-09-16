<template>
  <div class="min-h-screen bg-gray-100 flex items-center justify-center px-4">
    <div class="bg-white p-8 rounded-lg shadow-lg w-full max-w-md">
      <h1 class="text-3xl font-bold mb-6 text-center text-gray-800">
        Backapeando
      </h1>

      <!-- Error Alert -->
      <div
        v-if="error"
        class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
        data-testid="error-alert"
        role="alert"
      >
        <p class="font-bold">Erro de autenticação</p>
        <p>{{ error }}</p>
      </div>

      <!-- Login Form -->
      <form @submit.prevent="handleLogin" data-testid="login-form">
        <!-- Email Field -->
        <div class="mb-4">
          <label for="email" class="block text-gray-700 font-bold mb-2 text-sm">
            Email
          </label>
          <input
            id="email"
            v-model="email"
            type="email"
            required
            placeholder="usuario@example.com"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all"
            :disabled="loading"
            data-testid="email-input"
          />
        </div>

        <!-- Password Field -->
        <div class="mb-6">
          <label
            for="password"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Senha
          </label>
          <input
            id="password"
            v-model="password"
            type="password"
            required
            placeholder="••••••••"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all"
            :disabled="loading"
            data-testid="password-input"
          />
        </div>

        <!-- Submit Button -->
        <button
          type="submit"
          :disabled="loading"
          class="w-full bg-blue-600 text-white font-bold py-2 rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="submit-button"
        >
          <span v-if="!loading">Entrar</span>
          <span v-else>Entrando...</span>
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

const email = ref('')
const password = ref('')
const auth = useAuth()
const router = useRouter()

// Destructured at the top level of <script setup> so Vue's ref
// auto-unwrapping applies in the template. Accessing `auth.error`/
// `auth.loading` directly in the template reads the Ref object itself
// (always truthy) instead of its value — that bug made the error alert
// and the "Entrando..." state permanent on every mount.
const { error, loading } = auth

/**
 * Clear stale error state on mount — `error` is a module-level singleton
 * shared across the whole SPA, so a failed login from an earlier visit must
 * not bleed into a fresh navigation to /login.
 */
onMounted(() => {
  error.value = null
})

/**
 * Handle login form submission
 * Calls auth.login with email/password
 * On success: clears inputs/error and redirects to the dashboard
 * On failure: error is set automatically by useAuth
 */
async function handleLogin(): Promise<void> {
  const success = await auth.login(email.value, password.value)
  if (success) {
    // Clear sensitive fields after successful login
    email.value = ''
    password.value = ''
    router.push('/dashboard')
  }
  // Error is handled by auth.error ref — no explicit error handling needed
}
</script>
