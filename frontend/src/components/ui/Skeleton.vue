<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  lines?: number
}

const props = withDefaults(defineProps<Props>(), { lines: 1 })

const lineClasses = [
  'w-full',
  'w-full',
  'w-11/12',
  'w-10/12',
  'w-full',
  'w-3/4',
]

const rows = computed(() => Array.from({ length: props.lines }, (_, i) => i))
</script>

<template>
  <div class="animate-pulse space-y-2" aria-hidden="true">
    <div
      v-for="i in rows"
      :key="i"
      :class="[lineClasses[(i - 1) % lineClasses.length], cn('h-4 rounded-lg bg-slate-200 dark:bg-slate-800')]"
    />
  </div>
</template>
