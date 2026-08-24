import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type SystemInfo } from '@/lib/api'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

export const useSettingsStore = defineStore('settings', () => {
  const ui = useUiStore()
  const auth = useAuthStore()

  const savingProfile = ref(false)
  const info = ref<SystemInfo | null>(null)
  const infoLoading = ref(false)
  const infoError = ref<string | null>(null)

  async function saveProfile(payload: {
    name?: string
    current_password?: string
    new_password?: string
  }): Promise<string | null> {
    savingProfile.value = true
    try {
      const res = await api.updateProfile(payload)
      if (auth.user) auth.user = res.user
      ui.success('Perfil actualizado')
      return null
    } catch (e: any) {
      return e?.message || 'No se pudo actualizar el perfil'
    } finally {
      savingProfile.value = false
    }
  }

  async function fetchInfo() {
    infoLoading.value = true
    infoError.value = null
    try {
      info.value = await api.systemInfo()
    } catch (e: any) {
      infoError.value = e?.message || 'No se pudo obtener la información del sistema'
    } finally {
      infoLoading.value = false
    }
  }

  return { savingProfile, info, infoLoading, infoError, saveProfile, fetchInfo }
})
