<script setup lang="ts">
import { computed, ref } from 'vue'

interface SeriesPoint {
  date: string
  incoming: number
  outgoing: number
}

interface Props {
  series: SeriesPoint[]
  height?: number
}

const props = withDefaults(defineProps<Props>(), { height: 220 })

const PAD_LEFT = 38
const PAD_RIGHT = 6
const PAD_TOP = 12
const PAD_BOTTOM = 22
const SLOT_W = 46

const wrapRef = ref<HTMLElement | null>(null)
const hoveredIndex = ref<number | null>(null)
const tooltipPos = ref({ x: 0, y: 0 })

const count = computed(() => props.series.length)

const vbWidth = computed(() => PAD_LEFT + Math.max(count.value, 1) * SLOT_W + PAD_RIGHT)

const baseY = computed(() => props.height - PAD_BOTTOM)

const innerH = computed(() => props.height - PAD_TOP - PAD_BOTTOM)

const rawMax = computed(() =>
  count.value ? Math.max(...props.series.map((d) => (d.incoming || 0) + (d.outgoing || 0))) : 0,
)

const hasData = computed(() => rawMax.value > 0)

function niceCeil(v: number): number {
  if (v <= 0) return 4
  const pow = Math.pow(10, Math.floor(Math.log10(v)))
  for (const m of [1, 2, 2.5, 5, 10]) {
    if (m * pow >= v) return m * pow
  }
  return 10 * pow
}

const yMax = computed(() => niceCeil(rawMax.value))

const gridLines = computed(() =>
  [1, 2, 3, 4].map((i) => ({
    y: baseY.value - (innerH.value / 4) * i,
    value: (yMax.value / 4) * i,
  })),
)

const barW = computed(() => Math.min(SLOT_W * 0.55, 16))

const days = computed(() =>
  props.series.map((d, i) => {
    const hIn = ((d.incoming || 0) / yMax.value) * innerH.value
    const hOut = ((d.outgoing || 0) / yMax.value) * innerH.value
    return {
      i,
      date: d.date,
      x: PAD_LEFT + i * SLOT_W + (SLOT_W - barW.value) / 2,
      w: barW.value,
      hIn,
      hOut,
    }
  }),
)

const labelStep = computed(() => Math.ceil(Math.max(count.value, 1) / 7))

const hoveredPoint = computed<SeriesPoint | null>(() => {
  if (hoveredIndex.value === null || hoveredIndex.value >= props.series.length) return null
  return props.series[hoveredIndex.value]
})

const tooltipStyle = computed(() => ({
  left: `${tooltipPos.value.x}px`,
  top: `${tooltipPos.value.y}px`,
}))

function topRoundedPath(x: number, y: number, w: number, h: number): string {
  const r = Math.min(2, w / 2, h)
  return [
    `M ${x} ${y + h}`,
    `L ${x} ${y + r}`,
    `Q ${x} ${y} ${x + r} ${y}`,
    `L ${x + w - r} ${y}`,
    `Q ${x + w} ${y} ${x + w} ${y + r}`,
    `L ${x + w} ${y + h}`,
    'Z',
  ].join(' ')
}

function formatY(v: number): string {
  if (v >= 1000) return `${Number((v / 1000).toFixed(1))}k`
  return String(Math.round(v))
}

function parseDate(date: string): Date | null {
  const parts = date.split('-').map(Number)
  if (parts.length !== 3 || parts.some((p) => !Number.isFinite(p))) return null
  return new Date(parts[0], parts[1] - 1, parts[2])
}

function shortDate(date: string): string {
  const d = parseDate(date)
  if (!d) return date
  return d.toLocaleDateString('es', { day: 'numeric', month: 'short' })
}

function fullDate(date: string): string {
  const d = parseDate(date)
  if (!d) return date
  return d.toLocaleDateString('es', { weekday: 'long', day: 'numeric', month: 'long' })
}

function onMove(e: MouseEvent, i: number) {
  hoveredIndex.value = i
  const host = wrapRef.value
  if (!host) return
  const rect = host.getBoundingClientRect()
  tooltipPos.value = { x: e.clientX - rect.left, y: e.clientY - rect.top }
}

function onLeave() {
  hoveredIndex.value = null
}
</script>

<template>
  <div ref="wrapRef" class="relative w-full" @mouseleave="onLeave">
    <div class="mb-1 flex items-center justify-end gap-4">
      <span class="inline-flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
        <span class="h-2 w-2 rounded-full bg-sky-400" aria-hidden="true" />Entrantes
      </span>
      <span class="inline-flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
        <span class="h-2 w-2 rounded-full bg-indigo-500" aria-hidden="true" />Salientes
      </span>
    </div>

    <svg
      :viewBox="`0 0 ${vbWidth} ${height}`"
      class="block w-full select-none"
      role="img"
      aria-label="Gráfico de volumen de mensajes por día"
    >
      <g>
        <line
          v-for="(g, idx) in gridLines"
          :key="'grid-' + idx"
          :x1="PAD_LEFT"
          :x2="vbWidth - PAD_RIGHT"
          :y1="g.y"
          :y2="g.y"
          class="stroke-slate-200 dark:stroke-slate-800"
          stroke-width="1"
          stroke-dasharray="4 4"
        />
        <text
          v-for="(g, idx) in gridLines"
          :key="'grid-label-' + idx"
          :x="PAD_LEFT - 6"
          :y="g.y + 3"
          text-anchor="end"
          class="fill-slate-400 text-[10px] dark:fill-slate-500"
        >
          {{ formatY(g.value) }}
        </text>
      </g>

      <rect
        v-for="d in days"
        :key="'band-' + d.i"
        :x="PAD_LEFT + d.i * SLOT_W"
        y="0"
        :width="SLOT_W"
        :height="height"
        class="transition-colors duration-150"
        :class="
          hoveredIndex === d.i ? 'fill-slate-500/5 dark:fill-slate-400/10' : 'fill-transparent'
        "
      />

      <line
        :x1="PAD_LEFT"
        :x2="vbWidth - PAD_RIGHT"
        :y1="baseY"
        :y2="baseY"
        class="stroke-slate-300 dark:stroke-slate-700"
        stroke-width="1"
      />

      <g
        v-for="d in days"
        :key="'day-' + d.i"
        class="transition-opacity duration-150"
        :class="hoveredIndex !== null && hoveredIndex !== d.i ? 'opacity-40' : 'opacity-100'"
      >
        <path
          v-if="d.hOut > 0"
          :d="topRoundedPath(d.x, baseY - d.hIn - d.hOut, d.w, d.hOut)"
          class="fill-indigo-500"
        />
        <path
          v-else-if="d.hIn > 0"
          :d="topRoundedPath(d.x, baseY - d.hIn, d.w, d.hIn)"
          class="fill-sky-400"
        />
        <rect
          v-if="d.hOut > 0 && d.hIn > 0"
          :x="d.x"
          :y="baseY - d.hIn"
          :width="d.w"
          :height="d.hIn"
          class="fill-sky-400"
        />
        <text
          v-if="d.i % labelStep === 0"
          :x="PAD_LEFT + d.i * SLOT_W + SLOT_W / 2"
          :y="height - 6"
          text-anchor="middle"
          class="fill-slate-400 text-[10px] dark:fill-slate-500"
        >
          {{ shortDate(d.date) }}
        </text>
      </g>

      <g>
        <rect
          v-for="d in days"
          :key="'capture-' + d.i"
          :x="PAD_LEFT + d.i * SLOT_W"
          y="0"
          :width="SLOT_W"
          :height="height"
          class="cursor-pointer fill-transparent"
          @mousemove="onMove($event, d.i)"
        />
      </g>
    </svg>

    <div
      v-if="hoveredPoint"
      :style="tooltipStyle"
      class="pointer-events-none absolute z-10 min-w-[140px] -translate-x-1/2 translate-y-[calc(-100%_-_12px)] rounded-lg border border-slate-200 bg-white px-3 py-2 shadow-lg dark:border-slate-700 dark:bg-[#101828]"
    >
      <p class="text-[11px] font-medium capitalize text-slate-500 dark:text-slate-400">
        {{ fullDate(hoveredPoint.date) }}
      </p>
      <div class="mt-1 space-y-0.5">
        <p class="flex items-center gap-1.5 text-xs text-slate-700 dark:text-slate-200">
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-sky-400" aria-hidden="true" />
          Entrantes
          <span class="ml-auto pl-3 font-semibold">{{ hoveredPoint.incoming }}</span>
        </p>
        <p class="flex items-center gap-1.5 text-xs text-slate-700 dark:text-slate-200">
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-indigo-500" aria-hidden="true" />
          Salientes
          <span class="ml-auto pl-3 font-semibold">{{ hoveredPoint.outgoing }}</span>
        </p>
      </div>
    </div>

    <div v-if="!hasData" class="absolute inset-0 flex items-center justify-center">
      <p class="text-sm text-slate-400 dark:text-slate-500">Sin datos en el período</p>
    </div>
  </div>
</template>
