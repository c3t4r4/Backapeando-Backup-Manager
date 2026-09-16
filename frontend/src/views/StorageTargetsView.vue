<template>
  <div class="p-6">
    <!-- Header -->
    <h1 class="text-4xl font-bold mb-6 text-gray-800">Destinos de Backup</h1>

    <!-- Action Buttons -->
    <div class="mb-6 flex gap-4">
      <button
        @click="handleCreate"
        class="px-4 py-2 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 transition-colors"
        data-testid="create-target-btn"
      >
        + Novo Destino
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
      message="Carregando destinos..."
      data-testid="loading-spinner"
    />

    <!-- Error Alert -->
    <div
      v-if="error && !loading"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro ao carregar destinos</p>
      <p>{{ error }}</p>
    </div>

    <!-- Storage Target List -->
    <StorageTargetList
      v-if="!loading && targets.length > 0"
      :targets="targets"
      @edit="handleEdit"
      @delete="handleDelete"
      data-testid="storage-target-list"
    />

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :open="targetPendingDelete !== null"
      :loading="deleting"
      title="Deletar destino"
      message="Esta ação não pode ser desfeita. Servidores que usam este destino precisarão de um novo destino configurado."
      @confirm="confirmDelete"
      @cancel="cancelDelete"
    />

    <!-- Empty State -->
    <div
      v-if="!loading && targets.length === 0 && !error"
      class="text-center py-12"
      data-testid="empty-state"
    >
      <p class="text-gray-500 mb-6 text-lg">Nenhum destino cadastrado</p>
      <button
        @click="handleCreate"
        class="px-6 py-3 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 transition-colors"
        data-testid="create-first-target-btn"
      >
        Criar Primeiro Destino
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '@/api'
import type { StorageTargetDTO } from '@/api'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import StorageTargetList from '@/components/StorageTargetList.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const router = useRouter()
const targets = ref<StorageTargetDTO[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const targetPendingDelete = ref<string | null>(null)
const deleting = ref(false)

async function fetchTargets(): Promise<void> {
  loading.value = true
  error.value = null

  try {
    targets.value = await api.getStorageTargets()
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao carregar destinos. Tente novamente.'
    targets.value = []
  } finally {
    loading.value = false
  }
}

function handleCreate(): void {
  router.push('/storage-targets/new')
}

function handleRefresh(): void {
  fetchTargets()
}

function handleEdit(targetId: string): void {
  router.push(`/storage-targets/${targetId}/edit`)
}

function handleDelete(targetId: string): void {
  targetPendingDelete.value = targetId
}

async function confirmDelete(): Promise<void> {
  const targetId = targetPendingDelete.value
  if (!targetId || deleting.value) {
    return
  }

  deleting.value = true
  try {
    await api.deleteStorageTarget(targetId)
    targets.value = targets.value.filter((t) => t.id !== targetId)
    targetPendingDelete.value = null
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao deletar destino. Tente novamente.'
    targetPendingDelete.value = null
  } finally {
    deleting.value = false
  }
}

function cancelDelete(): void {
  if (deleting.value) {
    return
  }
  targetPendingDelete.value = null
}

onMounted(() => {
  fetchTargets()
})
</script>
