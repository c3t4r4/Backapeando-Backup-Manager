<template>
  <div class="overflow-x-auto">
    <table
      class="w-full border-collapse border border-gray-300"
      data-testid="servers-table"
    >
      <thead>
        <tr class="bg-gray-100">
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Nome
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Host
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Porta
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Status
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Ações
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="server in servers"
          :key="server.id"
          class="hover:bg-gray-50 border-b border-gray-300"
          :data-testid="`server-row-${server.id}`"
        >
          <td class="border border-gray-300 px-4 py-3">
            <span class="font-medium text-gray-800">{{ server.name }}</span>
          </td>
          <td class="border border-gray-300 px-4 py-3 text-gray-600">
            {{ server.host }}
          </td>
          <td class="border border-gray-300 px-4 py-3 text-gray-600">
            {{ server.port }}
          </td>
          <td class="border border-gray-300 px-4 py-3">
            <span
              :class="statusBadgeClass(server.status)"
              class="px-3 py-1 rounded-full text-sm font-semibold inline-block"
              :data-testid="`status-badge-${server.id}`"
            >
              {{ statusLabel(server.status) }}
            </span>
          </td>
          <td class="border border-gray-300 px-4 py-3">
            <div class="flex gap-2 flex-wrap">
              <button
                @click="$emit('edit', server.id)"
                class="px-3 py-1 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700 transition-colors"
                :data-testid="`edit-btn-${server.id}`"
              >
                Editar
              </button>
              <button
                @click="$emit('delete', server.id)"
                class="px-3 py-1 bg-red-600 text-white rounded text-sm font-medium hover:bg-red-700 transition-colors"
                :data-testid="`delete-btn-${server.id}`"
              >
                Deletar
              </button>
              <button
                @click="$emit('view-backups', server.id)"
                class="px-3 py-1 bg-purple-600 text-white rounded text-sm font-medium hover:bg-purple-700 transition-colors"
                :data-testid="`backups-btn-${server.id}`"
              >
                Backups
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import type { ServerDTO } from '@/api'

interface Props {
  servers: ServerDTO[]
}

defineProps<Props>()

defineEmits<{
  edit: [serverId: string]
  delete: [serverId: string]
  'view-backups': [serverId: string]
}>()

/**
 * Get human-readable status label
 * Maps internal status values to Portuguese labels
 */
function statusLabel(status: string): string {
  return (
    {
      pending_key: 'Aguardando Chave',
      awaiting_authorization: 'Aguardando Autorização',
      ready: 'Pronto',
    }[status] || status
  )
}

/**
 * Get CSS classes for status badge styling
 * Different colors for different status states
 * Logs warning if unknown status is encountered
 */
function statusBadgeClass(status: string): string {
  const classes: Record<string, string> = {
    pending_key: 'bg-yellow-100 text-yellow-800',
    awaiting_authorization: 'bg-orange-100 text-orange-800',
    ready: 'bg-green-100 text-green-800',
  }

  if (!classes[status]) {
    console.warn(`[SECURITY] Unknown server status from API: ${status}`)
  }

  return classes[status] || 'bg-gray-100 text-gray-800'
}
</script>
