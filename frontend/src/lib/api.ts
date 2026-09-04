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

export interface Assignee {
  id: string
  name: string
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
  assigned_to?: Assignee | null
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

export interface Template {
  id: string
  name: string
  category: 'marketing' | 'utility' | 'soporte' | 'general'
  content: string
  language: string
  created_at: string
  updated_at: string
}

export interface SystemInfo {
  version: string
  database: string
  zernio_configured: boolean
  openrouter_configured: boolean
  webhook_path: string
}

export interface StatsOverview {
  period: '24h' | '7d' | '30d'
  totals: {
    messages: { count: number; delta_pct?: number | null }
    conversations: { active: number; new_in_period: number }
    unread: { total: number }
    ai_replies: { count: number; human_count: number }
    first_response: { avg_seconds?: number | null }
  }
  by_channel: Array<{ channel: string; messages: number; conversations: number }>
  daily_series: Array<{ date: string; incoming: number; outgoing: number }>
}

export interface ChannelStatus {
  channel: string
  connected: boolean
  conversations_count: number
  messages_count: number
  last_activity_at?: string | null
  agent_enabled: boolean
}

export interface ReportRow {
  date: string
  channel: string
  incoming: number
  outgoing: number
}

export interface ChannelTotals {
  channel: string
  incoming: number
  outgoing: number
  conversations: number
}

export interface ReportsData {
  from: string
  to: string
  daily: ReportRow[]
  totals_by_channel: ChannelTotals[]
  response_times: {
    avg_seconds?: number | null
    min_seconds?: number | null
    max_seconds?: number | null
  }
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
    throw ApiError.from(error, res.status)
  }

  return res.json()
}

export class ApiError extends Error {
  code?: string
  data: any
  status?: number

  constructor(message: string, code?: string, status?: number, data?: any) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.data = data
  }

  static from(payload: any, status: number): ApiError {
    const raw = payload?.error
    if (raw && typeof raw === 'object') {
      return new ApiError(raw.message || 'Solicitud inválida', raw.code, status, raw)
    }
    const msg =
      typeof raw === 'string'
        ? raw
        : payload?.message || `Solicitud inválida (${status})`
    return new ApiError(msg, raw?.code, status, payload)
  }
}

export const api = {
  // Auth
  login(email: string, password: string) {
    return request<{ user: User; session_expires_at?: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
  },

  me() {
    return request<{ user: User; session_expires_at?: string }>('/auth/me')
  },

  logoutApi() {
    return request<{ ok: boolean }>('/auth/logout', { method: 'POST' })
  },

  // Conversations
  listConversations(params?: { channel?: string; status?: string; offset?: number }) {
    const CONVERSATIONS_PAGE_LIMIT = 30
    const query = new URLSearchParams()
    if (params?.channel) query.set('channel', params.channel)
    if (params?.status) query.set('status', params.status)
    query.set('limit', String(CONVERSATIONS_PAGE_LIMIT))
    if (params?.offset !== undefined) query.set('offset', String(params.offset))
    const qs = query.toString()
    return request<{
      data: Conversation[]
      meta?: { total: number; limit: number; offset: number }
    }>(`/inbox/conversations${qs ? '?' + qs : ''}`)
  },

  getConversation(id: string) {
    return request<ConversationDetail>(`/inbox/conversations/${id}`)
  },

  updateConversation(id: string, data: { status?: string; assigned_to?: string | null }) {
    return request<{ id: string; status: string; assigned_to: Assignee | null }>(`/inbox/conversations/${id}`, {
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

  // Inbox: búsqueda global + no leídos
  searchConversations(q: string, limit = 10) {
    return request<{ data: Conversation[]; count: number }>(
      `/inbox/search?q=${encodeURIComponent(q)}&limit=${limit}`,
    )
  },

  unreadFeed(limit = 8) {
    return request<{
      data: Array<{
        id: string
        channel: string
        contact_name: string
        preview_text?: string | null
        last_inbound_at?: string | null
        unread_count: number
      }>
      total: number
    }>(`/inbox/unread?limit=${limit}`)
  },

  // Templates
  listTemplates(params?: { search?: string; category?: string }) {
    const query = new URLSearchParams()
    if (params?.search) query.set('search', params.search)
    if (params?.category) query.set('category', params.category)
    const qs = query.toString()
    return request<{ data: Template[]; count: number }>(`/templates${qs ? '?' + qs : ''}`)
  },

  createTemplate(data: Pick<Template, 'name' | 'category' | 'content'> & { language?: string }) {
    return request<{ data: Template }>('/templates', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  updateTemplate(id: string, data: Partial<Pick<Template, 'name' | 'category' | 'content' | 'language'>>) {
    return request<{ data: Template }>(`/templates/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },

  deleteTemplate(id: string) {
    return request<{ ok: boolean }>(`/templates/${id}`, { method: 'DELETE' })
  },

  // Profile & system
  updateProfile(data: { name?: string; current_password?: string; new_password?: string }) {
    return request<{ user: User }>('/auth/profile', {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
  },

  systemInfo() {
    return request<SystemInfo>('/system/info')
  },

  // Stats
  statsOverview(period: '24h' | '7d' | '30d' = '7d') {
    return request<StatsOverview>(`/stats/overview?period=${period}`)
  },

  reports(from: string, to: string) {
    const query = new URLSearchParams({ from, to })
    return request<ReportsData>(`/stats/reports?${query.toString()}`)
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

  // Channels
  channelsStatus() {
    return request<{ channels: ChannelStatus[]; webhook_url: string }>('/channels/status')
  },

  // WhatsApp WABA templates (aprobadas por Meta)
  whatsappTemplates(accountId: string) {
    return request<{
      templates: { name: string; language: string; status: string; category: string }[]
    }>(`/whatsapp/templates?account_id=${encodeURIComponent(accountId)}`)
  },

  createWhatsAppTemplate(data: {
    account_id: string
    name: string
    category: string
    language: string
    content: string
  }) {
    return request<{
      success: boolean
      template: { id: string; name: string; status: string; category: string; language: string }
    }>('/whatsapp/templates', { method: 'POST', body: JSON.stringify(data) })
  },
}
