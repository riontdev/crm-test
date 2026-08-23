<script setup lang="ts">
export interface QuickAction {
  icon: string
  label: string
}

interface Props {
  actions?: QuickAction[]
}

withDefaults(defineProps<Props>(), {
  actions: () => [
    { icon: 'bolt', label: 'Info de precios' },
    { icon: 'event', label: 'Agendar llamada' },
  ],
})

const emit = defineEmits<{ select: [action: QuickAction] }>()
</script>

<template>
  <div class="flex gap-2 overflow-x-auto pb-1" role="toolbar" aria-label="Acciones rápidas">
    <button
      v-for="action in actions"
      :key="action.label"
      type="button"
      class="flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition-all hover:-translate-y-0.5 hover:border-sky-400 hover:text-sky-600 hover:shadow-sm dark:border-slate-700 dark:bg-[#101828] dark:text-slate-300 dark:hover:text-sky-400"
      @click="emit('select', action)"
    >
      <span class="material-symbols-outlined text-sm" aria-hidden="true">{{ action.icon }}</span>
      {{ action.label }}
    </button>
  </div>
</template>
