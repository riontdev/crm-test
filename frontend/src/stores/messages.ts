import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type ConversationDetail, type Message } from '@/lib/api'

export interface FailedPayload {
  text: string
  attachmentUrl?: string
  attachmentType?: string
}

export const useMessagesStore = defineStore('messages', () => {
  const conversation = ref<ConversationDetail | null>(null)
  const messages = ref<Message[]>([])
  const loading = ref(false)
  const sending = ref(false)
  const error = ref<string | null>(null)
  const lastFailed = ref<FailedPayload | null>(null)
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
      if (data.conversation_id === conversationId) {
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

  async function sendMessage(conversationId: string, text: string, accountId: string, attachmentUrl?: string, attachmentType?: string) {
    sending.value = true
    error.value = null
    try {
      await api.sendMessage(conversationId, {
        message: text,
        account_id: accountId,
        attachment_url: attachmentUrl,
        attachment_type: attachmentType,
      })
      lastFailed.value = null
      await fetchConversation(conversationId)
    } catch (e: any) {
      error.value = e.message || 'No se pudo enviar el mensaje'
      lastFailed.value = { text, attachmentUrl, attachmentType }
      throw e
    } finally {
      sending.value = false
    }
  }

  async function retrySend(conversationId: string, accountId: string) {
    if (!lastFailed.value) return
    const { text, attachmentUrl, attachmentType } = lastFailed.value
    await sendMessage(conversationId, text, accountId, attachmentUrl, attachmentType)
  }

  function $reset() {
    unsubscribe()
    conversation.value = null
    messages.value = []
    loading.value = false
    sending.value = false
    error.value = null
    lastFailed.value = null
  }

  return { conversation, messages, loading, sending, error, lastFailed, fetchConversation, subscribe, unsubscribe, sendMessage, retrySend, $reset }
})
