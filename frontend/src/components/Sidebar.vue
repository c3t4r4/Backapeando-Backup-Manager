<template>
  <nav
    class="bg-gray-800 text-white w-64 p-4 overflow-y-auto flex flex-col h-screen"
  >
    <ul class="space-y-2">
      <li v-for="item in items" :key="item.href">
        <router-link
          :to="item.href"
          class="block px-4 py-2 rounded text-gray-200 hover:bg-gray-700 transition-colors"
          :data-testid="`menu-item-${item.href}`"
        >
          {{ item.label }}
        </router-link>
      </li>
    </ul>
    <div class="mt-auto pt-4 border-t border-gray-700 text-sm text-gray-400">
      <p v-if="version" :data-testid="'version-display'">
        Versão: {{ version }}
      </p>
      <p v-else class="text-xs text-gray-500">Versão: indisponível</p>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getHealth } from '../api'

interface MenuItem {
  label: string
  href: string
}

withDefaults(
  defineProps<{
    items?: MenuItem[]
  }>(),
  {
    items: () => [
      { label: 'Dashboard', href: '/dashboard' },
      { label: 'Servers', href: '/servers' },
      { label: 'Destinos de Backup', href: '/storage-targets' },
      { label: 'Administradores', href: '/admin-users' },
      { label: 'History', href: '/history' },
      { label: 'Settings', href: '/settings' },
    ],
  }
)

const version = ref<string>('')

onMounted(async () => {
  try {
    const health = await getHealth()
    version.value = health.version
  } catch {
    // silently fail — version not available is not a blocker
  }
})
</script>
