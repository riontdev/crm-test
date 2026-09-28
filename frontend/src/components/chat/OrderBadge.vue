<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, PackageCheck, Info, MessageCircleWarning, Sparkles, Undo2 } from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import type { InsightIntent } from '@/lib/api'

interface Props {
  intent?: InsightIntent | null
  needsReview?: boolean
  /** Hay un pedido del cliente mas nuevo que el vigente. Distinto de needsReview. */
  pending?: boolean
  size?: 'sm' | 'md'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
})

/**
 * Badge de intención del pedido.
 *
 * `reclamo` NO es lo mismo que `info` ni que `otro`: es el único estado en el
 * que el operador tiene que mirar el mensaje YA. Por eso va en rojo y no en
 * gris. Confundir un reclamo con un saludo es exactamente el error que una
 * bandeja mal diseñada provoca.
 */
const styles: Record<string, { pill: string; label: string }> = {
  pedido: {
    pill: 'bg-indigo-500/10 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400',
    label: 'Pedido',
  },
  reclamo: {
    pill: 'bg-red-500/10 text-red-700 dark:bg-red-500/15 dark:text-red-400',
    label: 'Reclamo',
  },
  info: {
    pill: 'bg-sky-500/10 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400',
    label: 'Info',
  },
  otro: {
    pill: 'bg-slate-500/10 text-slate-600 dark:bg-slate-400/10 dark:text-slate-300',
    label: 'Otro',
  },
}

const active = computed(() => styles[props.intent || 'otro'])
const icon = computed(() => {
  // Pending le gana a needsReview: "el cliente pidio algo mas y no lo
  // consolidamos" es una accion pendiente del operador, mientras que revisar
  // es una duda de la IA. Lo primero se pierde si el operador no lo ve.
  if (props.pending) return Undo2
  if (props.needsReview) return AlertTriangle
  switch (props.intent) {
    case 'pedido':
      return PackageCheck
    case 'reclamo':
      return MessageCircleWarning
    case 'info':
      return Info
    default:
      return null
  }
})

const classes = computed(() =>
  cn(
    'inline-flex items-center gap-1 rounded-full font-medium whitespace-nowrap',
    props.size === 'sm' ? 'px-1.5 py-0.5 text-[10px]' : 'px-2 py-0.5 text-[11px]',
    active.value.pill,
    // Anillo amber para el pendiente: el color del texto dice el tipo de
    // pedido y el borde dice "esto necesita una decision".
    props.pending && 'ring-1 ring-amber-500/60',
  ),
)
</script>

<template>
  <span :class="classes">
    <component :is="icon" v-if="icon" class="h-3 w-3 shrink-0" aria-hidden="true" />
    <Sparkles v-if="!icon && intent === 'otro'" class="h-3 w-3 shrink-0" aria-hidden="true" />
    {{ active.label }}
    <template v-if="pending">· nuevo pedido</template>
    <template v-else-if="needsReview">· revisar</template>
  </span>
</template>
