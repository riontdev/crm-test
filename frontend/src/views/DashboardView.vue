<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useStatsStore, type StatsPeriod } from '@/stores/stats'
import KpiCard from '@/components/dashboard/KpiCard.vue'
import BarChart from '@/components/dashboard/BarChart.vue'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'

const store = useStatsStore()

const PERIOD_LABELS: Record<StatsPeriod, string> = {
  '24h': 'Últimas 24h',
  '7d': '7 días',
  '30d': '30 días',
}

const periodOptions: Array<{ value: StatsPeriod; label: string }> = [
  { value: '24h', label: 'Últimas 24h' },
  { value: '7d', label: '7 días' },
  { value: '30d', label: '30 días' },
]

const CHANNEL_COLORS: Record<string, string> = {
  whatsapp: '#25D366',
  instagram: '#E1306C',
  facebook: '#1877F2',
  messenger: '#1877F2',
}

const overview = computed(() => store.overview)

const byChannelSorted = computed(() =>
  [...(overview.value?.by_channel ?? [])].sort((a, b) => b.messages - a.messages),
)

const maxChannelMessages = computed(() =>
  byChannelSorted.value.reduce((max, c) => Math.max(max, c.messages), 0),
)

const aiRatio = computed(() => {
  const ai = overview.value?.totals.ai_replies.count ?? 0
  const human = overview.value?.totals.ai_replies.human_count ?? 0
  const total = ai + human
  return total > 0 ? Math.round((ai / total) * 100) : null
})

const canExport = computed(
  () => !!overview.value && overview.value.daily_series.length > 0,
)

const quickActions = [
  { to: '/inbox', icon: 'inbox', label: 'Bandeja', desc: 'Ver conversaciones' },
  {
    to: '/agents',
    icon: 'smart_toy',
    label: 'Agentes IA',
    desc: 'Configurar respuestas automáticas',
  },
  { to: '/users', icon: 'group', label: 'Usuarios', desc: 'Gestionar accesos' },
]

function setPeriod(period: StatsPeriod) {
  if (period === store.currentPeriod) return
  void store.fetchOverview(period)
}

function retry() {
  void store.fetchOverview(store.currentPeriod)
}

function fmtDuration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || seconds < 0) return '—'
  const total = Math.round(seconds)
  const minutes = Math.floor(total / 60)
  const secs = total % 60
  if (minutes === 0) return `${secs}s`
  return `${minutes}m ${secs}s`
}

function channelColor(channel: string): string {
  return CHANNEL_COLORS[channel] ?? '#38BDF8'
}

function channelPct(messages: number): string {
  if (messages <= 0 || maxChannelMessages.value === 0) return '0%'
  return `${Math.max((messages / maxChannelMessages.value) * 100, 4)}%`
}

function exportCsv() {
  const rows = overview.value?.daily_series ?? []
  if (rows.length === 0) return
  const lines = [
    'fecha,entrantes,salientes',
    ...rows.map((r) => `${r.date},${r.incoming},${r.outgoing}`),
  ]
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `dashboard-${store.currentPeriod}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  if (!store.overview) void store.fetchOverview('7d')
})
</script>

<template>
  <div class="h-full overflow-y-auto">
    <div class="px-6 pb-10 pt-6">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-semibold tracking-[-0.01em] text-slate-900 dark:text-white">
            Dashboard
          </h1>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
            Rendimiento en tiempo real de todos tus canales.
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <div class="flex rounded-lg bg-slate-100 p-1 dark:bg-slate-800">
            <button
              v-for="opt in periodOptions"
              :key="opt.value"
              type="button"
              class="rounded-md px-3 py-1.5 text-xs font-medium transition-all duration-200"
              :class="
                store.currentPeriod === opt.value
                  ? 'bg-white text-slate-900 shadow-sm dark:bg-slate-700 dark:text-white'
                  : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'
              "
              :aria-pressed="store.currentPeriod === opt.value"
              @click="setPeriod(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>

          <Button variant="secondary" size="sm" :disabled="!canExport" @click="exportCsv">
            <span class="material-symbols-outlined text-base" aria-hidden="true">download</span>
            Exportar
          </Button>
        </div>
      </header>

      <section v-if="store.loading && !overview">
        <div class="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <div
            v-for="i in 4"
            :key="i"
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <Skeleton :lines="3" />
          </div>
        </div>
        <div class="mt-4 grid gap-4 xl:grid-cols-3">
          <div
            class="col-span-2 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <Skeleton :lines="6" />
          </div>
          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <Skeleton :lines="6" />
          </div>
        </div>
      </section>

      <EmptyState
        v-else-if="store.error && !overview"
        icon="cloud_off"
        title="No se pudieron cargar las estadísticas"
        :description="store.error"
      >
        <template #action>
          <Button size="sm" @click="retry">Reintentar</Button>
        </template>
      </EmptyState>

      <section
        v-else-if="overview"
        class="transition-opacity duration-200"
        :class="store.loading ? 'opacity-60' : 'opacity-100'"
      >
        <div class="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <KpiCard
            icon="chat"
            label="Mensajes totales"
            :value="overview.totals.messages.count.toLocaleString('es')"
            :trend="overview.totals.messages.delta_pct ?? null"
          />
          <KpiCard
            icon="mark_email_unread"
            label="No leídos"
            :value="overview.totals.unread.total.toLocaleString('es')"
            :trend="null"
          />
          <KpiCard
            icon="forum"
            label="Conversaciones activas"
            :value="overview.totals.conversations.active.toLocaleString('es')"
            :trend="null"
            :sub-value="`+${overview.totals.conversations.new_in_period} nuevas en el período`"
          />
          <KpiCard
            icon="bolt"
            label="Tiempo medio de respuesta"
            :value="fmtDuration(overview.totals.first_response.avg_seconds)"
            :trend="null"
          />
        </div>

        <div class="mt-4 grid gap-4 xl:grid-cols-3">
          <div
            class="col-span-2 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <div class="mb-4 flex items-center justify-between gap-3">
              <h3 class="text-xl font-semibold text-slate-900 dark:text-white">
                Volumen de mensajes
              </h3>
              <Badge variant="info">{{ PERIOD_LABELS[store.currentPeriod] }}</Badge>
            </div>
            <BarChart :series="overview.daily_series" :height="220" />
          </div>

          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <h3 class="text-xl font-semibold text-slate-900 dark:text-white">Por canal</h3>
            <ul v-if="byChannelSorted.length > 0" class="mt-4 space-y-4">
              <li v-for="c in byChannelSorted" :key="c.channel">
                <div class="flex items-center justify-between gap-2">
                  <ChannelBadge :channel="c.channel" />
                  <div class="text-right">
                    <p class="text-sm font-bold text-slate-900 dark:text-white">
                      {{ c.messages.toLocaleString('es') }}
                    </p>
                    <p class="text-[11px] text-slate-400 dark:text-slate-500">
                      {{ c.conversations }} conv.
                    </p>
                  </div>
                </div>
                <div class="mt-1.5 h-1.5 rounded-full bg-slate-100 dark:bg-slate-800">
                  <div
                    class="h-full rounded-full transition-all duration-300"
                    :style="{ width: channelPct(c.messages), backgroundColor: channelColor(c.channel) }"
                  />
                </div>
              </li>
            </ul>
            <EmptyState
              v-else
              icon="bar_chart"
              title="Sin datos por canal"
              description="Aún no hay actividad registrada en este período."
            />
          </div>
        </div>

        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <div class="flex items-center justify-between gap-3">
              <h3 class="text-xl font-semibold text-slate-900 dark:text-white">
                Respuestas del agente IA
              </h3>
              <Badge variant="accent">{{ aiRatio !== null ? `${aiRatio}% IA` : '—' }}</Badge>
            </div>
            <div
              class="mt-4 flex items-stretch divide-x divide-slate-100 dark:divide-slate-800"
            >
              <div class="flex-1 pr-4 text-center">
                <p class="text-3xl font-bold tracking-[-0.02em] text-sky-600 dark:text-sky-400">
                  {{ overview.totals.ai_replies.count.toLocaleString('es') }}
                </p>
                <p
                  class="mt-1 text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
                >
                  IA
                </p>
              </div>
              <div class="flex-1 pl-4 text-center">
                <p class="text-3xl font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
                  {{ overview.totals.ai_replies.human_count.toLocaleString('es') }}
                </p>
                <p
                  class="mt-1 text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
                >
                  Humanas
                </p>
              </div>
            </div>
          </div>

          <div
            class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
          >
            <h3 class="text-xl font-semibold text-slate-900 dark:text-white">Accesos rápidos</h3>
            <nav class="mt-4 space-y-3">
              <RouterLink
                v-for="action in quickActions"
                :key="action.to"
                :to="action.to"
                class="flex items-center gap-3 rounded-xl border border-slate-200 px-4 py-3 transition-colors duration-200 hover:border-sky-400 dark:border-slate-800 dark:hover:border-sky-500"
              >
                <span
                  class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-300"
                >
                  <span class="material-symbols-outlined text-xl" aria-hidden="true">{{
                    action.icon
                  }}</span>
                </span>
                <span class="min-w-0">
                  <span class="block text-sm font-medium text-slate-900 dark:text-white">{{
                    action.label
                  }}</span>
                  <span class="block truncate text-xs text-slate-400 dark:text-slate-500">{{
                    action.desc
                  }}</span>
                </span>
                <span
                  class="material-symbols-outlined ml-auto shrink-0 text-xl text-slate-400 dark:text-slate-500"
                  aria-hidden="true"
                  >chevron_right</span
                >
              </RouterLink>
            </nav>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
