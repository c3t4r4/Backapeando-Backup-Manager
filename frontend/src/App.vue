<template>
  <Layout
    v-if="route.meta.requiresAuth"
    :user="user"
    :loading="loading"
    @logout="handleLogout"
  >
    <router-view />
  </Layout>
  <router-view v-else />
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import Layout from '@/components/Layout.vue'
import { useAuth } from '@/composables/useAuth'

const route = useRoute()
const router = useRouter()
// `useAuth()` is a module-level singleton — `checkSession()` already ran
// once in main.ts before mount, so this just reads the same `user`/`loading`
// refs rather than triggering a new network call.
const { user, loading, logout } = useAuth()

async function handleLogout(): Promise<void> {
  await logout()
  router.push('/login')
}
</script>
