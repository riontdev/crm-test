const API_BASE = '/api'

export interface User {
  id: string
  email: string
  name: string
  role: string
}

export interface Contact {
  id: string
  name?: string
  phone?: string
  email?: string
  avatar_url?: string
  company?: string
  tags?: string[]
  notes?: string
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
    if (res.status === 401 && !path.startsWith('/auth/login')) {
      window.dispatchEvent(new CustomEvent('crm:unauthorized'))
    }
    throw new Error(error.error || 'Request failed')
  }

  return res.json()
}

export const api = {
  // Auth
  login(email: string, password: string) {
    return request<{ user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
  },

  me() {
    return request<{ user: User }>('/auth/me')
  },

  logoutApi() {
    return request<{ ok: boolean }>('/auth/logout', { method: 'POST' })
  },

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

  updateConversation(id: string, data: { status?: string }) {
    return request<{ id: string; status: string }>(`/inbox/conversations/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
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

  updateContactNotes(id: string, notes: string) {
    return request<{ id: string; notes: string }>(`/inbox/contacts/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ notes }),
    })
  },

  // Agents
  listAgents() {
    return request<{ data: AgentConfig[] }>('/agents')
  },

  updateAgent(channel: string, data: Partial<Pick<AgentConfig, 'enabled' | 'model' | 'system_prompt' | 'temperature' | 'max_tokens'>>) {
    return request<AgentConfig>(`/agents/${channel}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
  },
}
