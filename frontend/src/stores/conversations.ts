import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type Conversation } from '@/lib/api'

export const useConversationsStore = defineStore('conversations', () => {
  const conversations = ref<Conversation[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const activeFilter = ref<{ channel?: string; status?: string }>({})

  async function fetchConversations() {
    loading.value = true
    error.value = null
    try {
      const res = await api.listConversations(activeFilter.value)
      conversations.value = res.data || []
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  function setFilter(channel?: string, status?: string) {
    activeFilter.value = { channel, status }
    fetchConversations()
  }

  return { conversations, loading, error, activeFilter, fetchConversations, setFilter }
})
