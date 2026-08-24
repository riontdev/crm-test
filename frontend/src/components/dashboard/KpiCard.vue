<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  icon: string
  label: string
  value: string
  trend?: number | null
  trendLabel?: string
  invertTrend?: boolean
  subValue?: string
  subLabel?: string
}

const props = withDefaults(defineProps<Props>(), {
  trendLabel: 'vs período anterior',
  invertTrend: false,
})

const showTrend = computed(() => props.trend !== undefined)

type TrendState = 'up' | 'down' | 'flat'

const trendState = computed<TrendState>(() => {
  if (props.trend === null || props.trend === undefined || props.trend === 0 || !Number.isFinite(props.trend)) {
    return 'flat'
  }
  return props.trend > 0 ? 'up' : 'down'
})

const trendIcon = computed(() => {
  if (trendState.value === 'up') return 'trending_up'
  if (trendState.value === 'down') return 'trending_down'
  return 'minus'
})

const trendClasses = computed(() => {
  if (trendState.value === 'flat') return 'text-slate-400 dark:text-slate-500'
  const positiveIsGood = !props.invertTrend
  const isPositive = trendState.value === 'up'
  if (isPositive === positiveIsGood) return 'text-emerald-600 dark:text-emerald-400'
  return 'text-red-500 dark:text-red-400'
})

const subLine = computed(() => {
  if (!props.subValue) return ''
  return props.subLabel ? `${props.subValue} ${props.subLabel}` : props.subValue
})

function formatDelta(trend: number | null | undefined): string {
  if (trend === null || trend === undefined || trend === 0 || !Number.isFinite(trend)) return '—'
  const abs = Math.abs(trend)
  const num = Number.isInteger(abs) ? abs.toString() : abs.toFixed(1)
  return `${trend > 0 ? '+' : '-'}${num}%`
}
</script>

<template>
  <article
    class="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-all duration-200 hover:shadow dark:border-slate-800 dark:bg-[#101828]"
  >
    <div class="flex items-center gap-3">
      <p class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500">
        {{ label }}
      </p>
      <span
        class="ml-auto flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-sky-500/10 text-sky-600 dark:bg-sky-500/15 dark:text-sky-400"
      >
        <span class="material-symbols-outlined text-xl" aria-hidden="true">{{ icon }}</span>
      </span>
    </div>

    <div>
      <p class="text-3xl font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
        {{ value }}
      </p>
      <p v-if="subLine" class="mt-1 text-xs text-slate-400 dark:text-slate-500">{{ subLine }}</p>
    </div>

    <p v-if="showTrend" class="inline-flex flex-wrap items-center gap-1 text-xs font-medium">
      <span :class="['inline-flex items-center gap-1', trendClasses]">
        <span class="material-symbols-outlined text-sm" aria-hidden="true">{{ trendIcon }}</span>
        {{ formatDelta(trend) }}
      </span>
      <span class="font-normal text-slate-400 dark:text-slate-500">{{ trendLabel }}</span>
    </p>
  </article>
</template>
