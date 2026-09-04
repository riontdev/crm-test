import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, ApiError, type ConversationDetail, type Message } from '@/lib/api'

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
  const errorCode = ref<string | null>(null)
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
      if (data.conversation_id !== conversationId) return
      // Insertar el mensaje entrante localmente en tiempo real, sin recargar
      // toda la conversación. Dedup por external_id.
      if (!data.external_id) {
        fetchConversation(conversationId)
        return
      }
      if (messages.value.some((m) => m.external_id === data.external_id)) return
      const msg: Message = {
        id: data.message_id || data.external_id,
        external_id: data.external_id,
        direction: data.direction || 'incoming',
        text: data.text || undefined,
        attachments: data.attachments || [],
        sender_type: data.sender_type || 'contact',
        status: 'sent',
        platform_message_id: data.platform_message_id || undefined,
        sent_at: data.sent_at,
        created_at: data.sent_at,
      }
      messages.value.push(msg)
      if (conversation.value) {
        conversation.value.last_inbound_at = data.sent_at || conversation.value.last_inbound_at
        conversation.value.last_message = {
          text: data.text,
          direction: 'incoming',
          sent_at: data.sent_at,
        }
        conversation.value.unread_count = (conversation.value.unread_count || 0) + 1
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

    // Añadir una burbuja local pendiente para ver el estado del envío en tiempo real.
    const clientId = `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const pending: Message = {
      id: clientId,
      external_id: clientId,
      direction: 'outgoing',
      text: text || '📎',
      sender_type: 'agent',
      status: 'sending',
      sent_at: new Date().toISOString(),
      created_at: new Date().toISOString(),
      client_id: clientId,
    }
    if (attachmentUrl) {
      pending.attachments = [{ type: attachmentType || 'file', url: attachmentUrl }]
    }
    messages.value.push(pending)

    try {
      const res = await api.sendMessage(conversationId, {
        message: text,
        account_id: accountId,
        attachment_url: attachmentUrl,
        attachment_type: attachmentType,
      })
      lastFailed.value = null
      // Reemplazar la burbuja local por el mensaje persistido sin recargar toda
      // la conversación (más fluido y en tiempo real).
      const persisted = res.message
      const idx = messages.value.findIndex((m) => m.client_id === clientId)
      if (persisted) {
        const clean = { ...persisted }
        if (idx >= 0) messages.value[idx] = clean
        else messages.value.push(clean)
        if (conversation.value) {
          conversation.value.last_message = {
            text: persisted.text,
            direction: 'outgoing',
            sent_at: persisted.sent_at,
          }
        }
      } else if (idx >= 0) {
        messages.value[idx].status = 'sent'
      }
    } catch (e: any) {
      // Marcar la burbuja local como fallida y conservar el error objetivo.
      const msg = messages.value.find((m) => m.client_id === clientId)
      if (msg) {
        msg.status = 'failed'
        msg.send_error = e.message || 'No se pudo enviar el mensaje'
      }
      error.value = e.message || 'No se pudo enviar el mensaje'
      errorCode.value = e instanceof ApiError ? (e.code ?? null) : null
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
    errorCode.value = null
    lastFailed.value = null
  }

  return { conversation, messages, loading, sending, error, errorCode, lastFailed, fetchConversation, subscribe, unsubscribe, sendMessage, retrySend, $reset }
})
