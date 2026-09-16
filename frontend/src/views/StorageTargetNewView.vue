<template>
  <div class="p-6 max-w-2xl mx-auto">
    <h1 class="text-3xl font-bold mb-6 text-gray-800">Novo Destino de Backup</h1>

    <!-- Error Alert -->
    <div
      v-if="error"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro ao criar destino</p>
      <p>{{ error }}</p>
    </div>

    <form
      @submit.prevent="handleSubmit"
      class="bg-white p-6 rounded-lg shadow"
      data-testid="storage-target-form"
    >
      <div class="mb-4">
        <label for="name" class="block text-gray-700 font-bold mb-2 text-sm">
          Nome
        </label>
        <input
          id="name"
          v-model="name"
          type="text"
          required
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="name-input"
        />
      </div>

      <div class="mb-6">
        <label for="type" class="block text-gray-700 font-bold mb-2 text-sm">
          Tipo
        </label>
        <select
          id="type"
          v-model="type"
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="type-select"
        >
          <option value="azure">Azure Blob Storage</option>
          <option value="s3">S3-compatível (AWS, MinIO, Backblaze, Wasabi, R2...)</option>
          <option value="filesystem">Pasta Local / NFS</option>
        </select>
      </div>

      <!-- Azure fields -->
      <template v-if="type === 'azure'">
        <div class="mb-4">
          <label
            for="accountName"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Conta de Armazenamento
          </label>
          <input
            id="accountName"
            v-model="accountName"
            type="text"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="account-name-input"
          />
        </div>

        <div class="mb-4">
          <label
            for="containerName"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Container
          </label>
          <input
            id="containerName"
            v-model="containerName"
            type="text"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="container-name-input"
          />
        </div>

        <div class="mb-6">
          <label
            for="sasToken"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            SAS Token
          </label>
          <input
            id="sasToken"
            v-model="sasToken"
            type="password"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="sas-token-input"
          />
        </div>
      </template>

      <!-- S3 fields -->
      <template v-else-if="type === 's3'">
        <div class="mb-4">
          <label for="bucket" class="block text-gray-700 font-bold mb-2 text-sm">
            Bucket
          </label>
          <input
            id="bucket"
            v-model="bucket"
            type="text"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="bucket-input"
          />
        </div>

        <div class="mb-4">
          <label for="region" class="block text-gray-700 font-bold mb-2 text-sm">
            Região (opcional)
          </label>
          <input
            id="region"
            v-model="region"
            type="text"
            placeholder="us-east-1"
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="region-input"
          />
        </div>

        <div class="mb-4">
          <label
            for="endpoint"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Endpoint (opcional — deixe em branco para AWS S3)
          </label>
          <input
            id="endpoint"
            v-model="endpoint"
            type="text"
            placeholder="https://s3.minio.example.com"
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="endpoint-input"
          />
        </div>

        <div class="mb-4">
          <label
            for="accessKeyId"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Access Key ID
          </label>
          <input
            id="accessKeyId"
            v-model="accessKeyId"
            type="text"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="access-key-id-input"
          />
        </div>

        <div class="mb-4">
          <label
            for="secretAccessKey"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Secret Access Key
          </label>
          <input
            id="secretAccessKey"
            v-model="secretAccessKey"
            type="password"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="secret-access-key-input"
          />
        </div>

        <div class="mb-6 flex items-center gap-2">
          <input
            id="usePathStyle"
            v-model="usePathStyle"
            type="checkbox"
            :disabled="loading"
            data-testid="use-path-style-checkbox"
          />
          <label for="usePathStyle" class="text-gray-700 text-sm">
            Usar path-style addressing (necessário para MinIO e a maioria dos
            servidores S3 self-hosted)
          </label>
        </div>
      </template>

      <!-- Filesystem fields -->
      <template v-else-if="type === 'filesystem'">
        <div class="mb-6">
          <label
            for="rootPath"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Caminho do Diretório
          </label>
          <input
            id="rootPath"
            v-model="rootPath"
            type="text"
            placeholder="/mnt/backups ou /srv/nfs/backups"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="root-path-input"
          />
          <p class="text-gray-500 text-xs mt-1">
            Pasta local no servidor, ou um ponto de montagem NFS já montado
            como diretório — o backend não implementa um cliente NFS de rede.
          </p>
        </div>
      </template>

      <div class="flex gap-4">
        <button
          type="submit"
          :disabled="loading"
          class="px-6 py-2 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="submit-button"
        >
          <span v-if="!loading">Criar Destino</span>
          <span v-else>Criando...</span>
        </button>
        <button
          type="button"
          @click="handleCancel"
          :disabled="loading"
          class="px-6 py-2 bg-gray-200 text-gray-800 font-medium rounded-lg hover:bg-gray-300 transition-colors"
          data-testid="cancel-button"
        >
          Cancelar
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '@/api'
import type { StorageTargetType, UpsertStorageTargetRequest } from '@/api'

const router = useRouter()

const name = ref('')
const type = ref<StorageTargetType>('azure')

// Azure fields
const accountName = ref('')
const containerName = ref('')
const sasToken = ref('')

// S3 fields
const bucket = ref('')
const region = ref('')
const endpoint = ref('')
const accessKeyId = ref('')
const secretAccessKey = ref('')
const usePathStyle = ref(false)

// Filesystem fields
const rootPath = ref('')

const loading = ref(false)
const error = ref<string | null>(null)

async function handleSubmit(): Promise<void> {
  loading.value = true
  error.value = null

  const req: UpsertStorageTargetRequest = {
    name: name.value,
    type: type.value,
    ...(type.value === 'azure'
      ? {
          accountName: accountName.value,
          containerName: containerName.value,
          sasToken: sasToken.value,
        }
      : {}),
    ...(type.value === 's3'
      ? {
          bucket: bucket.value,
          region: region.value || undefined,
          endpoint: endpoint.value || undefined,
          accessKeyId: accessKeyId.value,
          secretAccessKey: secretAccessKey.value,
          usePathStyle: usePathStyle.value,
        }
      : {}),
    ...(type.value === 'filesystem' ? { rootPath: rootPath.value } : {}),
  }

  try {
    await api.postStorageTarget(req)
    router.push('/storage-targets')
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao criar destino. Verifique os dados e tente novamente.'
  } finally {
    loading.value = false
  }
}

function handleCancel(): void {
  router.push('/storage-targets')
}
</script>
