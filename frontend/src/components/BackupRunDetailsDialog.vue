<template>
  <Dialog :open="backup !== null" @update:open="handleOpenChange">
    <DialogContent v-if="backup">
      <div data-testid="backup-details-dialog">
        <DialogHeader>
          <DialogTitle>Detalhes da execução</DialogTitle>
          <DialogDescription class="sr-only">
            Informações completas da execução de backup selecionada.
          </DialogDescription>
        </DialogHeader>

        <dl class="space-y-2 text-sm">
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Servidor</dt>
            <dd data-testid="detail-server">
              {{ formatServerId(backup.serverId) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Status</dt>
            <dd data-testid="detail-status">
              {{ formatBackupStatus(backup.status) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Iniciado</dt>
            <dd data-testid="detail-started">
              {{ formatDate(backup.startedAt) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Concluído</dt>
            <dd data-testid="detail-finished">
              {{ formatDate(backup.finishedAt) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Nome do blob</dt>
            <dd data-testid="detail-blob-name" class="truncate">
              {{ backup.blobName ?? '—' }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Tamanho</dt>
            <dd data-testid="detail-size">
              {{ formatSize(backup.blobSizeBytes) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Duração do dump</dt>
            <dd data-testid="detail-dump-duration">
              {{ formatDurationMs(backup.dumpDurationMs) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="font-semibold text-gray-700">Duração do upload</dt>
            <dd data-testid="detail-upload-duration">
              {{ formatDurationMs(backup.uploadDurationMs) }}
            </dd>
          </div>
          <div v-if="backup.errorMessage">
            <dt class="font-semibold text-gray-700 mb-1">Mensagem de erro</dt>
            <dd
              data-testid="detail-error-message"
              class="text-red-700 bg-red-50 rounded p-2"
            >
              {{ backup.errorMessage }}
            </dd>
          </div>
          <div v-if="backup.logOutput">
            <dt class="font-semibold text-gray-700 mb-1">Log de saída</dt>
            <dd data-testid="detail-log-output">
              <pre
                class="bg-gray-100 rounded p-2 text-xs overflow-x-auto whitespace-pre-wrap"
                >{{ backup.logOutput }}</pre>
            </dd>
          </div>
        </dl>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import type { BackupRunDTO } from '@/api'
import {
  formatDate,
  formatSize,
  formatDurationMs,
  formatBackupStatus,
  formatServerId,
} from '@/lib/format'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

defineProps<{
  backup: BackupRunDTO | null
}>()

const emit = defineEmits<{
  close: []
}>()

function handleOpenChange(open: boolean): void {
  if (!open) {
    emit('close')
  }
}
</script>
