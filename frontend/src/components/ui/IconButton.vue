<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  icon: string
  variant?: 'ghost' | 'outline'
  size?: 'sm' | 'md'
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'ghost',
  size: 'md',
})

const variantClasses: Record<NonNullable<Props['variant']>, string> = {
  ghost:
    'text-slate-500 hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200',
  outline:
    'border border-slate-300 text-slate-600 hover:bg-slate-50 dark:border-slate-600 dark:text-slate-300 dark:hover:bg-slate-800',
}

const classes = computed(() =>
  cn(
    'inline-flex shrink-0 items-center justify-center rounded-lg transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 disabled:pointer-events-none disabled:opacity-40',
    props.size === 'sm' ? 'h-8 w-8' : 'h-10 w-10',
    variantClasses[props.variant],
  ),
)
</script>

<template>
  <button type="button" :class="classes" :disabled="disabled" :aria-label="icon.replace('_', ' ')">
    <span
      class="material-symbols-outlined"
      :class="size === 'sm' ? 'text-lg' : 'text-xl'"
      aria-hidden="true"
      >{{ icon }}</span
    >
  </button>
</template>
