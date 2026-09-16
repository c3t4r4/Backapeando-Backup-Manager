<template>
  <div class="flex flex-col items-center justify-center gap-4">
    <div
      :class="spinnerSizeClass"
      class="border-4 border-gray-300 border-t-blue-600 rounded-full animate-spin"
      data-testid="spinner"
    />
    <p
      v-if="message"
      class="text-gray-600 text-sm"
      data-testid="spinner-message"
    >
      {{ message }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  size?: 'sm' | 'md' | 'lg'
  message?: string
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  message: '',
})

const spinnerSizeClass = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'w-8 h-8'
    case 'lg':
      return 'w-16 h-16'
    case 'md':
    default:
      return 'w-12 h-12'
  }
})
</script>
