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
  /** Interruptor por hilo del análisis de pedidos. */
  insight_enabled: boolean
  /** Relleno por el backend cuando el hilo ya tiene pedido consolidado. */
  order_intent?: InsightIntent
  order_needs_review?: boolean
  /** Hay un pedido del cliente mas nuevo que el vigente: la IA no lo pudo consolidar. */
  order_pending?: boolean
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
  client_id?: string
  send_error?: string
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

// ---------------------------------------------------------------------------
// Análisis de pedidos (IA local) + notas de voz
// ---------------------------------------------------------------------------

export type InsightIntent = 'pedido' | 'info' | 'reclamo' | 'otro'
export type OrderStatus = 'nuevo' | 'confirmado' | 'entregado' | 'descartado'
/**
 * Como hay que leer `cantidades`: 'delta' suma al pedido, 'total' lo reemplaza.
 * null = no se sabe, y el backend no toca el pedido en ese caso.
 */
export type TipoCantidad = 'delta' | 'total' | null

export type AnalysisStatus = 'pending' | 'processing' | 'ok' | 'error' | 'skipped'

/** Análisis de UN mensaje. Es lo que se pinta debajo de la burbuja. */
export interface MessageAnalysis {
  id: string
  message_id: string
  conversation_id: string
  /** 'message' = analisis de un mensaje, 'conversation' = resumen del hilo. */
  scope?: string
  intent?: InsightIntent | null
  resumen?: string | null
  productos: string[]
  cantidades: number[]
  tipo_cantidad?: TipoCantidad
  detalles: Record<string, string>
  confianza?: number | null
  needs_review: boolean
  asr_text?: string | null
  asr_ms?: number | null
  asr_model?: string | null
  model?: string | null
  latency_ms?: number | null
  status: AnalysisStatus
  error?: string | null
  skip_reason?: string | null
  created_at: string
}

/** Pedido consolidado de un hilo. Es lo que se edita a mano. */
export interface Order {
  conversation_id: string
  intent?: InsightIntent | null
  resumen?: string | null
  productos: string[]
  cantidades: number[]
  detalles: Record<string, string>
  confianza?: number | null
  needs_review: boolean
  source_message_id?: string | null
  model?: string | null
  edited: boolean
  id: string
  status: OrderStatus
  contact_id: string
  contact_name?: string | null
  contact_avatar?: string | null
  channel: string
  /** El cliente pidio algo que todavia no se consolido en este pedido. */
  pending: boolean
  unread_count: number
  last_inbound_at?: string | null
  created_at: string
  updated_at: string
  /**
   * Historial de revisiones. El pedido de un hilo es una cadena: la vigente
   * (is_current) es la que ve el operador, las anteriores quedan para
   * consultar. Ver POST /api/orders/:id/accept-pending.
   */
  revision: number
  is_current: boolean
  superseded_at?: string | null
  superseded_by?: string | null
}

/** Pedido sin datos de contacto: la versión que devuelve el PATCH. */
export interface OrderDraft {
  conversation_id: string
  intent?: InsightIntent | null
  resumen?: string | null
  productos: string[]
  cantidades: number[]
  detalles: Record<string, string>
  confianza?: number | null
  needs_review: boolean
  edited: boolean
}

export interface InsightChannelConfig {
  channel: string
  enabled: boolean
  asr_enabled: boolean
  model: string
  system_prompt?: string | null
  temperature: number
  context_messages: number
  updated_at: string
}

export interface InsightStatus {
  enabled: boolean
  master_enabled: boolean
  asr_enabled: boolean
  model: string
  model_ready: boolean
  model_error?: string
  asr_model: string
  channels: InsightChannelConfig[]
  counts: {
    total: number
    pending: number
    processing: number
    ok: number
    error: number
    skipped: number
    needs_review: number
  }
  queues: { text: number; audio: number }
  recent_errors: MessageAnalysis[]
}

export interface ConversationInsight {
  order: Order | null
  analyses: MessageAnalysis[]
  /** Historial del pedido, de la revision mas nueva a la mas vieja. Solo si hay mas de una. */
  revisions?: Order[]
  /** Analysis "pedido" que todavia no llego al pedido vigente. El banner de la tarjeta. */
  pending: MessageAnalysis | null
  enabled: boolean
  disabled_by?: string
}

export interface CatalogItem {
  id: string
  name: string
  category: string
  aliases: string[]
  active: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

export interface BackfillResult {
  encontrados: number
  encolados: number
  omitidos: number
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

  updateConversation(id: string, data: { status?: string; assigned_to?: string | null; insight_enabled?: boolean }) {
    return request<{ id: string; status: string; assigned_to: Assignee | null }>(`/inbox/conversations/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
  },

  sendMessage(conversationId: string, data: { message: string; account_id: string; attachment_url?: string; attachment_type?: string }) {
    return request<{ success: boolean; message_id: string; message?: Message }>(`/inbox/conversations/${conversationId}/messages`, {
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

  // Análisis de pedidos (IA local)
  insightStatus() {
    return request<InsightStatus>('/insights/status')
  },

  conversationInsight(conversationId: string) {
    return request<ConversationInsight>(`/insights/conversation/${conversationId}`)
  },

  messageAnalysis(messageId: string) {
    return request<MessageAnalysis | null>(`/insights/messages/${messageId}`)
  },

  /** El cuerpo es { enabled }, NO { value }. */
  setInsightMaster(enabled: boolean) {
    return request<{ key: string; enabled: boolean }>('/insights/settings/insight.master_enabled', {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    })
  },

  setInsightASR(enabled: boolean) {
    return request<{ key: string; enabled: boolean }>('/insights/settings/insight.asr_enabled', {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    })
  },

  updateInsightChannel(channel: string, data: Partial<InsightChannelConfig>) {
    return request<InsightChannelConfig>(`/insights/config/${channel}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
  },

  backfill(payload: { conversation_id?: string; limit?: number; force?: boolean } = {}) {
    return request<BackfillResult>('/insights/backfill', {
      method: 'POST',
      body: JSON.stringify({ limit: 200, force: true, ...payload }),
    })
  },

  // Pedidos
  listOrders(params: { status?: string; intent?: string; search?: string; limit?: number; offset?: number } = {}) {
    const query = new URLSearchParams()
    if (params.status) query.set('status', params.status)
    if (params.intent) query.set('intent', params.intent)
    if (params.search) query.set('search', params.search)
    query.set('limit', String(params.limit ?? 50))
    query.set('offset', String(params.offset ?? 0))
    return request<{ orders: Order[]; total: number; limit: number; offset: number }>(
      `/orders?${query.toString()}`,
    )
  },

  /** Editar a mano el pedido. Sella edited=true y la IA no lo vuelve a tocar. */
  updateOrder(id: string, data: Partial<Pick<Order, 'intent' | 'resumen' | 'productos' | 'cantidades' | 'detalles' | 'status' | 'needs_review'>>) {
    return request<Order>(`/orders/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
  },

  setOrderStatus(id: string, status: OrderStatus) {
    return request<{ status: OrderStatus }>(`/orders/${id}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    })
  },

  /**
   * Aplicar el pedido pendiente: el pedido nuevo del cliente pasa a ser la
   * revisión vigente y el anterior queda en el historial.
   *
   * POST y no PATCH porque no edita un recurso, ejecuta una transición. El
   * backend responde 409 si no hay nada pendiente.
   */
  acceptPendingOrder(id: string) {
    return request<Order>(`/orders/${id}/accept-pending`, { method: 'POST' })
  },

  /**
   * Reabrir el pedido para que la IA vuelva a consolidar sobre ESTA revisión,
   * sin crear una nueva. A diferencia de apply, no tira el pedido pendiente:
   * lo re-consolida encima con la misma MergeOrder del worker.
   */
  reopenOrder(id: string) {
    return request<Order>(`/orders/${id}/reopen`, { method: 'POST' })
  },

  // Catálogo
  listCatalog(all = false) {
    return request<{ items: CatalogItem[]; count: number }>(
      `/catalog${all ? '?all=true' : ''}`,
    )
  },

  createCatalogItem(data: { name: string; category?: string; aliases?: string[]; active?: boolean; sort_order?: number }) {
    return request<CatalogItem>('/catalog', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  updateCatalogItem(id: string, data: Partial<Pick<CatalogItem, 'name' | 'category' | 'aliases' | 'active' | 'sort_order'>>) {
    return request<CatalogItem>(`/catalog/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
  },

  deleteCatalogItem(id: string) {
    return request<{ ok: boolean }>(`/catalog/${id}`, { method: 'DELETE' })
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
