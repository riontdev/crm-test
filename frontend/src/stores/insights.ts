import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  api,
  type CatalogItem,
  type ConversationInsight,
  type InsightStatus,
  type MessageAnalysis,
  type Order,
  type OrderStatus,
} from '@/lib/api'
import { useUiStore } from '@/stores/ui'

/**
 * Estado del analizador de pedidos.
 *
 * Regla de oro del módulo: la UI es de SOLO LECTURA sobre lo que dice el
 * modelo. Lo unico que se escribe es la correccion del operador, y esa
 * correction sella edited=true para que la IA no la pise mas.
 *
 * El store escucha SSE por su cuenta. El backend emite `insight.analysis` al
 * terminar cada mensaje y `insight.transcript` cuando una nota de voz ya fue
 * transcrita, asi que la pantalla se actualiza sola sin que el operador
 * recargue nada.
 */
export const useInsightsStore = defineStore('insights', () => {
  const ui = useUiStore()

  const status = ref<InsightStatus | null>(null)
  const statusLoading = ref(false)

  // Un insight por conversación abierta. No se cachean todos: la vista de
  // Pedidos tiene su propia carga paginada.
  const byConversation = ref<Record<string, ConversationInsight>>({})
  const loadingConversation = ref<Set<string>>(new Set())

  const analysisByMessage = ref<Record<string, MessageAnalysis>>({})

  const orders = ref<Order[]>([])
  const ordersTotal = ref(0)
  const ordersLoading = ref(false)

  // Hilos con una transicion de pedido en curso (aplicar/reabrir). Es un Set
  // porque la accion es por hilo y dos hilos pueden estar transitioning a la
  // vez: con un solo booleano, aplicar en uno deshabilita el otro.
  const orderPendingIds = ref<Set<string>>(new Set())
  const catalog = ref<CatalogItem[]>([])
  const catalogLoading = ref(false)

  let eventSource: EventSource | null = null

  /** Ultima consulta de /orders, para refrescarla sin perder filtros. */
  let lastOrdersQuery: Parameters<typeof api.listOrders>[0] = {}

  const enabled = computed(() => status.value?.enabled ?? false)
  const modelReady = computed(() => status.value?.model_ready ?? false)
  const counts = computed(() => status.value?.counts)

  /** El modelo anda lento o no esta: la UI tiene que decirlo, no mentir. */
  const health = computed<'ok' | 'degraded' | 'off'>(() => {
    if (!status.value) return 'off'
    if (!status.value.enabled || !status.value.master_enabled) return 'off'
    if (!status.value.model_ready) return 'degraded'
    if ((counts.value?.error ?? 0) > 0) return 'degraded'
    return 'ok'
  })

  const queueBusy = computed(() => {
    const q = status.value?.queues
    return (q?.text ?? 0) + (q?.audio ?? 0)
  })

  function setLoading(id: string, on: boolean) {
    const next = new Set(loadingConversation.value)
    if (on) next.add(id)
    else next.delete(id)
    loadingConversation.value = next
  }

  async function fetchStatus() {
    statusLoading.value = true
    try {
      status.value = await api.insightStatus()
    } catch (e: any) {
      // Si el backend tiene INSIGHT_ENABLED=false las rutas ni siquiera
      // existen. No es un error que haya que mostrar: la UI simplemente
      // muestra el módulo apagado.
      status.value = null
    } finally {
      statusLoading.value = false
    }
  }

  async function fetchConversation(conversationId: string, force = false) {
    if (!force && byConversation.value[conversationId]) return
    setLoading(conversationId, true)
    try {
      const data = await api.conversationInsight(conversationId)
      byConversation.value = { ...byConversation.value, [conversationId]: data }
      // Indexar por mensaje: las burbujas de audio necesitan el asr_text y el
      // resumen, y pedirlos uno por uno seria una request por mensaje.
      const index = { ...analysisByMessage.value }
      for (const a of data.analyses || []) index[a.message_id] = a
      analysisByMessage.value = index
    } catch (e: any) {
      errorOf(e, 'No se pudo leer el análisis de este hilo')
    } finally {
      setLoading(conversationId, false)
    }
  }

  function errorOf(e: any, fallback: string) {
    ui.error(e?.message || fallback)
  }

  function clearConversation(conversationId: string) {
    const next = { ...byConversation.value }
    delete next[conversationId]
    byConversation.value = next
  }

  /**
   * El analysis que todavia no llego al pedido vigente, o null. Lo calcula el
   * backend (pending) y no el componente: el criterio es el mismo que usa el
   * badge del inbox, y si cada lado lo calculara distinto el badge y la tarjeta
   * terminan discrepando.
   */
  function pendingFor(conversationId: string): MessageAnalysis | null {
    return byConversation.value[conversationId]?.pending ?? null
  }

  /** Historial de revisiones del pedido, de la mas nueva a la mas vieja. */
  function revisionsFor(conversationId: string): Order[] {
    return byConversation.value[conversationId]?.revisions ?? []
  }

  function orderFor(conversationId: string): Order | null {
    return byConversation.value[conversationId]?.order ?? null
  }

  function analysisFor(messageId: string): MessageAnalysis | null {
    return analysisByMessage.value[messageId] ?? null
  }

  function analysisForConversation(conversationId: string): MessageAnalysis[] {
    return byConversation.value[conversationId]?.analyses ?? []
  }

  /**
   * Corrección del operador. Optimista con rollback: el PATCH sella
   * edited=true y la IA deja de tocar el pedido, asi que lo que se ve en
   * pantalla tiene que cambiar ya, no en 300ms.
   */
  async function saveOrder(
    orderId: string,
    conversationId: string,
    patch: Partial<Pick<Order, 'intent' | 'resumen' | 'productos' | 'cantidades' | 'detalles' | 'status' | 'needs_review'>>,
  ): Promise<boolean> {
    const prev = byConversation.value[conversationId]?.order
    if (prev) {
      byConversation.value = {
        ...byConversation.value,
        [conversationId]: { ...byConversation.value[conversationId], order: { ...prev, ...patch } as Order },
      }
    }
    try {
      const saved = await api.updateOrder(orderId, patch)
      const cur = byConversation.value[conversationId]
      if (cur) {
        byConversation.value = {
          ...byConversation.value,
          [conversationId]: { ...cur, order: { ...cur.order, ...saved } },
        }
      }
      ui.success('Pedido guardado. La IA ya no lo va a modificar.')
      void fetchStatus()
      return true
    } catch (e: any) {
      if (prev && byConversation.value[conversationId]) {
        byConversation.value = {
          ...byConversation.value,
          [conversationId]: { ...byConversation.value[conversationId], order: prev },
        }
      }
      errorOf(e, 'No se pudo guardar el pedido')
      return false
    }
  }

  async function setOrderStatus(orderId: string, conversationId: string, statusValue: OrderStatus) {
    try {
      await api.setOrderStatus(orderId, statusValue)
      const cur = byConversation.value[conversationId]
      if (cur?.order) {
        byConversation.value = {
          ...byConversation.value,
          [conversationId]: { ...cur, order: { ...cur.order, status: statusValue } },
        }
      }
      // El status no sella edited, asi que refresco la lista global por si
      // el pedido estaba en otra vista.
      void fetchOrders()
    } catch (e: any) {
      errorOf(e, 'No se pudo cambiar el estado')
    }
  }

  /**
   * Aplicar el pedido pendiente: el pedido nuevo del cliente pasa a ser la
   * revision vigente.
   *
   * No es optimista a proposito: crea una fila nueva en la DB y el historial
   * cambia de forma que no se puede adivinar en el cliente. Refetch del hilo y
   * un toast; el operator hizo una accion de estado, no de edicion de texto.
   */
  async function acceptPending(conversationId: string): Promise<boolean> {
    const order = byConversation.value[conversationId]?.order
    if (!order) return false
    orderPendingIds.value = new Set([...orderPendingIds.value, conversationId])
    try {
      await api.acceptPendingOrder(order.id)
      await fetchConversation(conversationId, true)
      void fetchOrders()
      void fetchStatus()
      ui.success('Pedido aplicado. El anterior quedó en el historial.')
      return true
    } catch (e: any) {
      errorOf(e, 'No se pudo aplicar el pedido pendiente')
      return false
    } finally {
      const next = new Set(orderPendingIds.value)
      next.delete(conversationId)
      orderPendingIds.value = next
    }
  }

  /**
   * Reabrir el pedido para que la IA vuelva a consolidar sobre esta revision.
   * Resuelve el pendiente sin crear revision, pero el contenido igual cambio,
   * asi que tampoco es optimista: refetch del hilo.
   */
  async function reopenOrder(conversationId: string): Promise<boolean> {
    const order = byConversation.value[conversationId]?.order
    if (!order) return false
    orderPendingIds.value = new Set([...orderPendingIds.value, conversationId])
    try {
      await api.reopenOrder(order.id)
      await fetchConversation(conversationId, true)
      void fetchOrders()
      void fetchStatus()
      ui.success('Pedido reabierto. La IA lo vuelve a actualizar.')
      return true
    } catch (e: any) {
      errorOf(e, 'No se pudo reabrir el pedido')
      return false
    } finally {
      const next = new Set(orderPendingIds.value)
      next.delete(conversationId)
      orderPendingIds.value = next
    }
  }

  async function fetchOrders(params: { status?: string; intent?: string; search?: string; limit?: number; offset?: number } = {}) {
    ordersLoading.value = true
    lastOrdersQuery = params
    try {
      const res = await api.listOrders(params)
      orders.value = res.orders || []
      ordersTotal.value = res.total || 0
    } catch (e: any) {
      errorOf(e, 'No se pudieron cargar los pedidos')
    } finally {
      ordersLoading.value = false
    }
  }

  async function fetchCatalog(all = false) {
    catalogLoading.value = true
    try {
      const res = await api.listCatalog(all)
      catalog.value = res.items || []
    } catch (e: any) {
      errorOf(e, 'No se pudo cargar el catálogo')
    } finally {
      catalogLoading.value = false
    }
  }

  async function createCatalogItem(payload: { name: string; category?: string; aliases?: string[]; active?: boolean; sort_order?: number }): Promise<boolean> {
    try {
      const item = await api.createCatalogItem(payload)
      catalog.value = [...catalog.value, item]
      ui.success('Producto agregado al catálogo')
      return true
    } catch (e: any) {
      errorOf(e, 'No se pudo agregar el producto')
      return false
    }
  }

  async function updateCatalogItem(id: string, patch: Partial<Pick<CatalogItem, 'name' | 'category' | 'aliases' | 'active' | 'sort_order'>>) {
    const snapshot = catalog.value
    catalog.value = catalog.value.map((c) => (c.id === id ? { ...c, ...patch } : c))
    try {
      const saved = await api.updateCatalogItem(id, patch)
      catalog.value = catalog.value.map((c) => (c.id === id ? saved : c))
    } catch (e: any) {
      catalog.value = snapshot
      errorOf(e, 'No se pudo actualizar el producto')
    }
  }

  async function deleteCatalogItem(id: string) {
    const snapshot = catalog.value
    catalog.value = catalog.value.filter((c) => c.id !== id)
    try {
      await api.deleteCatalogItem(id)
      ui.success('Producto eliminado')
    } catch (e: any) {
      catalog.value = snapshot
      errorOf(e, 'No se pudo eliminar el producto')
    }
  }

  async function setMasterEnabled(on: boolean) {
    const prev = status.value
    if (prev) status.value = { ...prev, master_enabled: on, enabled: on }
    try {
      await api.setInsightMaster(on)
      ui.success(on ? 'Análisis activado' : 'Análisis pausado')
      void fetchStatus()
    } catch (e: any) {
      status.value = prev
      errorOf(e, 'No se pudo cambiar el interruptor')
    }
  }

  async function setASREnabled(on: boolean) {
    const prev = status.value
    if (prev) status.value = { ...prev, asr_enabled: on }
    try {
      await api.setInsightASR(on)
      ui.success(on ? 'Notas de voz activadas' : 'Notas de voz pausadas')
      void fetchStatus()
    } catch (e: any) {
      status.value = prev
      errorOf(e, 'No se pudo cambiar el interruptor')
    }
  }

  async function setChannelEnabled(channel: string, on: boolean) {
    try {
      const saved = await api.updateInsightChannel(channel, { enabled: on })
      if (status.value) {
        status.value = {
          ...status.value,
          channels: status.value.channels.map((c) => (c.channel === channel ? saved : c)),
        }
      }
    } catch (e: any) {
      errorOf(e, 'No se pudo cambiar el canal')
    }
  }

  async function backfill(conversationId?: string) {
    try {
      const res = await api.backfill({ conversation_id: conversationId })
      ui.success(`Encolados ${res.encolados} de ${res.encontrados} mensajes`)
      return res
    } catch (e: any) {
      errorOf(e, 'No se pudo lanzar el reproceso')
      return null
    }
  }

  /**
   * Interruptor por hilo. Vive acá y no en el store de conversaciones para no
   * acoplar los dos: la lista de la bandeja mantiene su propio objeto
   * Conversation, pero el panel de detalle necesita saber el estado al vuelo.
   */
  const conversationEnabled = ref<Record<string, boolean>>({})

  function isEnabledFor(conversation: { id: string; insight_enabled?: boolean } | null | undefined) {
    if (!conversation) return false
    if (!enabled.value) return false
    const override = conversationEnabled.value[conversation.id]
    if (override !== undefined) return override
    return conversation.insight_enabled !== false
  }

  async function setConversationEnabled(conversationId: string, on: boolean) {
    const prev = conversationEnabled.value[conversationId]
    conversationEnabled.value = { ...conversationEnabled.value, [conversationId]: on }
    try {
      await api.updateConversation(conversationId, { insight_enabled: on })
      // El flag vive en el objeto Conversation de la bandeja; recargar la
      // insight deja de ser necesario porque la vista ya refleja el estado.
      void fetchConversation(conversationId, true)
    } catch (e: any) {
      const next = { ...conversationEnabled.value }
      if (prev === undefined) delete next[conversationId]
      else next[conversationId] = prev
      conversationEnabled.value = next
      errorOf(e, 'No se pudo cambiar el interruptor del hilo')
    }
  }

  /**
   * SSE.
   *
   * El backend emite tres eventos y cada uno pide una cosa distinta:
   *
   * - `insight.transcript`: Whisper termino. Trae el texto, asi que se pinta
   *   al instante sin refetch. Es el camino que NO puede esperar: si Ollama se
   *   cae, el operador igual tiene que poder leer lo que dijo el cliente.
   * - `insight.updated`: la IA的分析 esta listo. El payload trae el pedido
   *   fusionado pero parcial (sin id, ni edited, ni status), asi que se
   *   refresca el hilo abierto en vez de parquear un objeto incompleto.
   * - `insight.health`: Ollama termino de cargar el modelo.
   *
   * Solo se refresca lo que esta en pantalla. Un evento de un hilo que nadie
   * esta mirando no dispara ninguna request.
   */
  function subscribe() {
    if (eventSource) return
    eventSource = new EventSource('/api/events')

    eventSource.addEventListener('insight.transcript', (e) => {
      try {
        const data = JSON.parse(e.data)
        if (!data.message_id) return
        const index = { ...analysisByMessage.value }
        const prev = index[data.message_id]
        index[data.message_id] = {
          id: prev?.id ?? '',
          message_id: data.message_id,
          conversation_id: data.conversation_id,
          intent: prev?.intent ?? null,
          resumen: prev?.resumen ?? null,
          productos: prev?.productos ?? [],
          cantidades: prev?.cantidades ?? [],
          detalles: prev?.detalles ?? {},
          confianza: prev?.confianza ?? null,
          needs_review: prev?.needs_review ?? false,
          asr_text: data.text,
          asr_ms: data.asr_ms ?? null,
          asr_model: data.asr_model ?? null,
          model: prev?.model ?? null,
          latency_ms: prev?.latency_ms ?? null,
          // Todavia no esta el resumen de la IA: se ve la transcripcion y la
          // burbuja queda en analyzing hasta que llegue insight.updated.
          status: prev?.status === 'ok' ? 'ok' : 'processing',
          error: null,
          skip_reason: null,
          created_at: prev?.created_at ?? new Date().toISOString(),
        }
        analysisByMessage.value = index
        void refreshIfVisible(data.conversation_id)
      } catch {
        // Evento mal formado: se ignora. Romper la pantalla por un evento de
        // tiempo real no vale la pena.
      }
    })

    eventSource.addEventListener('insight.updated', (e) => {
      try {
        const data = JSON.parse(e.data)
        if (data.message_id && data.analysis) {
          const index = { ...analysisByMessage.value }
          const prev = index[data.message_id]
          index[data.message_id] = {
            ...(prev ?? {
              id: '',
              message_id: data.message_id,
              conversation_id: data.conversation_id,
              productos: [],
              cantidades: [],
              detalles: {},
              needs_review: false,
              created_at: new Date().toISOString(),
            }),
            ...data.analysis,
            // El texto transcrito viaja en el evento, no dentro de analysis.
            asr_text: data.asr_text ?? prev?.asr_text ?? null,
            // Este evento solo se emite cuando la IA termino bien. Los
            // errores van por otro camino, asi que no llega status.
            status: 'ok',
          } as MessageAnalysis
          analysisByMessage.value = index
        }
        void refreshIfVisible(data.conversation_id)
      } catch {
        // idem
      }
    })

    eventSource.addEventListener('insight.health', (e) => {
      try {
        const data = JSON.parse(e.data)
        if (status.value && typeof data.ready === 'boolean') {
          status.value = { ...status.value, model_ready: data.ready }
        }
      } catch {
        // idem
      }
    })

    eventSource.onerror = () => {
      eventSource?.close()
      eventSource = null
      setTimeout(subscribe, 3000)
    }
  }

  /**
   * Refresca solo lo que el operador tiene abierto. Si la lista de pedidos esta
   * cargada se refresca con los filtros actuales; si no, el evento se ignora y
   * se vera cuando el usuario abra la vista.
   */
  function refreshIfVisible(conversationId?: string) {
    if (conversationId && byConversation.value[conversationId]) {
      void fetchConversation(conversationId, true)
    }
    if (orders.value.length) void fetchOrders(lastOrdersQuery)
    void fetchStatus()
  }

  function unsubscribe() {
    eventSource?.close()
    eventSource = null
  }

  return {
    status,
    statusLoading,
    enabled,
    modelReady,
    counts,
    health,
    queueBusy,
    byConversation,
    loadingConversation,
    conversationEnabled,
    analysisByMessage,
    orderPendingIds,
    orders,
    ordersTotal,
    ordersLoading,
    catalog,
    catalogLoading,
    fetchStatus,
    fetchConversation,
    clearConversation,
    orderFor,
    pendingFor,
    revisionsFor,
    analysisFor,
    analysisForConversation,
    isEnabledFor,
    setConversationEnabled,
    saveOrder,
    setOrderStatus,
    acceptPending,
    reopenOrder,
    fetchOrders,
    fetchCatalog,
    createCatalogItem,
    updateCatalogItem,
    deleteCatalogItem,
    setMasterEnabled,
    setASREnabled,
    setChannelEnabled,
    backfill,
    subscribe,
    unsubscribe,
  }
})
