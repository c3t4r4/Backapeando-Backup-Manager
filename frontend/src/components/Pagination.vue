<template>
  <div class="flex items-center justify-between mt-6 gap-4">
    <!-- Previous Button -->
    <button
      @click="goToPrevious"
      :disabled="currentPage === 1"
      class="px-4 py-2 bg-gray-600 text-white rounded-lg font-medium hover:bg-gray-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
      data-testid="pagination-prev"
      aria-label="Página anterior"
    >
      Anterior
    </button>

    <!-- Page Info -->
    <div class="flex items-center gap-4">
      <span class="text-gray-700 font-medium" data-testid="pagination-info">
        Página {{ currentPage }} de {{ maxPages }}
      </span>

      <!-- Jump to Page Input -->
      <div class="flex items-center gap-2">
        <label for="jump-page" class="text-gray-700 text-sm">
          Ir para:
        </label>
        <input
          id="jump-page"
          v-model.number="jumpPageInput"
          type="number"
          min="1"
          :max="maxPages"
          @keyup.enter="handleJumpToPage"
          class="w-16 px-2 py-1 border border-gray-300 rounded text-center text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          data-testid="pagination-jump-input"
          :disabled="props.totalItems === 0"
        />
        <button
          @click="handleJumpToPage"
          :disabled="props.totalItems === 0"
          class="px-3 py-1 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          data-testid="pagination-jump-btn"
        >
          Ir
        </button>
      </div>
    </div>

    <!-- Next Button -->
    <button
      @click="goToNext"
      :disabled="currentPage >= maxPages"
      class="px-4 py-2 bg-gray-600 text-white rounded-lg font-medium hover:bg-gray-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
      data-testid="pagination-next"
      aria-label="Próxima página"
    >
      Próxima
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

interface Props {
  currentPage: number
  pageSize: number
  totalItems: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:page': [page: number]
}>()

const jumpPageInput = ref<number | null>(null)

/**
 * Calculate maximum number of pages
 */
const maxPages = computed(() => {
  return Math.max(1, Math.ceil(props.totalItems / props.pageSize))
})

/**
 * Go to previous page
 */
function goToPrevious(): void {
  if (props.currentPage > 1) {
    emit('update:page', props.currentPage - 1)
  }
}

/**
 * Go to next page
 */
function goToNext(): void {
  if (props.currentPage < maxPages.value) {
    emit('update:page', props.currentPage + 1)
  }
}

/**
 * Jump to a specific page
 */
function handleJumpToPage(): void {
  if (jumpPageInput.value === null) {
    return
  }

  const page = Math.max(1, Math.min(jumpPageInput.value, maxPages.value))
  if (page !== props.currentPage) {
    emit('update:page', page)
  }

  jumpPageInput.value = null
}
</script>
