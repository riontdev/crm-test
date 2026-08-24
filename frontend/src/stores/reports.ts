import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type ReportsData } from '@/lib/api'

export const useReportsStore = defineStore('reports', () => {
  const data = ref<ReportsData | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const params = ref<{ from: string; to: string } | null>(null)

  async function fetchReports(from: string, to: string) {
    loading.value = true
    error.value = null
    try {
      params.value = { from, to }
      data.value = await api.reports(from, to)
    } catch (e: any) {
      error.value = e?.message || 'No se pudieron cargar los reportes'
    } finally {
      loading.value = false
    }
  }

  return { data, loading, error, params, fetchReports }
})
