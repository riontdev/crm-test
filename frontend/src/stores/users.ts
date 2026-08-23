import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useUiStore } from '@/stores/ui'

export type UserRole = 'admin' | 'agente'

export interface UserRow {
  id: string
  email: string
  name: string
  role: UserRole
}

export interface CreateUserPayload {
  email: string
  name: string
  password: string
  role: UserRole
}

export interface UpdateUserPayload {
  name?: string
  role?: UserRole
  password?: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    if (res.status === 401) {
      window.dispatchEvent(new CustomEvent('crm:unauthorized'))
    }
    throw new Error(body.error || 'Request failed')
  }

  return res.json()
}

export const useUsersStore = defineStore('users', () => {
  const ui = useUiStore()
  const users = ref<UserRow[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchUsers() {
    loading.value = true
    error.value = null
    try {
      const res = await request<{ data: UserRow[]; count: number }>('/users')
      users.value = res.data || []
    } catch (e: any) {
      error.value = e.message || 'No se pudieron cargar los usuarios'
    } finally {
      loading.value = false
    }
  }

  async function createUser(payload: CreateUserPayload): Promise<string | null> {
    try {
      await request<{ user: UserRow }>('/users', {
        method: 'POST',
        body: JSON.stringify(payload),
      })
      await fetchUsers()
      ui.success('Usuario creado')
      return null
    } catch (e: any) {
      return e.message || 'No se pudo crear el usuario'
    }
  }

  async function updateUser(id: string, patch: UpdateUserPayload): Promise<string | null> {
    const index = users.value.findIndex((u) => u.id === id)
    if (index === -1) return 'Usuario no encontrado'

    const previous = users.value[index]
    users.value[index] = {
      ...previous,
      ...(patch.name !== undefined ? { name: patch.name } : {}),
      ...(patch.role !== undefined ? { role: patch.role } : {}),
    }

    try {
      const res = await request<{ user: UserRow }>(`/users/${id}`, {
        method: 'PUT',
        body: JSON.stringify(patch),
      })
      users.value[index] = res.user
      ui.success('Usuario actualizado')
      return null
    } catch (e: any) {
      users.value[index] = previous
      const message = e.message || 'No se pudo actualizar el usuario'
      ui.error(message)
      return message
    }
  }

  async function deleteUser(id: string): Promise<string | null> {
    const index = users.value.findIndex((u) => u.id === id)
    if (index === -1) return 'Usuario no encontrado'

    const previous = users.value[index]
    users.value.splice(index, 1)

    try {
      await request<{ ok: boolean }>(`/users/${id}`, { method: 'DELETE' })
      ui.success('Usuario eliminado')
      return null
    } catch (e: any) {
      users.value.splice(index, 0, previous)
      const message = e.message || 'No se pudo eliminar el usuario'
      ui.error(message)
      return message
    }
  }

  return { users, loading, error, fetchUsers, createUser, updateUser, deleteUser }
})
