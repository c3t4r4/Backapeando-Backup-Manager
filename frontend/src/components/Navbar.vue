<template>
  <header
    class="bg-gray-900 text-white py-4 px-6 flex justify-between items-center"
  >
    <h1 class="text-2xl font-bold">Backapeando</h1>
    <div v-if="user" class="flex items-center gap-4">
      <span class="text-sm text-gray-300">{{ user.email }}</span>
      <button
        @click="handleLogout"
        class="px-4 py-2 bg-red-600 rounded hover:bg-red-700 transition-colors"
        data-testid="logout-button"
      >
        Logout
      </button>
    </div>
    <div v-else class="text-gray-400">
      <span class="text-sm">Not logged in</span>
    </div>
  </header>
</template>

<script setup lang="ts">
import type { AuthResponse } from '@/api'

defineProps<{
  user: AuthResponse | null
}>()

const emit = defineEmits<{
  logout: []
}>()

function handleLogout() {
  emit('logout')
}
</script>
