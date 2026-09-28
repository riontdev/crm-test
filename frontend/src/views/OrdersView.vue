<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Filter,
  Lock,
  Mic,
  RefreshCw,
  Search,
} from 'lucide-vue-next'
import OrderBadge from '@/components/chat/OrderBadge.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'
import { useInsightsStore } from '@/stores/insights'
import type { InsightIntent, OrderStatus } from '@/lib/api'

const insights = useInsightsStore()

const search = ref('')
const statusFilter = ref<OrderStatus | ''>('')
const intentFilter = ref<InsightIntent | ''>('')
const page = ref(0)
const pageSize = 20

const statusOptions: { value: OrderStatus; label: string }[] = [
  { value: 'nuevo', label: 'Nuevo' },
  { value: 'confirmado', label: 'Confirmado' },
  { value: 'entregado', label: 'Entregado' },
  { value: 'descartado', label: 'Descartado' },
]

const intentOptions: { value: InsightIntent; label: string }[] = [
  { value: 'pedido', label: 'Pedido' },
  { value: 'info', label: 'Info' },
  { value: 'reclamo', label: 'Reclamo' },
  { value: 'otro', label: 'Otro' },
]

const totalPages = computed(() => Math.max(1, Math.ceil(insights.ordersTotal / pageSize)))
const hasFilters = computed(() => !!search.value || !!statusFilter.value || !!intentFilter.value)

const stats = computed(() => {
  const list = insights.orders
  return {
    pedidos: list.filter((o) => o.intent === 'pedido').length,
    revisar: list.filter((o) => o.needs_review).length,
    editados: list.filter((o) => o.edited).length,
  }
})

async function load() {
  await insights.fetchOrders({
    status: statusFilter.value || undefined,
    intent: intentFilter.value || undefined,
    search: search.value || undefined,
    limit: pageSize,
    offset: page.value * pageSize,
  })
}

onMounted(async () => {
  // await y no void: con `void fetchStatus()` seguido de `if (!enabled) return`,
  // enabled todavia daba false (status es null) y la vista se quedaba en
  // "cargando" para siempre al entrar directo o tras recargar la pagina.
  if (!insights.status) await insights.fetchStatus()
  if (!insights.enabled) return
  insights.subscribe()
  await load()
})

// Cada cambio de filtro vuelve a la primera pagina: quedarse en la pagina 4
// con un filtro nuevo muestra una lista vacia sin explicacion.
watch([search, statusFilter, intentFilter], () => {
  page.value = 0
  void load()
})

function goTo(p: number) {
  if (p < 0 || p >= totalPages.value) return
  page.value = p
  void load()
}

function pct(v?: number | null) {
  return v == null ? null : Math.round(v * 100)
}
</script>

<template>
  <div class="flex h-full flex-col overflow-hidden">
    <header class="shrink-0 border-b border-slate-200 px-6 py-4 dark:border-slate-800">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-lg font-semibold text-slate-900 dark:text-slate-50">Pedidos</h1>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Detectados por IA en las conversaciones. Corregí uno y queda fijado: la IA no lo vuelve a tocar.
          </p>
        </div>
        <button
          class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          :disabled="insights.ordersLoading"
          @click="load"
        >
          <RefreshCw class="h-3.5 w-3.5" :class="insights.ordersLoading && 'animate-spin'" />
          Actualizar
        </button>
      </div>

      <div class="mt-3 flex flex-wrap items-center gap-2">
        <div class="relative min-w-[200px] flex-1">
          <Search class="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" aria-hidden="true" />
          <input
            v-model="search"
            type="search"
            placeholder="Buscar por contacto, resumen o producto…"
            aria-label="Buscar pedidos"
            class="w-full rounded-lg border border-slate-200 bg-white py-1.5 pr-2 pl-8 text-sm dark:border-slate-800 dark:bg-slate-900"
          />
        </div>
        <div class="relative">
          <Filter class="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" aria-hidden="true" />
          <select
            v-model="statusFilter"
            aria-label="Filtrar por estado"
            class="appearance-none rounded-lg border border-slate-200 bg-white py-1.5 pr-7 pl-7 text-xs dark:border-slate-800 dark:bg-slate-900"
          >
            <option value="">Todos los estados</option>
            <option v-for="s in statusOptions" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </div>
        <select
          v-model="intentFilter"
          aria-label="Filtrar por intención"
          class="rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-xs dark:border-slate-800 dark:bg-slate-900"
        >
          <option value="">Toda intención</option>
          <option v-for="i in intentOptions" :key="i.value" :value="i.value">{{ i.label }}</option>
        </select>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto p-6">
      <p v-if="!insights.status" class="text-sm text-slate-500">Cargando estado del análisis…</p>

      <div v-else-if="!insights.enabled" class="rounded-xl border border-slate-200 p-6 text-sm dark:border-slate-800">
        <h2 class="font-medium text-slate-800 dark:text-slate-100">El análisis de pedidos está apagado</h2>
        <p class="mt-1 text-slate-500 dark:text-slate-400">
          Prendelo desde Agentes IA para empezar a detectar pedidos. No se está leyendo nada mientras esté apagado.
        </p>
      </div>

      <template v-else>
        <div v-if="insights.orders.length" class="mb-3 flex flex-wrap gap-4 text-xs text-slate-500 dark:text-slate-400">
          <span><strong class="tabular-nums">{{ insights.ordersTotal }}</strong> pedidos</span>
          <span v-if="stats.pedidos">{{ stats.pedidos }} de esta página son pedidos</span>
          <span v-if="stats.revisar" class="inline-flex items-center gap-1 text-amber-600 dark:text-amber-500">
            <AlertTriangle class="h-3 w-3" aria-hidden="true" /> {{ stats.revisar }} para revisar
          </span>
          <span v-if="stats.editados" class="inline-flex items-center gap-1">
            <Lock class="h-3 w-3" aria-hidden="true" /> {{ stats.editados }} editados a mano
          </span>
        </div>

        <div v-if="insights.ordersLoading && !insights.orders.length" class="space-y-2">
          <div v-for="i in 4" :key="i" class="h-24 animate-pulse rounded-xl bg-slate-100 dark:bg-slate-800/60" />
        </div>

        <EmptyState
          v-else-if="!insights.orders.length"
          :title="hasFilters ? 'Ningún pedido coincide' : 'Todavía no hay pedidos'"
          :description="
            hasFilters
              ? 'Probá con otros filtros o limpiá la búsqueda.'
              : 'Cuando un cliente pida algo, el pedido aparece acá automáticamente.'
          "
          :icon="hasFilters ? 'search_off' : 'inventory_2'"
        />

        <ul v-else class="space-y-2">
          <li v-for="o in insights.orders" :key="o.id">
            <RouterLink
              :to="`/inbox/${o.conversation_id}`"
              class="block rounded-xl border border-slate-200 bg-white p-3.5 transition-colors hover:border-sky-300 hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-sky-700 dark:hover:bg-slate-800/40"
            >
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span class="truncate text-sm font-medium text-slate-900 dark:text-slate-100">
                      {{ o.contact_name || 'Sin nombre' }}
                    </span>
                    <ChannelBadge :channel="o.channel" />
                    <OrderBadge
                      :intent="o.intent"
                      :needs-review="o.needs_review"
                      :pending="o.pending"
                      size="sm"
                    />
                    <span
                      v-if="o.edited"
                      class="inline-flex items-center gap-1 rounded-full bg-slate-500/10 px-1.5 py-0.5 text-[10px] font-medium text-slate-600 dark:bg-slate-400/10 dark:text-slate-300"
                    >
                      <Lock class="h-2.5 w-2.5" aria-hidden="true" /> editado
                    </span>
                    <span
                      v-if="insights.analysisForConversation(o.conversation_id).some(a => a.asr_text)"
                      class="inline-flex items-center gap-1 text-[10px] text-slate-400"
                      title="Detectado a partir de notas de voz"
                    >
                      <Mic class="h-3 w-3" aria-hidden="true" /> voz
                    </span>
                  </div>
                  <p class="mt-1 line-clamp-2 text-sm text-slate-600 dark:text-slate-300">{{ o.resumen }}</p>
                </div>

                <div class="flex shrink-0 flex-col items-end gap-1.5">
                  <select
                    :value="o.status"
                    :aria-label="`Estado del pedido de ${o.contact_name || 'este contacto'}`"
                    class="rounded-md border border-slate-200 bg-white px-1.5 py-1 text-[11px] text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                    @change.stop.prevent="insights.setOrderStatus(o.id, o.conversation_id, ($event.target as HTMLSelectElement).value as OrderStatus)"
                    @click.stop
                  >
                    <option v-for="s in statusOptions" :key="s.value" :value="s.value">{{ s.label }}</option>
                  </select>
                  <span
                    v-if="pct(o.confianza) != null"
                    class="text-[10px] text-slate-400 tabular-nums"
                    :class="(pct(o.confianza) ?? 100) < 70 && 'text-amber-600 dark:text-amber-500'"
                  >
                    {{ pct(o.confianza) }}%
                  </span>
                </div>
              </div>

              <ul v-if="o.productos.length" class="mt-2 flex flex-wrap gap-1.5">
                <li
                  v-for="(p, i) in o.productos"
                  :key="`${p}-${i}`"
                  class="rounded-md bg-indigo-500/10 px-1.5 py-0.5 text-[11px] text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300"
                >
                  <span v-if="o.cantidades[i] != null" class="font-bold tabular-nums">{{ o.cantidades[i] }}×</span>
                  {{ p }}
                </li>
              </ul>
            </RouterLink>
          </li>
        </ul>

        <nav v-if="totalPages > 1" class="mt-4 flex items-center justify-center gap-2" aria-label="Paginación">
          <button
            class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2 py-1 text-xs disabled:opacity-40 dark:border-slate-800"
            :disabled="page === 0"
            @click="goTo(page - 1)"
          >
            <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" /> Anterior
          </button>
          <span class="text-xs text-slate-500 tabular-nums">Página {{ page + 1 }} de {{ totalPages }}</span>
          <button
            class="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-2 py-1 text-xs disabled:opacity-40 dark:border-slate-800"
            :disabled="page >= totalPages - 1"
            @click="goTo(page + 1)"
          >
            Siguiente <ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
          </button>
        </nav>
      </template>
    </div>
  </div>
</template>
