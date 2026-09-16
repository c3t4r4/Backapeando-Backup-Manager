<template>
  <div
    v-if="open"
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    data-testid="confirm-dialog"
  >
    <div class="bg-background text-foreground border rounded-lg shadow-lg p-6 w-full max-w-sm">
      <h2
        class="text-lg font-semibold mb-2"
        data-testid="confirm-dialog-title"
      >
        {{ title }}
      </h2>
      <p class="text-muted-foreground mb-6" data-testid="confirm-dialog-message">
        {{ message }}
      </p>
      <div class="flex justify-end gap-3">
        <Button
          variant="outline"
          @click="$emit('cancel')"
          :disabled="loading"
          data-testid="confirm-dialog-cancel"
        >
          Cancelar
        </Button>
        <Button
          variant="destructive"
          @click="$emit('confirm')"
          :disabled="loading"
          data-testid="confirm-dialog-confirm"
        >
          <span v-if="!loading">Confirmar</span>
          <span v-else>Confirmando...</span>
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Button } from '@/components/ui/button'

interface Props {
  open: boolean
  title: string
  message: string
  loading?: boolean
}

withDefaults(defineProps<Props>(), {
  loading: false,
})

defineEmits<{
  confirm: []
  cancel: []
}>()
</script>
