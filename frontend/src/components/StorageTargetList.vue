<template>
  <div class="overflow-x-auto">
    <table
      class="w-full border-collapse border border-gray-300"
      data-testid="storage-targets-table"
    >
      <thead>
        <tr class="bg-gray-100">
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Nome
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Tipo
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Detalhes
          </th>
          <th
            class="border border-gray-300 px-4 py-2 text-left font-semibold text-gray-700"
          >
            Ações
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="target in targets"
          :key="target.id"
          class="hover:bg-gray-50 border-b border-gray-300"
          :data-testid="`storage-target-row-${target.id}`"
        >
          <td class="border border-gray-300 px-4 py-3">
            <span class="font-medium text-gray-800">{{ target.name }}</span>
          </td>
          <td class="border border-gray-300 px-4 py-3">
            <span
              class="inline-block px-2 py-1 rounded text-xs font-semibold uppercase"
              :class="typeBadgeClass(target.type)"
              :data-testid="`type-badge-${target.id}`"
            >
              {{ typeLabel(target.type) }}
            </span>
          </td>
          <td class="border border-gray-300 px-4 py-3 text-gray-600">
            {{ targetDetails(target) }}
          </td>
          <td class="border border-gray-300 px-4 py-3">
            <div class="flex gap-2 flex-wrap">
              <button
                @click="$emit('edit', target.id)"
                class="px-3 py-1 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700 transition-colors"
                :data-testid="`edit-btn-${target.id}`"
              >
                Editar
              </button>
              <button
                @click="$emit('delete', target.id)"
                class="px-3 py-1 bg-red-600 text-white rounded text-sm font-medium hover:bg-red-700 transition-colors"
                :data-testid="`delete-btn-${target.id}`"
              >
                Deletar
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import type { StorageTargetDTO, StorageTargetType } from '@/api'

interface Props {
  targets: StorageTargetDTO[]
}

defineProps<Props>()

defineEmits<{
  edit: [targetId: string]
  delete: [targetId: string]
}>()

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

function typeBadgeClass(type: StorageTargetType): string {
  switch (type) {
    case 'azure':
      return 'bg-blue-100 text-blue-800'
    case 's3':
      return 'bg-orange-100 text-orange-800'
    case 'filesystem':
      return 'bg-gray-200 text-gray-800'
    default:
      return 'bg-gray-200 text-gray-800'
  }
}

function targetDetails(target: StorageTargetDTO): string {
  switch (target.type) {
    case 'azure':
      return `${target.accountName ?? ''} / ${target.containerName ?? ''}`
    case 's3':
      return `${target.bucket ?? ''}${target.region ? ` (${target.region})` : ''}${
        target.endpoint ? ` @ ${target.endpoint}` : ''
      }`
    case 'filesystem':
      return target.rootPath ?? ''
    default:
      return ''
  }
}
</script>
