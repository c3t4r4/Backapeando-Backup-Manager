<template>
  <div class="p-6">
    <!-- Header -->
    <h1 class="text-4xl font-bold mb-6 text-gray-800">Servidores</h1>

    <!-- Action Buttons -->
    <div class="mb-6 flex gap-4">
      <button
        @click="handleCreate"
        class="px-4 py-2 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 transition-colors"
        data-testid="create-server-btn"
      >
        + Novo Servidor
      </button>
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
      message="Carregando servidores..."
      data-testid="loading-spinner"
    />

    <!-- Error Alert -->
    <div
      v-if="error && !loading"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro ao carregar servidores</p>
      <p>{{ error }}</p>
    </div>

    <!-- Server List -->
    <ServerList
      v-if="!loading && servers.length > 0"
      :servers="servers"
      @edit="handleEdit"
      @delete="handleDelete"
      @view-backups="handleViewBackups"
      data-testid="server-list"
    />

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :open="serverPendingDelete !== null"
      :loading="deleting"
      title="Deletar servidor"
      message="Esta ação não pode ser desfeita. O servidor e seu histórico de configuração serão removidos."
      @confirm="confirmDelete"
      @cancel="cancelDelete"
    />

    <!-- Empty State -->
    <div
      v-if="!loading && servers.length === 0 && !error"
      class="text-center py-12"
      data-testid="empty-state"
    >
      <p class="text-gray-500 mb-6 text-lg">Nenhum servidor cadastrado</p>
      <button
        @click="handleCreate"
        class="px-6 py-3 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 transition-colors"
        data-testid="create-first-server-btn"
      >
        Criar Primeiro Servidor
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '@/api'
import type { ServerDTO } from '@/api'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import ServerList from '@/components/ServerList.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const router = useRouter()
const servers = ref<ServerDTO[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const serverPendingDelete = ref<string | null>(null)
const deleting = ref(false)

/**
 * Fetch servers from API
 * Sets loading state, handles errors, and updates servers list
 */
async function fetchServers(): Promise<void> {
  loading.value = true
  error.value = null

  try {
    servers.value = await api.getServers()
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao carregar servidores. Tente novamente.'
    servers.value = []
  } finally {
    loading.value = false
  }
}

/**
 * Handle "Create Server" button click
 * Navigates to the new-server form
 */
function handleCreate(): void {
  router.push('/servers/new')
}

/**
 * Handle "Refresh" button click
 * Re-fetches the servers list
 */
function handleRefresh(): void {
  fetchServers()
}

/**
 * Handle "Edit" button click on a server
 * Navigates to the edit form for the given server
 *
 * @param serverId The ID of the server to edit
 */
function handleEdit(serverId: string): void {
  router.push(`/servers/${serverId}/edit`)
}

/**
 * Handle "Delete" button click on a server
 * Opens the confirmation dialog; the actual delete happens in confirmDelete()
 *
 * @param serverId The ID of the server to delete
 */
function handleDelete(serverId: string): void {
  serverPendingDelete.value = serverId
}

/**
 * Confirm deletion of the server pending removal
 * Calls the API and removes the server from the local list on success
 * Guarded by `deleting` so a double-click or slow network can't fire
 * api.deleteServer() twice for the same server
 */
async function confirmDelete(): Promise<void> {
  const serverId = serverPendingDelete.value
  if (!serverId || deleting.value) {
    return
  }

  deleting.value = true
  try {
    await api.deleteServer(serverId)
    servers.value = servers.value.filter((s) => s.id !== serverId)
    serverPendingDelete.value = null
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao deletar servidor. Tente novamente.'
    serverPendingDelete.value = null
  } finally {
    deleting.value = false
  }
}

/**
 * Cancel the pending delete without calling the API
 */
function cancelDelete(): void {
  if (deleting.value) {
    return
  }
  serverPendingDelete.value = null
}

/**
 * Handle "View Backups" button click on a server
 * Navigates to backup history page filtered by server
 *
 * @param serverId The ID of the server
 */
function handleViewBackups(serverId: string): void {
  router.push(`/history?server=${serverId}`)
}

/**
 * Load servers on component mount
 */
onMounted(() => {
  fetchServers()
})
</script>
