<template>
  <div class="overflow-x-auto">
    <table
      class="w-full border-collapse border border-gray-300"
      data-testid="backups-table"
    >
      <thead>
        <tr class="bg-gray-100">
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Servidor
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Iniciado
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Concluído
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Status
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Tamanho
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Ações
          </th>
        </tr>
      </thead>
      <tbody>
        <!-- Empty state -->
        <tr v-if="backups.length === 0">
          <td
            colspan="6"
            class="border border-gray-300 px-4 py-4 text-center text-gray-500"
            data-testid="empty-backups"
          >
            Nenhum backup encontrado
          </td>
        </tr>

        <!-- Backup rows -->
        <tr
          v-for="backup in backups"
          :key="backup.id"
          class="hover:bg-gray-50 border-b border-gray-300"
          :data-testid="`backup-row-${backup.id}`"
        >
          <td class="border border-gray-300 px-4 py-3">
            <span
              class="text-gray-600 text-sm"
              :data-testid="`server-${backup.id}`"
            >
              {{ serverNames?.[backup.serverId] ?? formatServerId(backup.serverId) }}
            </span>
          </td>
          <td class="border border-gray-300 px-4 py-3 text-sm text-gray-600">
            <span :data-testid="`started-${backup.id}`">
              {{ formatDate(backup.startedAt) }}
            </span>
          </td>
          <td class="border border-gray-300 px-4 py-3 text-sm text-gray-600">
            <span :data-testid="`finished-${backup.id}`">
              {{ formatDate(backup.finishedAt) }}
            </span>
          </td>
          <td class="border border-gray-300 px-4 py-3">
            <span
              :class="statusBadgeClass(backup.status)"
              class="px-3 py-1 rounded-full text-sm font-semibold inline-block"
              :data-testid="`status-badge-${backup.id}`"
            >
              {{ statusLabel(backup.status) }}
            </span>
          </td>
          <td class="border border-gray-300 px-4 py-3 text-sm text-gray-600">
            <span :data-testid="`size-${backup.id}`">
              {{ formatSize(backup.blobSizeBytes) }}
            </span>
          </td>
          <td class="border border-gray-300 px-4 py-3">
            <button
              @click="$emit('view-details', backup.id)"
              class="px-3 py-1 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700 transition-colors"
              :data-testid="`details-btn-${backup.id}`"
            >
              Detalhes
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import type { BackupRunDTO } from '@/api'
import {
  formatDate,
  formatSize,
  formatBackupStatus as statusLabel,
  formatServerId,
} from '@/lib/format'

interface Props {
  backups: BackupRunDTO[]
  loading?: boolean
  // Optional server id -> name map, used to show a readable server name in
  // the "Servidor" column (e.g. in the History screen's all-servers default
  // view, RN-BACKUP-026). Falls back to formatServerId when absent/missing.
  serverNames?: Record<string, string>
}

defineProps<Props>()

defineEmits<{
  'view-details': [backupId: string]
}>()

/**
 * Get CSS classes for status badge styling
 * Different colors for each status state
 */
function statusBadgeClass(status: string): string {
  const classes: Record<string, string> = {
    queued: 'bg-yellow-100 text-yellow-800',
    running: 'bg-blue-100 text-blue-800',
    success: 'bg-green-100 text-green-800',
    failed: 'bg-red-100 text-red-800',
  }

  if (!classes[status]) {
    console.warn(`[SECURITY] Unknown backup status from API: ${status}`)
  }

  return classes[status] || 'bg-gray-100 text-gray-800'
}
</script>
