import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type StatsOverview } from '@/lib/api'

export type StatsPeriod = '24h' | '7d' | '30d'

export const useStatsStore = defineStore('stats', () => {
  const overview = ref<StatsOverview | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const currentPeriod = ref<StatsPeriod>('7d')

  async function fetchOverview(period: StatsPeriod = currentPeriod.value) {
    loading.value = true
    error.value = null
    try {
      currentPeriod.value = period
      overview.value = await api.statsOverview(period)
    } catch (e: any) {
      error.value = e?.message || 'No se pudieron cargar las estadísticas'
    } finally {
      loading.value = false
    }
  }

  return { overview, loading, error, currentPeriod, fetchOverview }
})
