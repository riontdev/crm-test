<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  channel: string
}

const props = defineProps<Props>()

const channelStyles: Record<string, { dot: string; pill: string }> = {
  whatsapp: {
    dot: 'bg-[#25D366]',
    pill: 'bg-[#25D366]/10 text-[#075E54] dark:bg-[#25D366]/15 dark:text-[#4ADE80]',
  },
  instagram: {
    dot: 'bg-[#E1306C]',
    pill: 'bg-[#E1306C]/10 text-[#B02E5F] dark:bg-[#E1306C]/15 dark:text-[#F472B6]',
  },
  facebook: {
    dot: 'bg-[#1877F2]',
    pill: 'bg-[#1877F2]/10 text-[#1D5FBF] dark:bg-[#1877F2]/15 dark:text-[#60A5FA]',
  },
}

const label = computed(() => props.channel.charAt(0).toUpperCase() + props.channel.slice(1))

const classes = computed(() =>
  cn(
    'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap',
    channelStyles[props.channel]?.pill ?? 'bg-slate-500/10 text-slate-600 dark:bg-slate-400/10 dark:text-slate-300',
  ),
)

const dotClass = computed(() => channelStyles[props.channel]?.dot ?? 'bg-slate-400')
</script>

<template>
  <span :class="classes">
    <span :class="[dotClass, 'h-1.5 w-1.5 rounded-full']" aria-hidden="true" />
    {{ label }}
  </span>
</template>
