<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info' | 'accent'
  size?: 'sm' | 'md'
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'default',
  size: 'md',
})

const variantClasses: Record<NonNullable<Props['variant']>, string> = {
  default: 'bg-slate-500/10 text-slate-600 dark:bg-slate-400/10 dark:text-slate-300',
  success: 'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400',
  warning: 'bg-amber-500/10 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400',
  danger: 'bg-red-500/10 text-red-700 dark:bg-red-500/15 dark:text-red-400',
  info: 'bg-sky-500/10 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400',
  accent: 'bg-indigo-500/10 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400',
}

const classes = computed(() =>
  cn(
    'inline-flex items-center rounded-full font-medium whitespace-nowrap',
    props.size === 'sm' ? 'px-2 py-0.5 text-[11px]' : 'px-2.5 py-1 text-xs',
    variantClasses[props.variant],
  ),
)
</script>

<template>
  <span :class="classes"><slot /></span>
</template>
