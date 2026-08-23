import { defineStore } from 'pinia'
import { ref, onUnmounted } from 'vue'
import { api, type Conversation } from '@/lib/api'

export const useConversationsStore = defineStore('conversations', () => {
  const conversations = ref<Conversation[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const activeFilter = ref<{ channel?: string; status?: string }>({})
  let eventSource: EventSource | null = null

  async function fetchConversations(showLoading = false) {
    if (showLoading) loading.value = true
    error.value = null
    try {
      const res = await api.listConversations(activeFilter.value)
      conversations.value = res.data || []
    } catch (e: any) {
      error.value = e.message
    } finally {
      if (showLoading) loading.value = false
    }
  }

  function subscribe() {
    if (eventSource) return
    eventSource = new EventSource('/api/events')

    eventSource.addEventListener('message.received', (e) => {
      const data = JSON.parse(e.data)
      // Update the conversation in the list
      const idx = conversations.value.findIndex(c => c.id === data.conversation_id || c.zernio_conversation_id === data.conversation_id)
      if (idx >= 0) {
        conversations.value[idx].unread_count += 1
        conversations.value[idx].last_message = {
          text: data.text,
          direction: data.direction,
          sent_at: data.sent_at,
        }
        // Move to top
        const conv = conversations.value.splice(idx, 1)[0]
        conversations.value.unshift(conv)
      } else {
        // New conversation, refetch list
        fetchConversations()
      }
    })

    eventSource.onerror = () => {
      // EventSource auto-reconnects, but clean up if needed
      eventSource?.close()
      eventSource = null
      // Reconnect after 3s
      setTimeout(subscribe, 3000)
    }
  }

  function unsubscribe() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  function setFilter(channel?: string, status?: string) {
    activeFilter.value = { channel, status }
    fetchConversations(true)
  }

  onUnmounted(() => unsubscribe())

  return { conversations, loading, error, activeFilter, fetchConversations, subscribe, unsubscribe, setFilter }
})
