<script setup lang="ts">
import { computed } from 'vue'
import { hashColor, initials } from '@/lib/utils'

interface Props {
  src?: string
  name: string
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
  status?: 'online' | 'offline'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
})

const sizeMap: Record<NonNullable<Props['size']>, { box: string; text: string; dot: string }> = {
  xs: { box: 'h-6 w-6', text: 'text-[10px]', dot: 'h-2 w-2' },
  sm: { box: 'h-8 w-8', text: 'text-xs', dot: 'h-2.5 w-2.5' },
  md: { box: 'h-10 w-10', text: 'text-sm', dot: 'h-3 w-3' },
  lg: { box: 'h-12 w-12', text: 'text-base', dot: 'h-3.5 w-3.5' },
  xl: { box: 'h-16 w-16', text: 'text-xl', dot: 'h-4 w-4' },
}

const boxClass = computed(() => sizeMap[props.size].box)
const textClass = computed(() => sizeMap[props.size].text)
const dotClass = computed(() => sizeMap[props.size].dot)

const fallbackColor = computed(() => hashColor(props.name || '?'))

const ringClass = computed(() =>
  props.status ? 'ring-2 ring-white dark:ring-slate-900' : '',
)

const statusDotClass = computed(() =>
  props.status === 'online'
    ? 'bg-emerald-500'
    : 'bg-slate-400 dark:bg-slate-600',
)
</script>

<template>
  <div class="relative shrink-0" :class="boxClass">
    <img
      v-if="src"
      :src="src"
      :alt="name"
      :class="[boxClass, ringClass, 'rounded-full object-cover']"
    />
    <div
      v-else
      :class="[boxClass, textClass, fallbackColor, ringClass, 'flex items-center justify-center rounded-full font-semibold select-none']"
    >
      {{ initials(name) }}
    </div>
    <span
      v-if="status"
      :class="[dotClass, statusDotClass, 'absolute right-0 bottom-0 rounded-full ring-2 ring-white dark:ring-slate-900']"
      aria-hidden="true"
    />
  </div>
</template>
