import { defineStore } from 'pinia'
import { ref, computed, onUnmounted } from 'vue'
import { api, type Conversation } from '@/lib/api'
import { useAuthStore } from '@/stores/auth'

export const useConversationsStore = defineStore('conversations', () => {
  const auth = useAuthStore()

  const conversations = ref<Conversation[]>([])
  const loading = ref(false)
  const loadingMore = ref(false)
  const total = ref(0)
  const error = ref<string | null>(null)
  const activeFilter = ref<{ channel?: string; status?: string }>({})
  const searchQuery = ref('')
  const showUnreadOnly = ref(false)
  const mineOnly = ref(false)
  let eventSource: EventSource | null = null

  const hasMore = computed(() => conversations.value.length < total.value)

  const filteredConversations = computed<Conversation[]>(() => {
    let list = conversations.value

    const { channel, status } = activeFilter.value
    if (channel) list = list.filter(c => c.channel === channel)
    if (status) list = list.filter(c => c.status === status)

    if (showUnreadOnly.value) list = list.filter(c => c.unread_count > 0)
    if (mineOnly.value) {
      const userId = auth.user?.id
      list = list.filter(c => !!userId && c.assigned_to?.id === userId)
    }

    const q = searchQuery.value.trim().toLowerCase()
    if (q) {
      list = list.filter(c => {
        const name = (c.contact?.name ?? '').toLowerCase()
        const phone = (c.contact?.phone ?? '').toLowerCase()
        const text = (c.last_message?.text ?? '').toLowerCase()
        return name.includes(q) || phone.includes(q) || text.includes(q)
      })
    }

    return list
  })

  async function fetchFirstPage(showLoading = false) {
    if (showLoading) loading.value = true
    error.value = null
    try {
      const res = await api.listConversations({ ...activeFilter.value, offset: 0 })
      conversations.value = res.data || []
      total.value = res.meta?.total ?? conversations.value.length
    } catch (e: any) {
      error.value = e.message
    } finally {
      if (showLoading) loading.value = false
    }
  }

  async function loadMore() {
    if (!hasMore.value || loadingMore.value || loading.value) return
    loadingMore.value = true
    try {
      const offset = conversations.value.length
      const res = await api.listConversations({ ...activeFilter.value, offset })
      const existingIds = new Set(conversations.value.map(c => c.id))
      for (const conv of res.data || []) {
        if (!existingIds.has(conv.id)) conversations.value.push(conv)
      }
      total.value = res.meta?.total ?? total.value
    } catch (e: any) {
      error.value = e.message
    } finally {
      loadingMore.value = false
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
        // New conversation, refetch first page
        fetchFirstPage()
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
    fetchFirstPage(true)
  }

  function setSearch(q: string) {
    searchQuery.value = q
  }

  function toggleUnreadOnly() {
    showUnreadOnly.value = !showUnreadOnly.value
  }

  function toggleMine() {
    mineOnly.value = !mineOnly.value
  }

  onUnmounted(() => unsubscribe())

  return {
    conversations,
    loading,
    loadingMore,
    total,
    hasMore,
    error,
    activeFilter,
    searchQuery,
    showUnreadOnly,
    mineOnly,
    filteredConversations,
    fetchFirstPage,
    fetchConversations: fetchFirstPage,
    loadMore,
    subscribe,
    unsubscribe,
    setFilter,
    setSearch,
    toggleUnreadOnly,
    toggleMine,
  }
})
