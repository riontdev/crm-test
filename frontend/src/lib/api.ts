const API_BASE = '/api'

export interface Contact {
  id: string
  name?: string
  phone?: string
  email?: string
  avatar_url?: string
}

export interface Conversation {
  id: string
  channel: string
  provider: string
  zernio_conversation_id: string
  zernio_account_id?: string
  status: string
  last_inbound_at?: string
  unread_count: number
  created_at: string
  updated_at: string
  contact?: Contact
  last_message?: {
    text?: string
    direction: string
    sent_at?: string
  }
}

export interface Message {
  id: string
  external_id: string
  direction: string
  text?: string
  attachments?: Array<{ type: string; url: string }>
  sender_type: string
  status: string
  platform_message_id?: string
  sent_at?: string
  created_at: string
}

export interface ConversationDetail extends Conversation {
  messages: Message[]
}

export interface AgentConfig {
  id: string
  channel: string
  enabled: boolean
  model: string
  system_prompt?: string
  temperature: number
  max_tokens: number
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(error.error || 'Request failed')
  }

  return res.json()
}

export const api = {
  // Conversations
  listConversations(params?: { channel?: string; status?: string }) {
    const query = new URLSearchParams()
    if (params?.channel) query.set('channel', params.channel)
    if (params?.status) query.set('status', params.status)
    const qs = query.toString()
    return request<{ data: Conversation[]; count: number }>(`/inbox/conversations${qs ? '?' + qs : ''}`)
  },

  getConversation(id: string) {
    return request<ConversationDetail>(`/inbox/conversations/${id}`)
  },

  sendMessage(conversationId: string, data: { message: string; account_id: string; attachment_url?: string; attachment_type?: string }) {
    return request<{ success: boolean; message_id: string }>(`/inbox/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  uploadFile(file: File) {
    const formData = new FormData()
    formData.append('file', file)
    return fetch(`${API_BASE}/upload`, {
      method: 'POST',
      body: formData,
    }).then(res => res.json())
  },

  // Agents
  listAgents() {
    return request<{ data: AgentConfig[] }>('/agents')
  },
}
