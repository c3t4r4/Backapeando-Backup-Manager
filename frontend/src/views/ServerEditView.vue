<template>
  <div class="p-6 max-w-2xl mx-auto">
    <h1 class="text-3xl font-bold mb-6 text-gray-800">Editar Servidor</h1>

    <LoadingSpinner
      v-if="initialLoading"
      size="lg"
      message="Carregando servidor..."
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

    <form
      v-if="!initialLoading && loaded"
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
          :disabled="saving"
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
            :disabled="saving"
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
            :disabled="saving"
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
          :disabled="saving"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="ssh-user-input"
        />
      </div>

      <div class="mb-4 grid grid-cols-2 gap-4">
        <div>
          <label class="block text-gray-700 font-bold mb-2 text-sm">
            Motor do Banco de Dados
          </label>
          <Select v-model="dbEngine" :disabled="saving">
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
          <Select v-model="deploymentMode" :disabled="saving">
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
          :disabled="saving"
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
            :disabled="saving"
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
            :disabled="saving"
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
          :placeholder="
            hasDbPassword ? 'Deixe em branco para manter a atual' : ''
          "
          :disabled="saving"
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
          :disabled="saving"
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
          :disabled="saving"
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
          :disabled="saving"
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
          :disabled="saving"
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
          :disabled="saving"
          class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          data-testid="cron-expression-input"
        />
      </div>

      <div class="flex gap-4">
        <button
          type="submit"
          :disabled="saving"
          class="px-6 py-2 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="submit-button"
        >
          <span v-if="!saving">Salvar Alterações</span>
          <span v-else>Salvando...</span>
        </button>
        <button
          type="button"
          @click="handleCancel"
          :disabled="saving"
          class="px-6 py-2 bg-gray-200 text-gray-800 font-medium rounded-lg hover:bg-gray-300 transition-colors"
          data-testid="cancel-button"
        >
          Cancelar
        </button>
      </div>
    </form>

    <!-- Retention Policy -->
    <div
      v-if="!initialLoading && loaded"
      class="bg-white p-6 rounded-lg shadow mt-6"
      data-testid="retention-section"
    >
      <h2 class="text-xl font-bold mb-4 text-gray-800">Política de Retenção</h2>

      <LoadingSpinner
        v-if="retentionInitialLoading"
        size="md"
        data-testid="retention-loading-spinner"
      />

      <template v-else>
        <label class="flex items-center gap-2 mb-4">
          <input
            type="checkbox"
            v-model="useGlobalRetention"
            :disabled="retentionSaving"
            data-testid="use-global-retention-checkbox"
          />
          <span class="text-gray-700">Usar política global</span>
        </label>

        <div v-if="!useGlobalRetention" class="grid grid-cols-2 gap-4 mb-4">
          <div>
            <label
              for="retentionRecentCount"
              class="block text-gray-700 font-bold mb-2 text-sm"
            >
              Backups recentes a manter
            </label>
            <input
              id="retentionRecentCount"
              v-model.number="retentionRecentCount"
              type="number"
              min="0"
              :disabled="retentionSaving"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              data-testid="retention-recent-count-input"
            />
          </div>
          <div>
            <label
              for="retentionMonthlyCount"
              class="block text-gray-700 font-bold mb-2 text-sm"
            >
              Meses de backup mensal a manter
            </label>
            <input
              id="retentionMonthlyCount"
              v-model.number="retentionMonthlyCount"
              type="number"
              min="0"
              :disabled="retentionSaving"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              data-testid="retention-monthly-count-input"
            />
          </div>
        </div>

        <div
          v-if="retentionValidationError"
          class="text-red-700 text-sm mb-4"
          data-testid="retention-validation-error"
        >
          {{ retentionValidationError }}
        </div>

        <div
          v-if="retentionError"
          class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
          data-testid="retention-error-alert"
        >
          {{ retentionError }}
        </div>
        <div
          v-if="retentionSuccessMessage"
          class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4"
          data-testid="retention-success-alert"
        >
          {{ retentionSuccessMessage }}
        </div>

        <button
          type="button"
          @click="handleSaveRetention"
          :disabled="retentionSaving || !!retentionValidationError"
          class="px-6 py-2 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="save-retention-button"
        >
          <span v-if="!retentionSaving">Salvar Retenção</span>
          <span v-else>Salvando...</span>
        </button>
      </template>
    </div>

    <!-- SSH lifecycle + backup actions -->
    <div
      v-if="!initialLoading && loaded"
      class="bg-white p-6 rounded-lg shadow mt-6"
      data-testid="actions-section"
    >
      <h2 class="text-xl font-bold mb-4 text-gray-800">Ações</h2>

      <p class="text-gray-600 mb-4" data-testid="server-status">
        Status: {{ statusLabel }} ·
        {{ serverEnabled ? 'Habilitado' : 'Desabilitado' }}
      </p>

      <!-- Guia de autorização SSH -->
      <div
        v-if="sshPublicKey"
        class="mb-6 p-4 rounded-lg border"
        :class="
          serverStatus === 'ready'
            ? 'bg-gray-50 border-gray-200'
            : 'bg-yellow-50 border-yellow-300'
        "
        data-testid="ssh-authorization-guide"
      >
        <p class="text-gray-700 mb-2">
          Para autorizar este servidor, adicione a chave pública abaixo ao
          arquivo
          <code class="bg-gray-200 px-1 rounded">~/.ssh/authorized_keys</code>
          do usuário
          <strong>{{ sshUser }}</strong> no host <strong>{{ host }}</strong
          >. Depois, clique em "Testar Backup" para verificar a conexão.
        </p>
        <div class="flex gap-2 items-start">
          <code
            class="flex-1 block bg-gray-800 text-gray-100 text-xs p-3 rounded overflow-x-auto whitespace-pre-wrap break-all"
            data-testid="ssh-public-key"
            >{{ sshPublicKey }}</code
          >
          <button
            type="button"
            @click="handleCopyPublicKey"
            class="px-3 py-2 bg-gray-700 text-white text-sm rounded hover:bg-gray-600 transition-colors whitespace-nowrap"
            data-testid="copy-ssh-key-button"
          >
            {{ copiedPublicKey ? 'Copiado!' : 'Copiar' }}
          </button>
        </div>
      </div>

      <!-- Testar Backup -->
      <div class="mb-6">
        <button
          type="button"
          @click="handleTestBackup"
          :disabled="testing"
          class="px-4 py-2 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="test-backup-button"
        >
          <span v-if="!testing">Testar Backup</span>
          <span v-else>Testando...</span>
        </button>

        <div
          v-if="testChecks"
          class="mt-3 space-y-1 text-sm"
          data-testid="test-checks-result"
        >
          <p data-testid="check-ssh">
            SSH: {{ testChecks.ssh.ok ? '✅' : '❌' }}
            <span v-if="!testChecks.ssh.ok" class="text-red-700">{{
              testChecks.ssh.error
            }}</span>
          </p>
          <p data-testid="check-dump-tool">
            Ferramenta de Dump: {{ testChecks.dumpTool.ok ? '✅' : '❌' }}
            <span v-if="!testChecks.dumpTool.ok" class="text-red-700">{{
              testChecks.dumpTool.error
            }}</span>
          </p>
          <p v-if="testChecks.storage" data-testid="check-storage">
            Storage: {{ testChecks.storage.ok ? '✅' : '❌' }}
            <span v-if="!testChecks.storage.ok" class="text-red-700">{{
              testChecks.storage.error
            }}</span>
          </p>
        </div>
      </div>

      <!-- Executar Backup Agora -->
      <div class="mb-6">
        <button
          type="button"
          @click="handleRunNow"
          :disabled="runningBackup"
          class="px-4 py-2 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="run-now-button"
        >
          <span v-if="!runningBackup">Executar Backup Agora</span>
          <span v-else>Enfileirando...</span>
        </button>
        <p
          v-if="runNowMessage"
          class="mt-2 text-sm text-gray-700"
          data-testid="run-now-message"
        >
          {{ runNowMessage }}
          <router-link
            :to="`/history?server=${serverId}`"
            class="text-blue-600 underline"
            >Ver histórico</router-link
          >
        </p>
      </div>

      <!-- Regenerar Chave / Resetar Host Key / Ativar-Desativar -->
      <div class="flex gap-4">
        <button
          type="button"
          @click="confirmAction = 'regenerate-key'"
          class="px-4 py-2 bg-yellow-600 text-white font-medium rounded-lg hover:bg-yellow-700 transition-colors"
          data-testid="regenerate-key-button"
        >
          Regenerar Chave SSH
        </button>
        <button
          type="button"
          @click="confirmAction = 'reset-host-key'"
          class="px-4 py-2 bg-orange-600 text-white font-medium rounded-lg hover:bg-orange-700 transition-colors"
          data-testid="reset-host-key-button"
        >
          Resetar Host Key
        </button>
        <button
          v-if="serverEnabled"
          type="button"
          @click="confirmAction = 'disable'"
          class="px-4 py-2 bg-red-600 text-white font-medium rounded-lg hover:bg-red-700 transition-colors"
          data-testid="disable-server-button"
        >
          Desativar Servidor
        </button>
        <button
          v-else
          type="button"
          @click="confirmAction = 'enable'"
          class="px-4 py-2 bg-green-600 text-white font-medium rounded-lg hover:bg-green-700 transition-colors"
          data-testid="enable-server-button"
        >
          Ativar Servidor
        </button>
      </div>
    </div>

    <ConfirmDialog
      :open="confirmAction !== null"
      :loading="confirmActionLoading"
      :title="confirmDialogTitle"
      :message="confirmDialogMessage"
      @confirm="handleConfirmAction"
      @cancel="cancelConfirmAction"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '@/api'
import type {
  StorageTargetDTO,
  StorageTargetType,
  UpsertServerRequest,
  TestConnectionChecks,
  DBEngine,
  DeploymentMode,
  ServerDTO,
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
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const route = useRoute()
const router = useRouter()
const serverId = route.params.id as string

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
const hasDbPassword = ref(false)
const pgDumpExtraArgs = ref('')
const mysqlDumpExtraArgs = ref('')
const sqlCmdExtraArgs = ref('')
const storageTargetId = ref('')
const cronExpression = ref('')

const dumpPreview = computed(() =>
  buildDumpCommandPreview({
    dbEngine: dbEngine.value,
    deploymentMode: deploymentMode.value,
    containerName: containerName.value,
    dbUser: dbUser.value,
    dbName: dbName.value,
    hasPassword: dbPassword.value.length > 0 || hasDbPassword.value,
    pgDumpExtraArgs: pgDumpExtraArgs.value,
    mysqlDumpExtraArgs: mysqlDumpExtraArgs.value,
    sqlCmdExtraArgs: sqlCmdExtraArgs.value,
  })
)

const storageTargets = ref<StorageTargetDTO[]>([])
const initialLoading = ref(true)
const loaded = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)

const serverStatus = ref('')
const serverEnabled = ref(false)
const sshPublicKey = ref<string | null>(null)
const copiedPublicKey = ref(false)

const statusLabels: Record<string, string> = {
  pending_key: 'Aguardando Chave',
  awaiting_authorization: 'Aguardando Autorização',
  ready: 'Pronto',
}
const statusLabel = computed(
  () => statusLabels[serverStatus.value] ?? serverStatus.value
)

// Retention policy (per-server override vs. global default)
const useGlobalRetention = ref(true)
const retentionRecentCount = ref(3)
const retentionMonthlyCount = ref(12)
const retentionInitialLoading = ref(true)
const retentionSaving = ref(false)
const retentionError = ref<string | null>(null)
const retentionSuccessMessage = ref<string | null>(null)

// Mirrors RN-BACKUP-004: recentCount and monthlyCount cannot both be zero.
const retentionValidationError = computed(() => {
  if (
    !useGlobalRetention.value &&
    retentionRecentCount.value === 0 &&
    retentionMonthlyCount.value === 0
  ) {
    return 'Backups recentes e meses de retenção não podem ser ambos zero.'
  }
  return null
})

// SSH lifecycle + backup actions
const testing = ref(false)
const testChecks = ref<TestConnectionChecks | null>(null)
const runningBackup = ref(false)
const runNowMessage = ref<string | null>(null)
const confirmAction = ref<
  'regenerate-key' | 'reset-host-key' | 'enable' | 'disable' | null
>(null)
const confirmActionLoading = ref(false)

const confirmDialogTitles: Record<
  'regenerate-key' | 'reset-host-key' | 'enable' | 'disable',
  string
> = {
  'regenerate-key': 'Regenerar Chave SSH',
  'reset-host-key': 'Resetar Host Key',
  enable: 'Ativar Servidor',
  disable: 'Desativar Servidor',
}
const confirmDialogMessages: Record<
  'regenerate-key' | 'reset-host-key' | 'enable' | 'disable',
  string
> = {
  'regenerate-key':
    'Isso reseta o status do servidor para "aguardando autorização" e desabilita o agendamento até um novo teste de conexão bem-sucedido.',
  'reset-host-key':
    'Isso limpa a impressão digital do host confiável (TOFU). Um novo teste de conexão será necessário para fixar a nova impressão digital.',
  enable:
    'O servidor volta a ser elegível para o agendamento automático (RN-BACKUP-002).',
  disable:
    'O agendamento automático deste servidor para imediatamente. Backups manuais ("Executar Backup Agora") continuam disponíveis.',
}
const confirmDialogTitle = computed(() =>
  confirmAction.value ? confirmDialogTitles[confirmAction.value] : ''
)
const confirmDialogMessage = computed(() =>
  confirmAction.value ? confirmDialogMessages[confirmAction.value] : ''
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
 * Load the server being edited, the storage targets for the dropdown, and
 * its effective retention policy. Storage targets/retention failures are
 * non-fatal (both are optional/have sane defaults); server load failure
 * is fatal — there's nothing to edit without it.
 */
onMounted(async () => {
  try {
    const server = await api.getServer(serverId)
    name.value = server.name
    host.value = server.host
    port.value = server.port
    sshUser.value = server.sshUser
    dbEngine.value = server.dbEngine
    deploymentMode.value = server.deploymentMode
    containerName.value = server.containerName ?? ''
    dbName.value = server.dbName
    dbUser.value = server.dbUser
    hasDbPassword.value = server.hasDbPassword
    pgDumpExtraArgs.value = server.pgDumpExtraArgs
    mysqlDumpExtraArgs.value = server.mysqlDumpExtraArgs
    sqlCmdExtraArgs.value = server.sqlCmdExtraArgs
    storageTargetId.value = server.storageTargetId ?? ''
    cronExpression.value = server.cronExpression
    serverStatus.value = server.status
    serverEnabled.value = server.enabled
    sshPublicKey.value = server.sshPublicKey ?? null
    loaded.value = true
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao carregar servidor. Tente novamente.'
  } finally {
    initialLoading.value = false
  }

  try {
    storageTargets.value = await api.getStorageTargets()
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] Failed to load storage targets:', err)
    }
  }

  try {
    const policy = await api.getServerRetentionPolicy(serverId)
    useGlobalRetention.value = policy.serverId !== serverId
    retentionRecentCount.value = policy.recentCount
    retentionMonthlyCount.value = policy.monthlyCount
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] Failed to load retention policy:', err)
    }
  } finally {
    retentionInitialLoading.value = false
  }
})

async function handleSubmit(): Promise<void> {
  saving.value = true
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
    await api.putServer(serverId, req)
    router.push('/servers')
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value =
      'Erro ao salvar servidor. Verifique os dados e tente novamente.'
  } finally {
    saving.value = false
  }
}

function handleCancel(): void {
  router.push('/servers')
}

/**
 * Save the retention policy: either remove the per-server override (fall
 * back to the global default) or upsert a custom one.
 */
async function handleSaveRetention(): Promise<void> {
  if (retentionValidationError.value || retentionSaving.value) {
    return
  }

  retentionSaving.value = true
  retentionError.value = null
  retentionSuccessMessage.value = null

  try {
    if (useGlobalRetention.value) {
      await api.deleteServerRetentionPolicy(serverId)
    } else {
      await api.putServerRetentionPolicy(serverId, {
        recentCount: retentionRecentCount.value,
        monthlyCount: retentionMonthlyCount.value,
      })
    }
    retentionSuccessMessage.value = 'Política de retenção salva com sucesso.'
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    retentionError.value =
      'Erro ao salvar política de retenção. Tente novamente.'
  } finally {
    retentionSaving.value = false
  }
}

/**
 * Copy the SSH public key to the clipboard, with a brief "Copiado!"
 * confirmation — the key itself isn't secret (it's the whole point of a
 * public key), so no confirmation dialog is needed here.
 */
async function handleCopyPublicKey(): Promise<void> {
  if (!sshPublicKey.value) {
    return
  }
  try {
    await navigator.clipboard.writeText(sshPublicKey.value)
    copiedPublicKey.value = true
    setTimeout(() => {
      copiedPublicKey.value = false
    }, 2000)
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] Failed to copy SSH public key:', err)
    }
  }
}

/**
 * Run the combined test-connection (RN-BACKUP-013): SSH + Docker +
 * storage (when configured). Updates the displayed status/enabled from the
 * response regardless of outcome.
 */
async function handleTestBackup(): Promise<void> {
  testing.value = true
  testChecks.value = null
  error.value = null

  try {
    const result = await api.postTestConnection(serverId)
    testChecks.value = result.checks
    serverStatus.value = result.status
    serverEnabled.value = result.enabled
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao testar backup. Tente novamente.'
  } finally {
    testing.value = false
  }
}

/**
 * Enqueue an async backup run (RN-BACKUP-006 prerequisites checked
 * server-side: status=ready, enabled=true, storageTargetId configured).
 */
async function handleRunNow(): Promise<void> {
  runningBackup.value = true
  runNowMessage.value = null
  error.value = null

  try {
    const result = await api.postRunNow(serverId)
    runNowMessage.value = result.message
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value =
      'Erro ao executar backup. Verifique se o servidor está pronto, habilitado, e tem um destino de backup configurado.'
  } finally {
    runningBackup.value = false
  }
}

/**
 * Cancel a pending regenerate-key/reset-host-key confirmation without
 * calling the API.
 */
function cancelConfirmAction(): void {
  if (confirmActionLoading.value) {
    return
  }
  confirmAction.value = null
}

/**
 * Run whichever SSH lifecycle action is pending confirmation. Guarded by
 * confirmActionLoading against double-submit, same pattern as the delete
 * confirmation flow elsewhere in this app.
 */
async function handleConfirmAction(): Promise<void> {
  if (!confirmAction.value || confirmActionLoading.value) {
    return
  }

  confirmActionLoading.value = true
  error.value = null

  try {
    let updated: ServerDTO
    switch (confirmAction.value) {
      case 'regenerate-key':
        updated = await api.postRegenerateKey(serverId)
        break
      case 'reset-host-key':
        updated = await api.postResetHostKey(serverId)
        break
      case 'enable':
        updated = await api.postEnableServer(serverId)
        break
      case 'disable':
        updated = await api.postDisableServer(serverId)
        break
      default:
        return
    }
    serverStatus.value = updated.status
    serverEnabled.value = updated.enabled
    sshPublicKey.value = updated.sshPublicKey ?? null
    confirmAction.value = null
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao executar ação. Tente novamente.'
    confirmAction.value = null
  } finally {
    confirmActionLoading.value = false
  }
}
</script>
