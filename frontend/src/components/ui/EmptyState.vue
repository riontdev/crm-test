<script setup lang="ts">
import { cn } from '@/lib/utils'

interface Props {
  icon?: string
  title: string
  description?: string
}

withDefaults(defineProps<Props>(), {
  icon: 'inbox',
})
</script>

<template>
  <div :class="cn('flex flex-col items-center justify-center px-6 py-12 text-center')">
    <div
      class="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-slate-100 dark:bg-slate-800"
    >
      <span class="material-symbols-outlined text-2xl text-slate-400 dark:text-slate-500">{{
        icon
      }}</span>
    </div>
    <h3 class="text-base font-semibold text-slate-800 dark:text-slate-200">{{ title }}</h3>
    <p v-if="description" class="mt-1 max-w-xs text-sm text-slate-500 dark:text-slate-400">
      {{ description }}
    </p>
    <div v-if="$slots.action" class="mt-4">
      <slot name="action" />
    </div>
  </div>
</template>
