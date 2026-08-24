<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useReportsStore } from '@/stores/reports'
import type { ReportRow } from '@/lib/api'
import StackedBars from '@/components/reports/StackedBars.vue'
import Button from '@/components/ui/Button.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'

const store = useReportsStore()

const CHANNEL_KEYS = ['whatsapp', 'instagram', 'facebook']

const CHANNEL_COLORS: Record<string, string> = {
  whatsapp: '#25D366',
  instagram: '#E1306C',
  facebook: '#1877F2',
  messenger: '#1877F2',
}

const CHANNEL_LABELS: Record<string, string> = {
  whatsapp: 'WhatsApp',
  instagram: 'Instagram',
  facebook: 'Facebook',
  messenger: 'Messenger',
}

function toDateInput(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

const today = new Date()
const fromDate = ref(toDateInput(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 29)))
const toDate = ref(toDateInput(today))

const totals = computed(() =>
  [...(store.data?.totals_by_channel ?? [])].sort(
    (a, b) => b.incoming + b.outgoing - (a.incoming + a.outgoing),
  ),
)

const totalIncoming = computed(() =>
  totals.value.reduce((acc, t) => acc + t.incoming, 0),
)

const totalOutgoing = computed(() =>
  totals.value.reduce((acc, t) => acc + t.outgoing, 0),
)

interface ChartSegment {
  key: string
  value: number
}

interface ChartRow {
  label: string
  segments: ChartSegment[]
}

const chartRows = computed<ChartRow[]>(() => {
  const daily: ReportRow[] = store.data?.daily ?? []
  if (daily.length === 0) return []
  const dates = [...new Set(daily.map((r) => r.date))].sort()
  const byDateChannel = new Map(daily.map((r) => [`${r.date}|${r.channel}`, r]))
  return dates.map((date) => ({
    label: date,
    segments: CHANNEL_KEYS.map((ch) => {
      const row = byDateChannel.get(`${date}|${ch}`)
      return { key: ch, value: row ? row.incoming + row.outgoing : 0 }
    }),
  }))
})

function fmtDuration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || seconds < 0) return '—'
  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  if (hours > 0) return `${hours}h ${String(minutes).padStart(2, '0')}m`
  if (minutes === 0) return `${secs}s`
  return `${minutes}m ${secs}s`
}

function applyFilters() {
  if (!fromDate.value || !toDate.value) return
  void store.fetchReports(fromDate.value, toDate.value)
}

function retry() {
  void store.fetchReports(fromDate.value || toDateInput(new Date(Date.now() - 29 * 86400000)), toDate.value || toDateInput(new Date()))
}

function exportCsv() {
  const rows = store.data?.daily ?? []
  if (rows.length === 0) return
  const lines = [
    'fecha,canal,entrantes,salientes',
    ...rows.map((r) => `${r.date},${r.channel},${r.incoming},${r.outgoing}`),
  ]
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `reportes-${store.data?.from ?? fromDate.value}_${store.data?.to ?? toDate.value}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  if (!store.data) void store.fetchReports(fromDate.value, toDate.value)
})
</script>

<template>
  <div class="h-full overflow-y-auto">
    <div class="px-6 pb-10 pt-6">
      <header>
        <h1 class="text-2xl font-semibold tracking-[-0.01em] text-slate-900 dark:text-white">
          Reportes
        </h1>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Analítica de tus conversaciones por período.
        </p>
      </header>

      <!-- Controles de rango -->
      <div class="mt-4 flex flex-wrap items-end gap-3">
        <div>
          <label
            for="reports-from"
            class="mb-1 block text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
          >
            Desde
          </label>
          <input
            id="reports-from"
            v-model="fromDate"
            type="date"
            class="w-40 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-900 outline-none transition-shadow focus-visible:ring-2 focus-visible:ring-sky-400/60 dark:bg-slate-800 dark:text-slate-100 [color-scheme:light] dark:[color-scheme:dark]"
          />
        </div>
        <div>
          <label
            for="reports-to"
            class="mb-1 block text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
          >
            Hasta
          </label>
          <input
            id="reports-to"
            v-model="toDate"
            type="date"
            class="w-40 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-900 outline-none transition-shadow focus-visible:ring-2 focus-visible:ring-sky-400/60 dark:bg-slate-800 dark:text-slate-100 [color-scheme:light] dark:[color-scheme:dark]"
          />
        </div>

        <Button size="sm" :disabled="!fromDate || !toDate" @click="applyFilters">Aplicar</Button>

        <Button
          variant="secondary"
          size="sm"
          class="ml-auto"
          :disabled="!store.data || store.data.daily.length === 0"
          @click="exportCsv"
        >
          <span class="material-symbols-outlined text-base" aria-hidden="true">download</span>
          Exportar CSV
        </Button>
      </div>

      <p
        v-if="store.error && store.data"
        class="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300"
      >
        {{ store.error }}
      </p>

      <!-- Carga -->
      <section v-if="store.loading && !store.data">
        <div class="mt-4 grid grid-cols-2 gap-4 xl:grid-cols-4">
          <div
            v-for="i in 4"
            :key="i"
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <Skeleton :lines="3" />
          </div>
        </div>
        <div class="mt-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]">
          <Skeleton :lines="6" />
        </div>
        <div class="mt-4 grid gap-4 lg:grid-cols-2">
          <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]">
            <Skeleton :lines="5" />
          </div>
          <div class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]">
            <Skeleton :lines="5" />
          </div>
        </div>
      </section>

      <EmptyState
        v-else-if="store.error && !store.data"
        icon="cloud_off"
        title="No se pudieron cargar los reportes"
        :description="store.error"
      >
        <template #action>
          <Button size="sm" @click="retry">Reintentar</Button>
        </template>
      </EmptyState>

      <template v-else-if="store.data">
        <!-- KPIs -->
        <section
          class="mt-4 grid grid-cols-2 gap-4 transition-opacity duration-200 xl:grid-cols-4"
          :class="store.loading ? 'opacity-60' : 'opacity-100'"
        >
          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <p
              class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
            >
              Total entrantes
            </p>
            <p class="mt-1 text-3xl font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
              {{ totalIncoming.toLocaleString('es') }}
            </p>
          </div>
          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <p
              class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
            >
              Total salientes
            </p>
            <p class="mt-1 text-3xl font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
              {{ totalOutgoing.toLocaleString('es') }}
            </p>
          </div>
          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <p
              class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
            >
              Tiempo medio respuesta
            </p>
            <p class="mt-1 text-3xl font-bold tracking-[-0.02em] text-sky-600 dark:text-sky-400">
              {{ fmtDuration(store.data.response_times.avg_seconds) }}
            </p>
          </div>
          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <p
              class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
            >
              Rango consultado
            </p>
            <p class="mt-1 truncate text-base font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
              {{ store.data.from }} → {{ store.data.to }}
            </p>
          </div>
        </section>

        <!-- Gráfico -->
        <section
          class="mt-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-opacity duration-200 dark:border-slate-800 dark:bg-[#101828]"
          :class="store.loading ? 'opacity-60' : 'opacity-100'"
        >
          <h3 class="mb-4 text-xl font-semibold text-slate-900 dark:text-white">
            Volumen diario por canal
          </h3>
          <StackedBars
            :rows="chartRows"
            :colors="CHANNEL_COLORS"
            :legend-labels="CHANNEL_LABELS"
            :height="240"
          />
        </section>

        <!-- Detalle inferior -->
        <section class="mt-4 grid gap-4 lg:grid-cols-2">
          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-opacity duration-200 dark:border-slate-800 dark:bg-[#101828]"
            :class="store.loading ? 'opacity-60' : 'opacity-100'"
          >
            <h3 class="text-xl font-semibold text-slate-900 dark:text-white">Totales por canal</h3>
            <ul v-if="totals.length > 0" class="mt-4 divide-y divide-slate-100 dark:divide-slate-800">
              <li
                v-for="t in totals"
                :key="t.channel"
                class="flex items-center justify-between gap-3 py-3 first:pt-0 last:pb-0"
              >
                <ChannelBadge :channel="t.channel" />
                <div class="flex items-center gap-6 text-right">
                  <div>
                    <p class="text-sm font-bold text-slate-900 dark:text-white">
                      {{ t.incoming.toLocaleString('es') }}
                    </p>
                    <p class="text-[11px] uppercase tracking-wide text-slate-400 dark:text-slate-500">
                      Entrantes
                    </p>
                  </div>
                  <div>
                    <p class="text-sm font-bold text-slate-900 dark:text-white">
                      {{ t.outgoing.toLocaleString('es') }}
                    </p>
                    <p class="text-[11px] uppercase tracking-wide text-slate-400 dark:text-slate-500">
                      Salientes
                    </p>
                  </div>
                  <div>
                    <p class="text-sm font-bold tabular-nums text-slate-900 dark:text-white">
                      {{ t.conversations.toLocaleString('es') }}
                    </p>
                    <p class="text-[11px] uppercase tracking-wide text-slate-400 dark:text-slate-500">
                      Convers.
                    </p>
                  </div>
                </div>
              </li>
            </ul>
            <EmptyState
              v-else
              icon="bar_chart"
              title="Sin datos por canal"
              description="Aún no hay actividad registrada en este rango."
            />
          </div>

          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-opacity duration-200 dark:border-slate-800 dark:bg-[#101828]"
            :class="store.loading ? 'opacity-60' : 'opacity-100'"
          >
            <h3 class="text-xl font-semibold text-slate-900 dark:text-white">
              Tiempos de primera respuesta
            </h3>
            <div class="mt-4 flex items-stretch divide-x divide-slate-100 dark:divide-slate-800">
              <div class="flex-1 pr-4 text-center">
                <p class="text-3xl font-bold tracking-[-0.02em] text-sky-600 dark:text-sky-400">
                  {{ fmtDuration(store.data.response_times.avg_seconds) }}
                </p>
                <p
                  class="mt-1 text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
                >
                  Promedio
                </p>
              </div>
              <div class="flex-1 px-4 text-center">
                <p class="text-3xl font-bold tracking-[-0.02em] text-emerald-500 dark:text-emerald-400">
                  {{ fmtDuration(store.data.response_times.min_seconds) }}
                </p>
                <p
                  class="mt-1 text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
                >
                  Mínima
                </p>
              </div>
              <div class="flex-1 pl-4 text-center">
                <p class="text-3xl font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
                  {{ fmtDuration(store.data.response_times.max_seconds) }}
                </p>
                <p
                  class="mt-1 text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
                >
                  Máxima
                </p>
              </div>
            </div>
          </div>
        </section>
      </template>
    </div>
  </div>
</template>
