<template>
  <div class="p-6 max-w-2xl mx-auto">
    <h1 class="text-3xl font-bold mb-6 text-gray-800">Novo Administrador</h1>

    <!-- Error Alert -->
    <div
      v-if="error"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro ao criar administrador</p>
      <p>{{ error }}</p>
    </div>

    <form
      @submit.prevent="handleSubmit"
      class="bg-white p-6 rounded-lg shadow space-y-4"
      data-testid="admin-user-form"
    >
      <div>
        <Label for="email">Email</Label>
        <Input
          id="email"
          v-model="email"
          type="email"
          required
          :disabled="loading"
          data-testid="email-input"
        />
      </div>

      <div>
        <Label for="cpf">CPF</Label>
        <Input
          id="cpf"
          v-model="cpf"
          type="text"
          maxlength="14"
          required
          :disabled="loading"
          data-testid="cpf-input"
        />
      </div>

      <div>
        <Label for="password">Senha</Label>
        <Input
          id="password"
          v-model="password"
          type="password"
          minlength="12"
          required
          :disabled="loading"
          data-testid="password-input"
        />
        <p class="text-sm text-gray-500 mt-1">Mínimo de 12 caracteres.</p>
      </div>

      <div class="flex gap-3 pt-2">
        <Button type="submit" :disabled="loading" data-testid="submit-button">
          <span v-if="!loading">Criar Administrador</span>
          <span v-else>Criando...</span>
        </Button>
        <Button
          type="button"
          variant="outline"
          :disabled="loading"
          @click="handleCancel"
          data-testid="cancel-button"
        >
          Cancelar
        </Button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '@/api'
import type { CreateAdminUserRequest } from '@/api'
import { apiErrorMessage, logApiError } from '@/lib/apiError'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const router = useRouter()

const email = ref('')
const password = ref('')

const loading = ref(false)
const error = ref<string | null>(null)

/** Digits-only mask "000.000.000-00" applied progressively as the operator types. */
function maskCPF(value: string): string {
  const digits = value.replace(/\D/g, '').slice(0, 11)
  let out = digits.slice(0, 3)
  if (digits.length > 3) out += '.' + digits.slice(3, 6)
  if (digits.length > 6) out += '.' + digits.slice(6, 9)
  if (digits.length > 9) out += '-' + digits.slice(9)
  return out
}

const cpfRaw = ref('')
const cpf = computed<string>({
  get: () => cpfRaw.value,
  set: (val: string) => {
    cpfRaw.value = maskCPF(val)
  },
})

async function handleSubmit(): Promise<void> {
  loading.value = true
  error.value = null

  const req: CreateAdminUserRequest = {
    email: email.value,
    cpf: cpf.value.replace(/\D/g, ''),
    password: password.value,
  }

  try {
    await api.postAdminUser(req)
    router.push('/admin-users')
  } catch (err) {
    logApiError('Create admin user failed', err)
    error.value = apiErrorMessage(
      err,
      'Erro ao criar administrador. Verifique os dados e tente novamente.'
    )
  } finally {
    loading.value = false
  }
}

function handleCancel(): void {
  router.push('/admin-users')
}
</script>
