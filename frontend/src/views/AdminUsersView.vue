<template>
  <div class="p-6">
    <!-- Header -->
    <h1 class="text-4xl font-bold mb-6 text-gray-800">Administradores</h1>

    <!-- Action Buttons -->
    <div class="mb-6 flex gap-4">
      <Button @click="handleCreate" data-testid="create-admin-user-btn">
        + Novo Administrador
      </Button>
      <Button
        variant="outline"
        @click="handleRefresh"
        :disabled="loading"
        data-testid="refresh-btn"
      >
        Atualizar
      </Button>
    </div>

    <!-- Loading Spinner Overlay -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      message="Carregando administradores..."
      data-testid="loading-spinner"
    />

    <!-- Error Alert -->
    <div
      v-if="error && !loading"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro</p>
      <p>{{ error }}</p>
    </div>

    <!-- Admin Users Table -->
    <Table v-if="!loading && users.length > 0" data-testid="admin-users-table">
      <TableHeader>
        <TableRow>
          <TableHead>Email</TableHead>
          <TableHead>CPF</TableHead>
          <TableHead>Criado em</TableHead>
          <TableHead>Último login</TableHead>
          <TableHead>Ações</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow
          v-for="user in users"
          :key="user.id"
          :data-testid="`admin-user-row-${user.id}`"
        >
          <TableCell>{{ user.email }}</TableCell>
          <TableCell>{{ formatCPF(user.cpf) }}</TableCell>
          <TableCell>{{ formatDate(user.createdAt) }}</TableCell>
          <TableCell>{{ formatDate(user.lastLoginAt) }}</TableCell>
          <TableCell>
            <div class="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                @click="handleEdit(user.id)"
                :data-testid="`edit-btn-${user.id}`"
              >
                Editar
              </Button>
              <Button
                variant="destructive"
                size="sm"
                :disabled="user.id === currentUserId"
                :title="
                  user.id === currentUserId
                    ? 'Você não pode excluir sua própria conta'
                    : undefined
                "
                @click="handleDelete(user.id)"
                :data-testid="`delete-btn-${user.id}`"
              >
                Excluir
              </Button>
            </div>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :open="userPendingDelete !== null"
      :loading="deleting"
      title="Excluir administrador"
      message="Esta ação não pode ser desfeita."
      @confirm="confirmDelete"
      @cancel="cancelDelete"
    />

    <!-- Empty State -->
    <div
      v-if="!loading && users.length === 0 && !error"
      class="text-center py-12"
      data-testid="empty-state"
    >
      <p class="text-gray-500 mb-6 text-lg">Nenhum administrador cadastrado</p>
      <Button @click="handleCreate" data-testid="create-first-admin-user-btn">
        Criar Primeiro Administrador
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '@/api'
import type { AdminUserDTO } from '@/api'
import { useAuth } from '@/composables/useAuth'
import { apiErrorMessage } from '@/lib/apiError'
import { formatDate, formatCPF } from '@/lib/format'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table'

const router = useRouter()
const { user } = useAuth()

const users = ref<AdminUserDTO[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const userPendingDelete = ref<string | null>(null)
const deleting = ref(false)

const currentUserId = computed(() => user.value?.id)

async function fetchUsers(): Promise<void> {
  loading.value = true
  error.value = null

  try {
    users.value = await api.getAdminUsers()
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao carregar administradores. Tente novamente.'
    users.value = []
  } finally {
    loading.value = false
  }
}

function handleCreate(): void {
  router.push('/admin-users/new')
}

function handleRefresh(): void {
  fetchUsers()
}

function handleEdit(userId: string): void {
  router.push(`/admin-users/${userId}/edit`)
}

function handleDelete(userId: string): void {
  if (userId === currentUserId.value) {
    return
  }
  userPendingDelete.value = userId
}

async function confirmDelete(): Promise<void> {
  const userId = userPendingDelete.value
  if (!userId || deleting.value) {
    return
  }

  deleting.value = true
  try {
    await api.deleteAdminUser(userId)
    users.value = users.value.filter((u) => u.id !== userId)
    userPendingDelete.value = null
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = apiErrorMessage(
      err,
      'Erro ao excluir administrador. Tente novamente.'
    )
    userPendingDelete.value = null
  } finally {
    deleting.value = false
  }
}

function cancelDelete(): void {
  if (deleting.value) {
    return
  }
  userPendingDelete.value = null
}

onMounted(() => {
  fetchUsers()
})
</script>
