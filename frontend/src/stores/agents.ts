import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type AgentConfig } from '@/lib/api'

export const useAgentsStore = defineStore('agents', () => {
  const agents = ref<AgentConfig[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchAgents() {
    loading.value = true
    error.value = null
    try {
      const res = await api.listAgents()
      agents.value = res.data || []
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  return { agents, loading, error, fetchAgents }
})
