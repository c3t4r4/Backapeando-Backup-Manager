<template>
  <div class="p-6 max-w-2xl mx-auto">
    <h1 class="text-3xl font-bold mb-6 text-gray-800">Novo Servidor</h1>

    <!-- Error Alert -->
    <div
      v-if="error"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro ao criar servidor</p>
      <p>{{ error }}</p>
    </div>

    <form
      @submit.prevent="handleSubmit"
      class="bg-white p-6 rounded-lg shadow"
      data-testid="server-form"
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

      <div class="mb-4 grid grid-cols-3 gap-4">
        <div class="col-span-2">
          <label for="host" class="block text-gray-700 font-bold mb-2 text-sm">
            Host
          </label>
          <input
            id="host"
            v-model="host"
            type="text"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="host-input"
          />
        </div>
        <div>
          <label for="port" class="block text-gray-700 font-bold mb-2 text-sm">
            Porta
          </label>
          <input
            id="port"
            v-model.number="port"
            type="number"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="port-input"
          />
        </div>
      </div>

      <div class="mb-4">
        <label for="sshUser" class="block text-gray-700 font-bold mb-2 text-sm">
          Usuário SSH
        </label>
        <input
          id="sshUser"
          v-model="sshUser"
          type="text"
          required
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="ssh-user-input"
        />
      </div>

      <div class="mb-4 grid grid-cols-2 gap-4">
        <div>
          <label class="block text-gray-700 font-bold mb-2 text-sm">
            Motor do Banco de Dados
          </label>
          <Select v-model="dbEngine" :disabled="loading">
            <SelectTrigger data-testid="db-engine-select">
              <SelectValue placeholder="Motor do banco" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="postgres">PostgreSQL</SelectItem>
              <SelectItem value="mysql">MySQL</SelectItem>
              <SelectItem value="sqlserver">SQL Server</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <label class="block text-gray-700 font-bold mb-2 text-sm">
            Onde o Banco Roda
          </label>
          <Select v-model="deploymentMode" :disabled="loading">
            <SelectTrigger data-testid="deployment-mode-select">
              <SelectValue placeholder="Onde o banco roda" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="docker">Container Docker</SelectItem>
              <SelectItem value="host">Direto no Host/Instância</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div v-if="deploymentMode === 'docker'" class="mb-4">
        <label
          for="containerName"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Container Docker
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
        <p class="text-gray-500 text-xs mt-1">
          Pode ser parte do nome — o container é localizado por busca no momento
          da execução. Se mais de um corresponder, o backup falha pedindo um
          nome mais específico.
        </p>
      </div>

      <div class="mb-4 grid grid-cols-2 gap-4">
        <div>
          <label
            for="dbName"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Banco de Dados
          </label>
          <input
            id="dbName"
            v-model="dbName"
            type="text"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="db-name-input"
          />
        </div>
        <div>
          <label
            for="dbUser"
            class="block text-gray-700 font-bold mb-2 text-sm"
          >
            Usuário do Banco
          </label>
          <input
            id="dbUser"
            v-model="dbUser"
            type="text"
            required
            :disabled="loading"
            class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            data-testid="db-user-input"
          />
        </div>
      </div>

      <div class="mb-4">
        <label
          for="dbPassword"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Senha do Banco
          <span v-if="dbEngine === 'postgres'">(opcional)</span>
        </label>
        <input
          id="dbPassword"
          v-model="dbPassword"
          type="password"
          :required="dbEngine !== 'postgres'"
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="db-password-input"
        />
      </div>

      <div v-if="dbEngine === 'postgres'" class="mb-4">
        <label
          for="pgDumpExtraArgs"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Argumentos extras do pg_dump (opcional)
        </label>
        <input
          id="pgDumpExtraArgs"
          v-model="pgDumpExtraArgs"
          type="text"
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="pg-dump-extra-args-input"
        />
      </div>
      <div v-else-if="dbEngine === 'mysql'" class="mb-4">
        <label
          for="mysqlDumpExtraArgs"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Argumentos extras do mysqldump (opcional)
        </label>
        <input
          id="mysqlDumpExtraArgs"
          v-model="mysqlDumpExtraArgs"
          type="text"
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="mysql-dump-extra-args-input"
        />
      </div>
      <div v-else-if="dbEngine === 'sqlserver'" class="mb-4">
        <label
          for="sqlCmdExtraArgs"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Argumentos extras do sqlcmd (opcional)
        </label>
        <input
          id="sqlCmdExtraArgs"
          v-model="sqlCmdExtraArgs"
          type="text"
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="sqlcmd-extra-args-input"
        />
      </div>

      <Card class="mb-4" data-testid="dump-command-preview-card">
        <CardHeader>
          <CardTitle class="text-sm">Prévia do comando remoto</CardTitle>
        </CardHeader>
        <CardContent>
          <pre
            v-for="(line, i) in dumpPreview.lines"
            :key="i"
            class="bg-gray-800 text-gray-100 text-xs p-2 rounded overflow-x-auto whitespace-pre-wrap break-all mb-1 last:mb-0"
            data-testid="dump-command-preview-line"
            >{{ line }}</pre>
        </CardContent>
      </Card>

      <div class="mb-4">
        <label
          for="storageTargetId"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Destino de Backup (opcional)
        </label>
        <select
          id="storageTargetId"
          v-model="storageTargetId"
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="storage-target-select"
        >
          <option value="">Nenhum</option>
          <option
            v-for="target in storageTargets"
            :key="target.id"
            :value="target.id"
          >
            {{ target.name }} ({{ typeLabel(target.type) }})
          </option>
        </select>
      </div>

      <div class="mb-6">
        <label
          for="cronExpression"
          class="block text-gray-700 font-bold mb-2 text-sm"
        >
          Expressão Cron
        </label>
        <input
          id="cronExpression"
          v-model="cronExpression"
          type="text"
          required
          :disabled="loading"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="cron-expression-input"
        />
      </div>

      <div class="flex gap-4">
        <button
          type="submit"
          :disabled="loading"
          class="px-6 py-2 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="submit-button"
        >
          <span v-if="!loading">Criar Servidor</span>
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
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '@/api'
import type {
  StorageTargetDTO,
  StorageTargetType,
  UpsertServerRequest,
  DBEngine,
  DeploymentMode,
} from '@/api'
import { buildDumpCommandPreview } from '@/lib/dumpCommand'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const router = useRouter()

const name = ref('')
const host = ref('')
const port = ref(22)
const sshUser = ref('')
const dbEngine = ref<DBEngine>('postgres')
const deploymentMode = ref<DeploymentMode>('docker')
const containerName = ref('')
const dbName = ref('')
const dbUser = ref('')
const dbPassword = ref('')
const pgDumpExtraArgs = ref('')
const mysqlDumpExtraArgs = ref('')
const sqlCmdExtraArgs = ref('')
const storageTargetId = ref('')
const cronExpression = ref('0 3 * * *')

const storageTargets = ref<StorageTargetDTO[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const dumpPreview = computed(() =>
  buildDumpCommandPreview({
    dbEngine: dbEngine.value,
    deploymentMode: deploymentMode.value,
    containerName: containerName.value,
    dbUser: dbUser.value,
    dbName: dbName.value,
    hasPassword: dbPassword.value.length > 0,
    pgDumpExtraArgs: pgDumpExtraArgs.value,
    mysqlDumpExtraArgs: mysqlDumpExtraArgs.value,
    sqlCmdExtraArgs: sqlCmdExtraArgs.value,
  })
)

function typeLabel(type: StorageTargetType): string {
  switch (type) {
    case 'azure':
      return 'Azure'
    case 's3':
      return 'S3'
    case 'filesystem':
      return 'Local/NFS'
    default:
      return type
  }
}

/**
 * Load storage targets for the optional dropdown.
 * Failure here doesn't block server creation — storageTargetId is optional.
 */
onMounted(async () => {
  try {
    storageTargets.value = await api.getStorageTargets()
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] Failed to load storage targets:', err)
    }
  }
})

async function handleSubmit(): Promise<void> {
  loading.value = true
  error.value = null

  const req: UpsertServerRequest = {
    name: name.value,
    host: host.value,
    port: port.value,
    sshUser: sshUser.value,
    dbEngine: dbEngine.value,
    deploymentMode: deploymentMode.value,
    containerName:
      deploymentMode.value === 'docker' ? containerName.value : undefined,
    dbName: dbName.value,
    dbUser: dbUser.value,
    dbPassword: dbPassword.value || undefined,
    ...(dbEngine.value === 'postgres'
      ? { pgDumpExtraArgs: pgDumpExtraArgs.value || undefined }
      : {}),
    ...(dbEngine.value === 'mysql'
      ? { mysqlDumpExtraArgs: mysqlDumpExtraArgs.value || undefined }
      : {}),
    ...(dbEngine.value === 'sqlserver'
      ? { sqlCmdExtraArgs: sqlCmdExtraArgs.value || undefined }
      : {}),
    storageTargetId: storageTargetId.value || undefined,
    cronExpression: cronExpression.value,
  }

  try {
    await api.postServer(req)
    router.push('/servers')
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value =
      'Erro ao criar servidor. Verifique os dados e tente novamente.'
  } finally {
    loading.value = false
  }
}

function handleCancel(): void {
  router.push('/servers')
}
</script>
