<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useTemplatesStore } from '@/stores/templates'
import type { Template } from '@/lib/api'

const emit = defineEmits<{ select: [template: Template]; close: [] }>()

const store = useTemplatesStore()
const search = ref('')

onMounted(() => {
  if (!store.loadedOnce) store.fetchTemplates()
})

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return store.templates
  return store.templates.filter(
    (t) =>
      t.name.toLowerCase().includes(q) ||
      t.content.toLowerCase().includes(q),
  )
})

function pick(t: Template) {
  emit('select', t)
  emit('close')
}

const categoryLabels: Record<string, string> = {
  marketing: 'Marketing',
  utility: 'Utilidad',
  soporte: 'Soporte',
  general: 'General',
}
</script>

<template>
  <div
    class="absolute bottom-full left-0 z-30 mb-2 w-80 rounded-xl border border-slate-200 bg-white shadow-lg dark:border-slate-800 dark:bg-[#101828]"
  >
    <div class="border-b border-slate-100 p-2 dark:border-slate-800">
      <input
        v-model="search"
        type="text"
        placeholder="Buscar plantilla..."
        class="w-full rounded-lg bg-slate-100 px-3 py-1.5 text-sm text-slate-700 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-sky-400/60 dark:bg-slate-800 dark:text-slate-100"
      />
    </div>

    <div class="max-h-56 overflow-y-auto p-1">
      <div v-if="store.loading && !store.loadedOnce" class="space-y-1 p-1">
        <div v-for="i in 3" :key="i" class="h-12 animate-pulse rounded-lg bg-slate-100 dark:bg-slate-800" />
      </div>

      <p v-else-if="filtered.length === 0" class="px-3 py-6 text-center text-xs text-slate-400">
        No hay plantillas que coincidan
      </p>

      <button
        v-for="t in filtered"
        :key="t.id"
        type="button"
        class="w-full rounded-lg px-3 py-2 text-left transition-colors hover:bg-slate-100 dark:hover:bg-slate-800"
        @click="pick(t)"
      >
        <div class="flex items-center justify-between gap-2">
          <span class="truncate text-xs font-semibold text-slate-700 dark:text-slate-200">{{ t.name }}</span>
          <span class="shrink-0 text-[10px] font-medium uppercase tracking-wide text-slate-400">{{
            categoryLabels[t.category] || t.category
          }}</span>
        </div>
        <p class="mt-0.5 line-clamp-2 text-[11px] leading-snug text-slate-500 dark:text-slate-400">
          {{ t.content }}
        </p>
      </button>
    </div>
  </div>
</template>
