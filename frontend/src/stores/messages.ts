import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type ConversationDetail, type Message } from '@/lib/api'

export const useMessagesStore = defineStore('messages', () => {
  const conversation = ref<ConversationDetail | null>(null)
  const messages = ref<Message[]>([])
  const loading = ref(false)
  const sending = ref(false)
  const error = ref<string | null>(null)
  let eventSource: EventSource | null = null

  async function fetchConversation(id: string) {
    loading.value = true
    error.value = null
    try {
      const res = await api.getConversation(id)
      conversation.value = res
      messages.value = res.messages || []
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  function subscribe(conversationId: string) {
    unsubscribe()
    eventSource = new EventSource(`/api/events?conversation_id=${conversationId}`)

    eventSource.addEventListener('message.received', (e) => {
      const data = JSON.parse(e.data)
      // Only update if it's for this conversation
      if (data.conversation_id === conversationId) {
        // Refetch to get full message data with attachments
        fetchConversation(conversationId)
      }
    })

    eventSource.onerror = () => {
      eventSource?.close()
      eventSource = null
      setTimeout(() => subscribe(conversationId), 3000)
    }
  }

  function unsubscribe() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  async function sendMessage(conversationId: string, text: string, accountId: string) {
    sending.value = true
    error.value = null
    try {
      await api.sendMessage(conversationId, { message: text, account_id: accountId })
      await fetchConversation(conversationId)
    } catch (e: any) {
      error.value = e.message
      throw e
    } finally {
      sending.value = false
    }
  }

  function $reset() {
    unsubscribe()
    conversation.value = null
    messages.value = []
    loading.value = false
    sending.value = false
    error.value = null
  }

  return { conversation, messages, loading, sending, error, fetchConversation, subscribe, unsubscribe, sendMessage, $reset }
})
