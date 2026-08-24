import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type Template } from '@/lib/api'
import { useUiStore } from '@/stores/ui'

export const useTemplatesStore = defineStore('templates', () => {
  const ui = useUiStore()

  const templates = ref<Template[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const loadedOnce = ref(false)
  const savingIds = ref<Set<string>>(new Set())

  async function fetchTemplates(search?: string, category?: string) {
    loading.value = true
    error.value = null
    try {
      const res = await api.listTemplates({ search, category })
      templates.value = res.data || []
      loadedOnce.value = true
    } catch (e: any) {
      error.value = e?.message || 'No se pudieron cargar las plantillas'
    } finally {
      loading.value = false
    }
  }

  async function createTemplate(payload: { name: string; category: Template['category']; content: string; language?: string }): Promise<string | null> {
    try {
      const res = await api.createTemplate(payload)
      templates.value = [res.data, ...templates.value]
      ui.success('Plantilla creada')
      return null
    } catch (e: any) {
      return e?.message || 'No se pudo crear la plantilla'
    }
  }

  async function updateTemplate(id: string, patch: Partial<Pick<Template, 'name' | 'category' | 'content' | 'language'>>): Promise<string | null> {
    const snapshot = templates.value
    const idx = templates.value.findIndex((t) => t.id === id)
    if (idx >= 0) {
      templates.value[idx] = { ...templates.value[idx], ...patch }
    }
    savingIds.value.add(id)
    try {
      const res = await api.updateTemplate(id, patch)
      if (idx >= 0) templates.value[idx] = res.data
      ui.success('Plantilla actualizada')
      return null
    } catch (e: any) {
      templates.value = snapshot
      ui.error(e?.message || 'No se pudo actualizar la plantilla')
      return e?.message || 'error'
    } finally {
      savingIds.value.delete(id)
    }
  }

  async function deleteTemplate(id: string): Promise<string | null> {
    const snapshot = templates.value
    templates.value = templates.value.filter((t) => t.id !== id)
    try {
      await api.deleteTemplate(id)
      ui.success('Plantilla eliminada')
      return null
    } catch (e: any) {
      templates.value = snapshot
      ui.error(e?.message || 'No se pudo eliminar la plantilla')
      return e?.message || 'error'
    }
  }

  return { templates, loading, error, loadedOnce, savingIds, fetchTemplates, createTemplate, updateTemplate, deleteTemplate }
})
