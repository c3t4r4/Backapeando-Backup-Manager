<template>
  <div class="p-6 max-w-2xl mx-auto">
    <h1 class="text-3xl font-bold mb-6 text-gray-800">Configurações</h1>

    <h2 class="text-xl font-bold mb-4 text-gray-800">
      Política de Retenção Global
    </h2>
    <p class="text-gray-600 mb-4">
      Aplicada a todo servidor que não tiver uma política própria configurada.
    </p>

    <LoadingSpinner
      v-if="initialLoading"
      size="lg"
      message="Carregando política de retenção..."
      data-testid="loading-spinner"
    />

    <!-- Error Alert -->
    <div
      v-if="error && !initialLoading"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro</p>
      <p>{{ error }}</p>
    </div>

    <!-- Success Alert -->
    <div
      v-if="successMessage"
      class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4"
      data-testid="success-alert"
    >
      {{ successMessage }}
    </div>

    <form
      v-if="!initialLoading"
      @submit.prevent="handleSubmit"
      class="bg-white p-6 rounded-lg shadow"
      data-testid="retention-form"
    >
      <div class="mb-4 grid grid-cols-2 gap-4">
        <div>
          <label
            for="recentCount"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Backups recentes a manter
          </label>
          <input
            id="recentCount"
            v-model.number="recentCount"
            type="number"
            min="0"
            required
            :disabled="saving"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="recent-count-input"
          />
        </div>
        <div>
          <label
            for="monthlyCount"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Meses de backup mensal a manter
          </label>
          <input
            id="monthlyCount"
            v-model.number="monthlyCount"
            type="number"
            min="0"
            required
            :disabled="saving"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="monthly-count-input"
          />
        </div>
      </div>

      <div
        v-if="validationError"
        class="text-red-700 text-sm mb-4"
        data-testid="validation-error"
      >
        {{ validationError }}
      </div>

      <button
        type="submit"
        :disabled="saving"
        class="px-6 py-2 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
        data-testid="submit-button"
      >
        <span v-if="!saving">Salvar</span>
        <span v-else>Salvando...</span>
      </button>
    </form>

    <!-- Global Purge Section -->
    <div class="mt-8 pt-8 border-t">
      <h2 class="text-xl font-bold mb-4 text-gray-800">Expurgo Global</h2>
      <p class="text-gray-600 mb-4">
        Executar limpeza de retenção em todos os servidores elegíveis (prontos e habilitados com destino de backup configurado),
        aplicando a política de retenção de cada um. A execução é assíncrona.
      </p>

      <div
        v-if="sweepStatus && (sweepStatus.status === 'running' || sweepStatus.status === 'pending')"
        class="bg-blue-100 border border-blue-400 text-blue-700 px-4 py-3 rounded mb-4"
      >
        <p class="font-bold">Processando...</p>
        <p v-if="sweepStatus.status === 'pending'" class="text-sm">Aguardando início da execução...</p>
        <p v-else class="text-sm">Expurgo em andamento. Atualizando...</p>
      </div>

      <div
        v-if="sweepStatus && sweepStatus.status === 'completed'"
        class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4"
      >
        <p class="font-bold">Concluído com sucesso!</p>
        <p v-if="sweepStatus.summary" class="text-sm mt-2">
          Servidores processados: {{ sweepStatus.summary.serversProcessed }}<br />
          Blobs deletados: {{ sweepStatus.summary.totalBlobsDeleted }}<br />
          <span v-if="sweepStatus.summary.errors?.length > 0" class="text-red-600">
            Erros: {{ sweepStatus.summary.errors.length }} servidor(es) com falhas
          </span>
        </p>
      </div>

      <div
        v-if="sweepStatus && sweepStatus.status === 'failed'"
        class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      >
        <p class="font-bold">Erro na execução</p>
        <p v-if="sweepStatus.error" class="text-sm">{{ sweepStatus.error }}</p>
      </div>

      <button
        type="button"
        @click="triggerSweep"
        :disabled="sweepLoading || (sweepStatus && (sweepStatus.status === 'running' || sweepStatus.status === 'pending'))"
        class="px-6 py-2 bg-orange-600 text-white font-medium rounded-lg hover:bg-orange-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
        data-testid="sweep-button"
      >
        <span v-if="!sweepLoading">Rodar expurgo agora</span>
        <span v-else>Iniciando...</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import * as api from '@/api'
import LoadingSpinner from '@/components/LoadingSpinner.vue'

const recentCount = ref(3)
const monthlyCount = ref(12)

const initialLoading = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)
const successMessage = ref<string | null>(null)

const sweepLoading = ref(false)
const sweepStatus = ref<{
  status: string
  summary?: Record<string, unknown>
  error?: string
} | null>(null)

// Mirrors RN-BACKUP-004: recentCount and monthlyCount cannot both be zero.
const validationError = computed(() => {
  if (recentCount.value === 0 && monthlyCount.value === 0) {
    return 'Backups recentes e meses de retenção não podem ser ambos zero.'
  }
  return null
})

onMounted(async () => {
  try {
    const policy = await api.getDefaultRetentionPolicy()
    recentCount.value = policy.recentCount
    monthlyCount.value = policy.monthlyCount
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao carregar política de retenção. Tente novamente.'
  } finally {
    initialLoading.value = false
  }
})

async function handleSubmit(): Promise<void> {
  if (validationError.value) {
    return
  }

  saving.value = true
  error.value = null
  successMessage.value = null

  try {
    await api.putDefaultRetentionPolicy({
      recentCount: recentCount.value,
      monthlyCount: monthlyCount.value,
    })
    successMessage.value = 'Política de retenção salva com sucesso.'
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao salvar política de retenção. Tente novamente.'
  } finally {
    saving.value = false
  }
}

async function triggerSweep(): Promise<void> {
  sweepLoading.value = true
  try {
    await api.postRetentionSweep()
    sweepStatus.value = { status: 'pending' }
    // Start polling
    pollSweepStatus()
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    sweepStatus.value = {
      status: 'failed',
      error: 'Erro ao iniciar expurgo. Tente novamente.',
    }
  } finally {
    sweepLoading.value = false
  }
}

async function pollSweepStatus(): Promise<void> {
  // Poll every 2 seconds until complete or failed
  const maxAttempts = 300 // ~10 minutes max
  let attempts = 0

  while (attempts < maxAttempts) {
    attempts++
    await new Promise((resolve) => setTimeout(resolve, 2000)) // 2 second delay

    try {
      const latest = await api.getLatestRetentionSweep()
      sweepStatus.value = latest

      // Stop polling if not running/pending
      if (latest.status !== 'running' && latest.status !== 'pending') {
        break
      }
    } catch (err) {
      if (import.meta.env.DEV) {
        console.error('[DEBUG] Polling error:', err)
      }
    }
  }
}
</script>
