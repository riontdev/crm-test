import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import router from '@/router'
import { api, type User } from '@/lib/api'
import { useUiStore } from '@/stores/ui'

export const useAuthStore = defineStore('auth', () => {
  const ui = useUiStore()

  const user = ref<User | null>(null)
  const initialized = ref(false)
  const loading = ref(false)

  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function init() {
    if (initialized.value) return
    try {
      const res = await api.me()
      user.value = res.user
    } catch {
      user.value = null
    } finally {
      initialized.value = true
    }
  }

  async function login(email: string, password: string): Promise<string | null> {
    loading.value = true
    try {
      const res = await api.login(email, password)
      user.value = res.user
      ui.success(`Bienvenido, ${res.user.name}`)
      return null
    } catch (e: any) {
      const message = e?.message || 'No se pudo iniciar sesión'
      ui.error(message)
      return message
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await api.logoutApi()
    } catch {
      // La sesión local se limpia igualmente
    }
    user.value = null
    ui.info('Sesión cerrada')
    router.push('/login')
  }

  function handleUnauthorized() {
    if (!user.value) return
    user.value = null
    ui.error('Tu sesión expiró')
    router.push('/login')
  }

  window.addEventListener('crm:unauthorized', handleUnauthorized)

  return { user, initialized, loading, isAuthenticated, isAdmin, init, login, logout }
})
