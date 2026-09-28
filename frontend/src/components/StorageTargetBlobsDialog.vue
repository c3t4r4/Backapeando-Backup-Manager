<template>
  <Dialog :open="isOpen" @update:open="handleOpenChange">
    <DialogContent v-if="isOpen" class="max-w-2xl">
      <DialogHeader>
        <DialogTitle>Blobs do destino de backup</DialogTitle>
        <DialogDescription class="sr-only">
          Lista de todos os backups armazenados neste destino com ações de cópia e exclusão.
        </DialogDescription>
      </DialogHeader>

      <div v-if="loading" class="flex justify-center items-center py-8">
        <div class="text-gray-500">Carregando blobs...</div>
      </div>

      <div v-else-if="blobs.length === 0" class="text-center py-8">
        <p class="text-gray-500">Nenhum blob encontrado neste destino</p>
      </div>

      <div v-else class="space-y-4">
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b">
                <th class="text-left py-2 px-3 font-semibold">Nome</th>
                <th class="text-right py-2 px-3 font-semibold">Tamanho</th>
                <th class="text-right py-2 px-3 font-semibold">Data</th>
                <th class="text-right py-2 px-3 font-semibold">Ações</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="blob in blobs" :key="blob.name" class="border-b hover:bg-gray-50">
                <td class="py-2 px-3 break-all text-gray-700">{{ blob.name }}</td>
                <td class="py-2 px-3 text-right text-gray-600">{{ formatSize(blob.sizeBytes) }}</td>
                <td class="py-2 px-3 text-right text-gray-600">{{ formatDate(blob.lastModified) }}</td>
                <td class="py-2 px-3 text-right space-x-2">
                  <button
                    @click="copyBlobName(blob.name)"
                    class="text-blue-600 hover:text-blue-800 text-xs font-medium"
                    :disabled="deleting === blob.name"
                  >
                    Copiar
                  </button>
                  <button
                    @click="deleteBlob(blob.name)"
                    class="text-red-600 hover:text-red-800 text-xs font-medium"
                    :disabled="deleting === blob.name"
                  >
                    {{ deleting === blob.name ? 'Deletando...' : 'Deletar' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { deleteStorageTargetBlob, getStorageTargetBlobs } from '@/api'
import { formatSize, formatDate } from '@/lib/format'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

interface BlobInfo {
  name: string
  lastModified: string
  sizeBytes: number
}

defineProps<{
  storageTargetId: string
  isOpen: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const blobs = ref<BlobInfo[]>([])
const loading = ref(false)
const deleting = ref<string | null>(null)

async function loadBlobs(): Promise<void> {
  loading.value = true
  try {
    const result = await getStorageTargetBlobs(storageTargetId)
    blobs.value = result.blobs
  } catch {
    // silently fail — error handling is minimal per spec
  } finally {
    loading.value = false
  }
}

async function deleteBlob(blobName: string): Promise<void> {
  if (!confirm(`Tem certeza que deseja deletar "${blobName}"?`)) {
    return
  }

  deleting.value = blobName
  try {
    await deleteStorageTargetBlob(storageTargetId, blobName)
    blobs.value = blobs.value.filter((b) => b.name !== blobName)
  } catch {
    // silently fail
  } finally {
    deleting.value = null
  }
}

function copyBlobName(blobName: string): void {
  navigator.clipboard.writeText(blobName).catch(() => {
    // silently fail — copy is convenience, not critical
  })
}

function handleOpenChange(open: boolean): void {
  if (open) {
    loadBlobs()
  } else {
    emit('close')
  }
}

defineExpose({
  storageTargetId,
  isOpen,
})
</script>
