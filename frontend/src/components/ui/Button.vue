<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'warning'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  disabled?: boolean
  type?: 'button' | 'submit' | 'reset'
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'primary',
  size: 'md',
  type: 'button',
})

const variantClasses: Record<NonNullable<Props['variant']>, string> = {
  primary: 'bg-sky-400 text-white hover:bg-sky-500 active:bg-sky-600 shadow-sm',
  secondary:
    'border border-slate-300 text-slate-700 hover:bg-slate-100 dark:border-slate-600 dark:text-slate-200 dark:hover:bg-slate-800',
  ghost: 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800',
  danger: 'bg-red-600 text-white hover:bg-red-700 active:bg-red-800 shadow-sm',
  warning: 'bg-amber-500 text-white hover:bg-amber-600 active:bg-amber-700 shadow-sm',
}

const sizeClasses: Record<NonNullable<Props['size']>, string> = {
  sm: 'px-3 py-1.5 text-xs gap-1',
  md: 'px-4 py-2 text-sm gap-2',
  lg: 'px-6 py-3 text-base gap-2',
}

const classes = computed(() =>
  cn(
    'inline-flex items-center justify-center rounded-lg font-medium transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-slate-900 disabled:opacity-50 disabled:pointer-events-none',
    variantClasses[props.variant],
    sizeClasses[props.size],
  ),
)
</script>

<template>
  <button :type="type" :class="classes" :disabled="disabled || loading">
    <span
      v-if="loading"
      class="h-4 w-4 animate-spin rounded-full border-2 border-current/30 border-t-current"
      aria-hidden="true"
    />
    <slot />
  </button>
</template>
