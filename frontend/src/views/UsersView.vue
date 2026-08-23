<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  useUsersStore,
  type UserRow,
  type UserRole,
  type UpdateUserPayload,
} from '@/stores/users'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Avatar from '@/components/ui/Avatar.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import IconButton from '@/components/ui/IconButton.vue'

const store = useUsersStore()

const modalOpen = ref(false)
const editingUser = ref<UserRow | null>(null)
const isEdit = computed(() => editingUser.value !== null)

const form = reactive<{
  name: string
  email: string
  role: UserRole
  password: string
}>({
  name: '',
  email: '',
  role: 'agente',
  password: '',
})

const formError = ref('')
const saving = ref(false)

function roleLabel(role: string): string {
  return role === 'admin' ? 'Admin' : 'Agente'
}

function openCreate() {
  editingUser.value = null
  form.name = ''
  form.email = ''
  form.role = 'agente'
  form.password = ''
  formError.value = ''
  modalOpen.value = true
}

function openEdit(user: UserRow) {
  editingUser.value = user
  form.name = user.name
  form.email = user.email
  form.role = user.role
  form.password = ''
  formError.value = ''
  modalOpen.value = true
}

function closeModal() {
  if (saving.value) return
  modalOpen.value = false
}

async function handleSubmit() {
  if (saving.value) return
  formError.value = ''

  const name = form.name.trim()
  const email = form.email.trim()
  const password = form.password

  if (!name) {
    formError.value = 'El nombre es obligatorio'
    return
  }
  if (!isEdit.value && !email) {
    formError.value = 'El email es obligatorio'
    return
  }
  if (!isEdit.value && password.length < 6) {
    formError.value = 'La contraseña debe tener al menos 6 caracteres'
    return
  }

  saving.value = true

  let err: string | null
  if (isEdit.value && editingUser.value) {
    const patch: UpdateUserPayload = { name, role: form.role }
    if (password) patch.password = password
    err = await store.updateUser(editingUser.value.id, patch)
  } else {
    err = await store.createUser({ name, email, password, role: form.role })
  }

  saving.value = false

  if (err === null) {
    modalOpen.value = false
  } else {
    formError.value = err
  }
}

function onKeydown(e: KeyboardEvent) {
  if (modalOpen.value && e.key === 'Escape') closeModal()
}

onMounted(() => {
  store.fetchUsers()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
})

const inputClasses =
  'mt-1 w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm text-slate-800 transition-all duration-200 placeholder:text-slate-400 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500'

const labelClasses =
  'block text-xs font-semibold uppercase tracking-[0.05em] text-slate-500 dark:text-slate-400'

const thClasses =
  'px-6 py-3 text-xs font-semibold uppercase tracking-[0.05em] text-slate-500 dark:text-slate-400'
</script>

<template>
  <div class="mx-auto w-full max-w-6xl px-6 py-8">
    <header class="mb-6 flex flex-wrap items-start gap-4">
      <div class="min-w-0">
        <h1 class="text-2xl font-semibold tracking-[-0.01em] text-slate-900 dark:text-slate-100">
          Usuarios
        </h1>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Gestioná quién accede al CRM
        </p>
      </div>
      <Button size="sm" class="ml-auto" @click="openCreate">
        <span class="material-symbols-outlined text-base" aria-hidden="true">add</span>
        Nuevo usuario
      </Button>
    </header>

    <div
      class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-[#101828]"
    >
      <!-- Loading -->
      <div v-if="store.loading" class="divide-y divide-slate-100 dark:divide-slate-800" aria-hidden="true">
        <div v-for="i in 4" :key="i" class="flex items-center gap-4 px-6 py-5">
          <div class="h-10 w-10 shrink-0 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
          <Skeleton :lines="2" class="flex-1" />
        </div>
      </div>

      <!-- Error -->
      <EmptyState
        v-else-if="store.error"
        icon="cloud_off"
        title="No se pudieron cargar los usuarios"
        :description="store.error"
      >
        <template #action>
          <Button size="sm" @click="store.fetchUsers()">Reintentar</Button>
        </template>
      </EmptyState>

      <!-- Vacío -->
      <EmptyState
        v-else-if="store.users.length === 0"
        icon="group"
        title="No hay usuarios"
        description="Creá el primer usuario para que pueda acceder al CRM."
      />

      <template v-else>
        <!-- Tabla desktop -->
        <table class="hidden w-full text-left md:table">
          <thead class="bg-slate-50 dark:bg-slate-800/60">
            <tr>
              <th :class="thClasses">Usuario</th>
              <th :class="thClasses">Email</th>
              <th :class="thClasses">Rol</th>
              <th :class="[thClasses, 'text-right']">Acciones</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
            <tr v-for="user in store.users" :key="user.id">
              <td class="px-6 py-4">
                <div class="flex items-center gap-3">
                  <Avatar :name="user.name" size="md" />
                  <div class="min-w-0">
                    <p class="truncate font-semibold text-slate-800 dark:text-slate-100">
                      {{ user.name }}
                    </p>
                    <p class="truncate text-xs text-slate-500 dark:text-slate-400">
                      {{ user.email }}
                    </p>
                  </div>
                </div>
              </td>
              <td class="px-6 py-4 text-slate-600 dark:text-slate-300">{{ user.email }}</td>
              <td class="px-6 py-4">
                <Badge :variant="user.role === 'admin' ? 'accent' : 'default'">
                  {{ roleLabel(user.role) }}
                </Badge>
              </td>
              <td class="px-6 py-4 text-right">
                <div class="flex justify-end gap-1">
                  <IconButton icon="edit" size="sm" @click="openEdit(user)" />
                  <IconButton icon="delete" size="sm" @click="store.deleteUser(user.id)" />
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Lista mobile -->
        <ul class="md:hidden">
          <li
            v-for="user in store.users"
            :key="user.id"
            class="flex items-start justify-between gap-3 border-b border-slate-100 p-4 last:border-b-0 dark:border-slate-800"
          >
            <div class="flex min-w-0 items-center gap-3">
              <Avatar :name="user.name" size="md" />
              <div class="min-w-0">
                <p class="truncate font-semibold text-slate-800 dark:text-slate-100">
                  {{ user.name }}
                </p>
                <p class="truncate text-xs text-slate-500 dark:text-slate-400">{{ user.email }}</p>
                <Badge
                  class="mt-1"
                  :variant="user.role === 'admin' ? 'accent' : 'default'"
                  size="sm"
                >
                  {{ roleLabel(user.role) }}
                </Badge>
              </div>
            </div>
            <div class="flex shrink-0 gap-1">
              <IconButton icon="edit" size="sm" @click="openEdit(user)" />
              <IconButton icon="delete" size="sm" @click="store.deleteUser(user.id)" />
            </div>
          </li>
        </ul>
      </template>
    </div>

    <!-- Modal crear / editar -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0"
        leave-active-class="transition duration-150 ease-in"
        leave-to-class="opacity-0"
      >
        <div
          v-if="modalOpen"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          :aria-label="isEdit ? 'Editar usuario' : 'Nuevo usuario'"
          @click.self="closeModal"
        >
          <div
            class="w-full max-w-md rounded-xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-700 dark:bg-[#101828]"
          >
            <h2 class="text-lg font-semibold text-slate-900 dark:text-slate-100">
              {{ isEdit ? 'Editar usuario' : 'Nuevo usuario' }}
            </h2>

            <form class="mt-5 space-y-4" @submit.prevent="handleSubmit">
              <div>
                <label for="user-name" :class="labelClasses">Nombre</label>
                <input
                  id="user-name"
                  v-model="form.name"
                  type="text"
                  autocomplete="off"
                  placeholder="Juan Pérez"
                  :class="inputClasses"
                />
              </div>

              <div>
                <label for="user-email" :class="labelClasses">Email</label>
                <input
                  id="user-email"
                  v-model="form.email"
                  type="text"
                  autocomplete="off"
                  placeholder="juan@empresa.com"
                  :disabled="isEdit"
                  :class="inputClasses"
                />
              </div>

              <div>
                <label for="user-role" :class="labelClasses">Rol</label>
                <select id="user-role" v-model="form.role" :class="inputClasses">
                  <option value="admin">Administrador</option>
                  <option value="agente">Agente</option>
                </select>
              </div>

              <div>
                <label for="user-password" :class="labelClasses">Contraseña</label>
                <input
                  id="user-password"
                  v-model="form.password"
                  type="password"
                  autocomplete="new-password"
                  :placeholder="isEdit ? 'Dejar vacío para mantener la actual' : 'Mínimo 6 caracteres'"
                  :class="inputClasses"
                />
              </div>

              <p v-if="formError" class="text-xs text-red-500" role="alert">{{ formError }}</p>

              <div class="flex justify-end gap-2 pt-2">
                <Button variant="ghost" @click="closeModal">Cancelar</Button>
                <Button type="submit" :loading="saving">Guardar</Button>
              </div>
            </form>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
