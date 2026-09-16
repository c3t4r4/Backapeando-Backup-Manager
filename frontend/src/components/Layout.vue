<template>
  <div class="flex h-screen bg-gray-100">
    <Sidebar />
    <div class="flex-1 flex flex-col">
      <Navbar :user="user" @logout="handleLogout" />
      <main class="flex-1 overflow-auto p-6">
        <LoadingSpinner v-if="loading" size="lg" data-testid="layout-loading" />
        <slot v-else />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import Navbar from './Navbar.vue'
import Sidebar from './Sidebar.vue'
import LoadingSpinner from './LoadingSpinner.vue'
import type { AuthResponse } from '@/api'

defineProps<{
  user: AuthResponse | null
  loading?: boolean
}>()

const emit = defineEmits<{
  logout: []
}>()

function handleLogout() {
  emit('logout')
}
</script>
