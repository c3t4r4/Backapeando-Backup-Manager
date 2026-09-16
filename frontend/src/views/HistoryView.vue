<template>
  <div class="p-6">
    <!-- Header -->
    <h1 class="text-4xl font-bold mb-6 text-gray-800">Histórico de Backups</h1>

    <!-- Server Selector + Status Filter + Action Buttons -->
    <div class="mb-6 flex gap-4 items-end">
      <div>
        <label
          for="serverSelect"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Servidor
        </label>
        <select
          id="serverSelect"
          v-model="selectedServerId"
          class="px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="server-select"
        >
          <option value="">Selecione um servidor</option>
          <option v-for="server in servers" :key="server.id" :value="server.id">
            {{ server.name }}
          </option>
        </select>
      </div>
      <div>
        <label
          for="statusSelect"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Status
        </label>
        <select
          id="statusSelect"
          v-model="selectedStatus"
          class="px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="status-select"
        >
          <option value="">Todos</option>
          <option value="queued">Enfileirado</option>
          <option value="running">Em execução</option>
          <option value="success">Sucesso</option>
          <option value="failed">Falhou</option>
        </select>
      </div>
      <button
        @click="handleRefresh"
        :disabled="loading"
        class="px-4 py-2 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
        data-testid="refresh-btn"
      >
        Atualizar
      </button>
    </div>

    <!-- Loading Spinner Overlay -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      message="Carregando histórico..."
      data-testid="loading-spinner"
    />

    <!-- Error Alert -->
    <div
      v-if="error && !loading"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro ao carregar histórico</p>
      <p>{{ error }}</p>
    </div>

    <!-- Backup Table -->
    <BackupTable
      v-if="!loading && backups.length > 0"
      :backups="backups"
      :server-names="serverNames"
      @view-details="handleViewDetails"
      data-testid="backup-table"
    />

    <!-- Backup Run Details -->
    <BackupRunDetailsDialog
      :backup="selectedBackup"
      @close="selectedBackup = null"
    />

    <!-- Empty State -->
    <div
      v-if="!loading && backups.length === 0 && !error"
      class="text-center py-12"
      data-testid="empty-state"
    >
      <p class="text-gray-500 mb-6 text-lg">Nenhum backup encontrado</p>
    </div>

    <!-- Pagination -->
    <Pagination
      v-if="!loading && totalBackups > 0"
      :current-page="page"
      :page-size="pageSize"
      :total-items="totalBackups"
      @update:page="handlePageChange"
      data-testid="pagination"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import * as api from '@/api'
import type { BackupRunDTO, ServerDTO } from '@/api'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import BackupTable from '@/components/BackupTable.vue'
import BackupRunDetailsDialog from '@/components/BackupRunDetailsDialog.vue'
import Pagination from '@/components/Pagination.vue'

const route = useRoute()

// State
const servers = ref<ServerDTO[]>([])
const selectedServerId = ref('')
const selectedStatus = ref<'' | BackupRunDTO['status']>('')
const backups = ref<BackupRunDTO[]>([])
const totalBackups = ref(0)
const loading = ref(false)
const error = ref<string | null>(null)
const page = ref(1)
const pageSize = ref(50)
const selectedBackup = ref<BackupRunDTO | null>(null)

// Maps server id -> name, so the "Servidor" column shows a readable name
// instead of a truncated UUID when the default (all servers) view is active
// (RN-BACKUP-026). Built from the same server list already loaded for the
// filter dropdown, no extra request needed.
const serverNames = computed<Record<string, string>>(() =>
  Object.fromEntries(servers.value.map((s) => [s.id, s.name]))
)

/**
 * Fetch the current page of backups, filtered by status when set
 * (RN-BACKUP-025) and by server when one is selected. With no server
 * selected, defaults to the last 50 backups across every server
 * (RN-BACKUP-026) via GET /api/backup-runs.
 */
async function fetchBackups(): Promise<void> {
  loading.value = true
  error.value = null

  try {
    const response = await api.getAllBackupRuns({
      page: page.value,
      pageSize: pageSize.value,
      status: selectedStatus.value || undefined,
      serverId: selectedServerId.value || undefined,
    })
    backups.value = response.items
    totalBackups.value = response.total
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao carregar histórico de backups. Tente novamente.'
    backups.value = []
    totalBackups.value = 0
  } finally {
    loading.value = false
  }
}

function handleRefresh(): void {
  page.value = 1
  fetchBackups()
}

function handlePageChange(newPage: number): void {
  page.value = newPage
  fetchBackups()
}

function handleViewDetails(backupId: string): void {
  selectedBackup.value = backups.value.find((b) => b.id === backupId) ?? null
}

watch(selectedServerId, () => {
  page.value = 1
  fetchBackups()
})

watch(selectedStatus, () => {
  page.value = 1
  fetchBackups()
})

onMounted(async () => {
  try {
    servers.value = await api.getServers()
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] Failed to load servers:', err)
    }
  }

  const queryServerId = route.query.server
  if (typeof queryServerId === 'string' && queryServerId) {
    // Triggers fetchBackups() via the watch(selectedServerId) below.
    selectedServerId.value = queryServerId
  } else {
    // No server pre-selected: fetch the default view (last 50 across every
    // server, RN-BACKUP-026) immediately instead of waiting for user input.
    fetchBackups()
  }
})
</script>
