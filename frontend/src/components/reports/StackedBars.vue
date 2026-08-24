<script setup lang="ts">
import { computed, ref } from 'vue'

interface Segment {
  key: string
  value: number
}

interface Row {
  label: string
  segments: Segment[]
}

interface Props {
  rows: Row[]
  colors: Record<string, string>
  height?: number
  legendLabels?: Record<string, string>
}

const props = withDefaults(defineProps<Props>(), {
  height: 240,
  legendLabels: () => ({}),
})

const PAD_LEFT = 38
const PAD_RIGHT = 6
const PAD_TOP = 12
const PAD_BOTTOM = 22
const SLOT_W = 46

const wrapRef = ref<HTMLElement | null>(null)
const hoveredIndex = ref<number | null>(null)
const tooltipPos = ref({ x: 0, y: 0 })

const count = computed(() => props.rows.length)

const MIN_VB_WIDTH = 560

const contentW = computed(() => PAD_LEFT + Math.max(count.value, 1) * SLOT_W + PAD_RIGHT)

const vbWidth = computed(() => Math.max(contentW.value, MIN_VB_WIDTH))

const offsetX = computed(() => (vbWidth.value - contentW.value) / 2)

const baseY = computed(() => props.height - PAD_BOTTOM)

const innerH = computed(() => props.height - PAD_TOP - PAD_BOTTOM)

const rawMax = computed(() =>
  count.value ? Math.max(...props.rows.map((r) => r.segments.reduce((acc, s) => acc + (s.value || 0), 0))) : 0,
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

interface PlacedSegment extends Segment {
  y: number
  h: number
}

const bars = computed(() =>
  props.rows.map((row, i) => {
    let cum = 0
    const segs: PlacedSegment[] = row.segments.map((s) => {
      const h = ((s.value || 0) / yMax.value) * innerH.value
      const placed = { ...s, y: baseY.value - cum - h, h }
      cum += h
      return placed
    })
    return {
      i,
      label: row.label,
      x: PAD_LEFT + i * SLOT_W + (SLOT_W - barW.value) / 2,
      w: barW.value,
      segs,
    }
  }),
)

const labelStep = computed(() => Math.ceil(Math.max(count.value, 1) / 7))

const legendKeys = computed(() => {
  const keys: string[] = []
  const seen = new Set<string>()
  for (const row of props.rows) {
    for (const s of row.segments) {
      if (!seen.has(s.key)) {
        seen.add(s.key)
        keys.push(s.key)
      }
    }
  }
  return keys
})

const hoveredRow = computed<Row | null>(() => {
  if (hoveredIndex.value === null || hoveredIndex.value >= props.rows.length) return null
  return props.rows[hoveredIndex.value]
})

const tooltipStyle = computed(() => ({
  left: `${tooltipPos.value.x}px`,
  top: `${tooltipPos.value.y}px`,
}))

function segmentColor(key: string): string {
  return props.colors[key] ?? '#38BDF8'
}

function segmentLabel(key: string): string {
  return props.legendLabels[key] ?? key
}

function formatY(v: number): string {
  if (v >= 1000) return `${Number((v / 1000).toFixed(1))}k`
  return String(Math.round(v))
}

function shortLabel(label: string): string {
  const parts = label.split('-').map(Number)
  if (parts.length !== 3 || parts.some((p) => !Number.isFinite(p))) return label
  const d = new Date(parts[0], parts[1] - 1, parts[2])
  if (Number.isNaN(d.getTime())) return label
  return d.toLocaleDateString('es', { day: 'numeric', month: 'short' })
}

function fullLabel(label: string): string {
  const parts = label.split('-').map(Number)
  if (parts.length !== 3 || parts.some((p) => !Number.isFinite(p))) return label
  const d = new Date(parts[0], parts[1] - 1, parts[2])
  if (Number.isNaN(d.getTime())) return label
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
      <span
        v-for="key in legendKeys"
        :key="'legend-' + key"
        class="inline-flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400"
      >
        <span
          class="h-2 w-2 rounded-full"
          :style="{ backgroundColor: segmentColor(key) }"
          aria-hidden="true"
        />{{ segmentLabel(key) }}
      </span>
    </div>

    <svg
      :viewBox="`0 0 ${vbWidth} ${height}`"
      class="block w-full select-none"
      role="img"
      aria-label="Gráfico de barras apiladas"
    >
      <g :transform="`translate(${offsetX},0)`">
        <g>
          <line
            v-for="(g, idx) in gridLines"
            :key="'grid-' + idx"
            :x1="PAD_LEFT"
            :x2="contentW - PAD_RIGHT"
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
          v-for="b in bars"
          :key="'band-' + b.i"
          :x="PAD_LEFT + b.i * SLOT_W"
          y="0"
          :width="SLOT_W"
          :height="height"
          class="transition-colors duration-150"
          :class="hoveredIndex === b.i ? 'fill-slate-500/5 dark:fill-slate-400/10' : 'fill-transparent'"
        />

        <line
          :x1="PAD_LEFT"
          :x2="contentW - PAD_RIGHT"
          :y1="baseY"
          :y2="baseY"
          class="stroke-slate-300 dark:stroke-slate-700"
          stroke-width="1"
        />

        <g
          v-for="b in bars"
          :key="'bar-' + b.i"
          class="transition-opacity duration-150"
          :class="hoveredIndex !== null && hoveredIndex !== b.i ? 'opacity-40' : 'opacity-100'"
        >
          <rect
            v-for="(seg, sIdx) in b.segs"
            v-show="seg.h > 0"
            :key="'seg-' + b.i + '-' + sIdx"
            :x="b.x"
            :y="seg.y"
            :width="b.w"
            :height="seg.h"
            :fill="segmentColor(seg.key)"
            rx="1.5"
          />
          <text
            v-if="b.i % labelStep === 0"
            :x="PAD_LEFT + b.i * SLOT_W + SLOT_W / 2"
            :y="height - 6"
            text-anchor="middle"
            class="fill-slate-400 text-[10px] dark:fill-slate-500"
          >
            {{ shortLabel(b.label) }}
          </text>
        </g>

        <g>
          <rect
            v-for="b in bars"
            :key="'capture-' + b.i"
            :x="PAD_LEFT + b.i * SLOT_W"
            y="0"
            :width="SLOT_W"
            :height="height"
            class="cursor-pointer fill-transparent"
            @mousemove="onMove($event, b.i)"
          />
        </g>
      </g>
    </svg>

    <div
      v-if="hoveredRow"
      :style="tooltipStyle"
      class="pointer-events-none absolute z-10 min-w-[140px] -translate-x-1/2 translate-y-[calc(-100%_-_12px)] rounded-lg border border-slate-200 bg-white px-3 py-2 shadow-lg dark:border-slate-700 dark:bg-[#101828]"
    >
      <p class="text-[11px] font-medium capitalize text-slate-500 dark:text-slate-400">
        {{ fullLabel(hoveredRow.label) }}
      </p>
      <div class="mt-1 space-y-0.5">
        <p
          v-for="seg in hoveredRow.segments"
          :key="'tip-' + seg.key"
          class="flex items-center gap-1.5 text-xs text-slate-700 dark:text-slate-200"
        >
          <span
            class="h-1.5 w-1.5 shrink-0 rounded-full"
            :style="{ backgroundColor: segmentColor(seg.key) }"
            aria-hidden="true"
          />
          {{ segmentLabel(seg.key) }}
          <span class="ml-auto pl-3 font-semibold">{{ seg.value }}</span>
        </p>
      </div>
    </div>

    <div v-if="!hasData" class="absolute inset-0 flex items-center justify-center">
      <p class="text-sm text-slate-400 dark:text-slate-500">Sin datos en el rango</p>
    </div>
  </div>
</template>
