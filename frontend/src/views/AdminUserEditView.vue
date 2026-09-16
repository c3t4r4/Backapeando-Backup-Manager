<template>
  <div class="p-6 max-w-2xl mx-auto">
    <h1 class="text-3xl font-bold mb-6 text-gray-800">Editar Administrador</h1>

    <LoadingSpinner
      v-if="initialLoading"
      size="lg"
      message="Carregando administrador..."
      data-testid="loading-spinner"
    />

    <template v-else>
      <!-- Error Alert -->
      <div
        v-if="error"
        class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
        role="alert"
        data-testid="error-alert"
      >
        <p class="font-bold">Erro</p>
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
            :disabled="loading"
            placeholder="Deixe em branco para manter a atual"
            data-testid="password-input"
          />
          <p class="text-sm text-gray-500 mt-1">
            Deixe em branco para manter a senha atual. Se preenchida, mínimo de
            12 caracteres.
          </p>
        </div>

        <div class="flex gap-3 pt-2">
          <Button type="submit" :disabled="loading" data-testid="submit-button">
            <span v-if="!loading">Salvar</span>
            <span v-else>Salvando...</span>
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
          <Button
            type="button"
            variant="destructive"
            class="ml-auto"
            :disabled="loading || isSelf"
            :title="
              isSelf ? 'Você não pode excluir sua própria conta' : undefined
            "
            @click="handleDeleteClick"
            data-testid="delete-button"
          >
            Excluir
          </Button>
        </div>
      </form>

      <ConfirmDialog
        :open="confirmingDelete"
        :loading="deleting"
        title="Excluir administrador"
        message="Esta ação não pode ser desfeita."
        @confirm="confirmDelete"
        @cancel="confirmingDelete = false"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '@/api'
import type { UpdateAdminUserRequest } from '@/api'
import { useAuth } from '@/composables/useAuth'
import { apiErrorMessage, logApiError } from '@/lib/apiError'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const route = useRoute()
const router = useRouter()
const { user } = useAuth()

const userId = route.params.id as string

const email = ref('')
const cpf = ref('')
const password = ref('')

const initialLoading = ref(true)
const loading = ref(false)
const error = ref<string | null>(null)

const confirmingDelete = ref(false)
const deleting = ref(false)

const isSelf = computed(() => userId === user.value?.id)

async function fetchUser(): Promise<void> {
  initialLoading.value = true
  error.value = null

  try {
    const existing = await api.getAdminUser(userId)
    email.value = existing.email
    cpf.value = existing.cpf
  } catch (err) {
    logApiError('Fetch admin user failed', err)
    error.value = apiErrorMessage(
      err,
      'Erro ao carregar administrador. Tente novamente.'
    )
  } finally {
    initialLoading.value = false
  }
}

async function handleSubmit(): Promise<void> {
  loading.value = true
  error.value = null

  const req: UpdateAdminUserRequest = {
    email: email.value,
    cpf: cpf.value.replace(/\D/g, ''),
    ...(password.value ? { password: password.value } : {}),
  }

  try {
    await api.putAdminUser(userId, req)
    router.push('/admin-users')
  } catch (err) {
    logApiError('Update admin user failed', err)
    error.value = apiErrorMessage(
      err,
      'Erro ao salvar administrador. Verifique os dados e tente novamente.'
    )
  } finally {
    loading.value = false
  }
}

function handleCancel(): void {
  router.push('/admin-users')
}

function handleDeleteClick(): void {
  if (isSelf.value) {
    return
  }
  confirmingDelete.value = true
}

async function confirmDelete(): Promise<void> {
  if (deleting.value) {
    return
  }
  deleting.value = true
  try {
    await api.deleteAdminUser(userId)
    router.push('/admin-users')
  } catch (err) {
    logApiError('Delete admin user failed', err)
    error.value = apiErrorMessage(
      err,
      'Erro ao excluir administrador. Tente novamente.'
    )
    confirmingDelete.value = false
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  fetchUser()
})
</script>
