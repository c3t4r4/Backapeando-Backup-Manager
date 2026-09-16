<template>
  <nav class="bg-gray-800 text-white w-64 p-4 overflow-y-auto">
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
  </nav>
</template>

<script setup lang="ts">
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
</script>
