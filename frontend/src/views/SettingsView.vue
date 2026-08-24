<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useSettingsStore } from '@/stores/settings'
import Avatar from '@/components/ui/Avatar.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Skeleton from '@/components/ui/Skeleton.vue'

const auth = useAuthStore()
const settings = useSettingsStore()

const nameField = ref(auth.user?.name ?? '')
const currentPassword = ref('')
const newPassword = ref('')
const formError = ref<string | null>(null)

const dirty = computed(() => {
  const nameChanged = nameField.value.trim() !== (auth.user?.name ?? '')
  const passwordsFilled =
    currentPassword.value.length > 0 && newPassword.value.length > 0
  return nameChanged || passwordsFilled
})

onMounted(() => {
  if (auth.isAdmin && !settings.info) settings.fetchInfo()
})

async function save() {
  formError.value = null
  const payload: { name?: string; current_password?: string; new_password?: string } = {}

  if (nameField.value.trim() !== (auth.user?.name ?? '')) {
    payload.name = nameField.value.trim()
  }
  if (currentPassword.value || newPassword.value) {
    if (!currentPassword.value || !newPassword.value) {
      formError.value = 'Completá la contraseña actual y la nueva para cambiarla'
      return
    }
    payload.current_password = currentPassword.value
    payload.new_password = newPassword.value
  }

  const err = await settings.saveProfile(payload)
  if (err) {
    formError.value = err
    return
  }
  currentPassword.value = ''
  newPassword.value = ''
}

function fmtBool(ok: boolean, okLabel: string, badLabel: string): string {
  return ok ? okLabel : badLabel
}
</script>

<template>
  <div class="h-full overflow-y-auto px-6 py-6">
    <div class="mx-auto max-w-3xl space-y-6">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-slate-900 dark:text-white">Configuración</h1>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">Tu cuenta y el estado del sistema.</p>
      </div>

      <!-- Perfil -->
      <section class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-[#101828]">
        <div class="flex items-center gap-4">
          <Avatar size="lg" :name="auth.user?.name || 'Operador'" />
          <div class="min-w-0">
            <p class="truncate text-base font-semibold text-slate-800 dark:text-slate-100">
              {{ auth.user?.name }}
            </p>
            <p class="text-xs text-slate-500 dark:text-slate-400">{{ auth.user?.email }}</p>
          </div>
          <Badge :variant="auth.isAdmin ? 'accent' : 'default'" size="sm" class="ml-auto">
            {{ auth.isAdmin ? 'Admin' : 'Agente' }}
          </Badge>
        </div>

        <form class="mt-6 space-y-4" @submit.prevent="save">
          <div>
            <label for="settings-name" class="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">Nombre</label>
            <input
              id="settings-name"
              v-model="nameField"
              type="text"
              class="w-full rounded-lg border border-slate-300 bg-slate-50 px-4 py-2.5 text-sm text-slate-800 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
            />
          </div>

          <div class="pt-2">
            <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">Cambiar contraseña</p>
            <div class="mt-3 grid gap-4 sm:grid-cols-2">
              <div>
                <label for="settings-current-pass" class="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">Contraseña actual</label>
                <input
                  id="settings-current-pass"
                  v-model="currentPassword"
                  type="password"
                  autocomplete="current-password"
                  placeholder="••••••••"
                  class="w-full rounded-lg border border-slate-300 bg-slate-50 px-4 py-2.5 text-sm text-slate-800 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                />
              </div>
              <div>
                <label for="settings-new-pass" class="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">Nueva contraseña</label>
                <input
                  id="settings-new-pass"
                  v-model="newPassword"
                  type="password"
                  autocomplete="new-password"
                  placeholder="Mínimo 6 caracteres"
                  class="w-full rounded-lg border border-slate-300 bg-slate-50 px-4 py-2.5 text-sm text-slate-800 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
                />
              </div>
            </div>
          </div>

          <p v-if="formError" role="alert" class="text-xs text-red-500">{{ formError }}</p>

          <div class="flex justify-end pt-1">
            <Button type="submit" variant="primary" :disabled="!dirty" :loading="settings.savingProfile">
              Guardar cambios
            </Button>
          </div>
        </form>
      </section>

      <!-- Estado del sistema (solo admin) -->
      <section
        v-if="auth.isAdmin"
        class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
      >
        <h2 class="text-base font-semibold text-slate-800 dark:text-slate-100">Estado del sistema</h2>

        <div v-if="settings.infoLoading" class="mt-4 space-y-2">
          <Skeleton :lines="4" />
        </div>

        <dl v-else-if="settings.info" class="mt-4 grid gap-3 sm:grid-cols-2">
          <div class="rounded-xl bg-slate-50 p-3.5 dark:bg-slate-800/60">
            <dt class="text-[10px] font-semibold uppercase tracking-wider text-slate-400">Versión</dt>
            <dd class="mt-1 font-mono text-sm font-medium text-slate-700 dark:text-slate-200">{{ settings.info.version }}</dd>
          </div>
          <div class="rounded-xl bg-slate-50 p-3.5 dark:bg-slate-800/60">
            <dt class="text-[10px] font-semibold uppercase tracking-wider text-slate-400">Base de datos</dt>
            <dd class="mt-1">
              <Badge :variant="settings.info.database === 'ok' ? 'success' : 'danger'" size="sm">
                {{ fmtBool(settings.info.database === 'ok', 'Conectada', 'Sin conexión') }}
              </Badge>
            </dd>
          </div>
          <div class="rounded-xl bg-slate-50 p-3.5 dark:bg-slate-800/60">
            <dt class="text-[10px] font-semibold uppercase tracking-wider text-slate-400">Zernio</dt>
            <dd class="mt-1">
              <Badge :variant="settings.info.zernio_configured ? 'success' : 'danger'" size="sm">
                {{ fmtBool(settings.info.zernio_configured, 'Configurado', 'No configurado') }}
              </Badge>
            </dd>
          </div>
          <div class="rounded-xl bg-slate-50 p-3.5 dark:bg-slate-800/60">
            <dt class="text-[10px] font-semibold uppercase tracking-wider text-slate-400">OpenRouter</dt>
            <dd class="mt-1">
              <Badge :variant="settings.info.openrouter_configured ? 'accent' : 'warning'" size="sm">
                {{ fmtBool(settings.info.openrouter_configured, 'Configurado', 'Pendiente de clave') }}
              </Badge>
            </dd>
            <p v-if="!settings.info.openrouter_configured" class="mt-1.5 text-[11px] text-slate-400 dark:text-slate-500">
              Cargá OPENROUTER_API_KEY en Railway para habilitar los agentes IA.
            </p>
          </div>
          <div class="rounded-xl bg-slate-50 p-3.5 sm:col-span-2 dark:bg-slate-800/60">
            <dt class="text-[10px] font-semibold uppercase tracking-wider text-slate-400">Webhook</dt>
            <dd class="mt-1">
              <code class="rounded-md bg-slate-100 px-2 py-1 font-mono text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{{
                settings.info.webhook_path
              }}</code>
            </dd>
          </div>
        </dl>

        <div v-else-if="settings.infoError" class="mt-4 flex items-center justify-between">
          <p class="text-xs text-red-500">{{ settings.infoError }}</p>
          <Button variant="secondary" size="sm" @click="settings.fetchInfo()">Reintentar</Button>
        </div>
      </section>
    </div>
  </div>
</template>
