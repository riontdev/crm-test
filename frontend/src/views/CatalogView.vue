<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Pencil, Plus, RotateCcw, Save, Search, Trash2, X } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { useInsightsStore } from '@/stores/insights'
import type { CatalogItem } from '@/lib/api'

const insights = useInsightsStore()

const search = ref('')
const showInactive = ref(true)
const editingId = ref<string | null>(null)
const draft = ref({ name: '', category: 'general', aliases: '' })
const showForm = ref(false)

const categories = [
  { value: 'general', label: 'General' },
  { value: 'marketing', label: 'Marketing' },
  { value: 'material', label: 'Material' },
  { value: 'envio', label: 'Envío' },
]

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return insights.catalog.filter((c) => {
    if (!showInactive.value && !c.active) return false
    if (!q) return true
    return (
      c.name.toLowerCase().includes(q) ||
      c.aliases.some((a) => a.toLowerCase().includes(q))
    )
  })
})

/** Agrupar por categoría convierte 45 filas en algo legible de un vistazo. */
const grouped = computed(() => {
  const map = new Map<string, CatalogItem[]>()
  for (const item of filtered.value) {
    const list = map.get(item.category) || []
    list.push(item)
    map.set(item.category, list)
  }
  return [...map.entries()].sort((a, b) => a[0].localeCompare(b[0], 'es'))
})

const inactiveCount = computed(() => insights.catalog.filter((c) => !c.active).length)

function startEdit(item: CatalogItem) {
  editingId.value = item.id
  draft.value = {
    name: item.name,
    category: item.category,
    aliases: item.aliases.join(', '),
  }
  showForm.value = false
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit() {
  if (!editingId.value || !draft.value.name.trim()) return
  await insights.updateCatalogItem(editingId.value, {
    name: draft.value.name.trim(),
    category: draft.value.category,
    aliases: draft.value.aliases
      .split(',')
      .map((a) => a.trim().toLowerCase())
      .filter(Boolean),
  })
  editingId.value = null
}

async function toggleActive(item: CatalogItem) {
  await insights.updateCatalogItem(item.id, { active: !item.active })
}

async function remove(item: CatalogItem) {
  if (!confirm(`¿Sacar “${item.name}” del catálogo?`)) return
  await insights.deleteCatalogItem(item.id)
}

function openForm() {
  showForm.value = true
  draft.value = { name: '', category: 'general', aliases: '' }
}

function parseAliases(s: string) {
  return s
    .split(',')
    .map((a) => a.trim().toLowerCase())
    .filter(Boolean)
}

async function submitNew() {
  if (!draft.value.name.trim()) return
  const ok = await insights.createCatalogItem({
    name: draft.value.name.trim(),
    category: draft.value.category,
    aliases: parseAliases(draft.value.aliases),
  })
  if (ok) showForm.value = false
}

onMounted(async () => {
  if (!insights.status) void insights.fetchStatus()
  await insights.fetchCatalog(showInactive.value)
})
</script>

<template>
  <div class="flex h-full flex-col overflow-hidden">
    <header class="shrink-0 border-b border-slate-200 px-6 py-4 dark:border-slate-800">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-lg font-semibold text-slate-900 dark:text-slate-50">Catálogo</h1>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Productos que la IA reconoce en los mensajes. Los alias son las formas alternativas que escribe la gente.
          </p>
        </div>
        <Button variant="primary" size="sm" @click="openForm">
          <Plus class="h-3.5 w-3.5" /> Producto
        </Button>
      </div>

      <div class="mt-3 flex flex-wrap items-center gap-3">
        <div class="relative min-w-[200px] flex-1">
          <Search class="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" aria-hidden="true" />
          <input
            v-model="search"
            type="search"
            placeholder="Buscar producto o alias…"
            aria-label="Buscar en el catálogo"
            class="w-full rounded-lg border border-slate-200 bg-white py-1.5 pr-2 pl-8 text-sm dark:border-slate-800 dark:bg-slate-900"
          />
        </div>
        <label class="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-300">
          <input v-model="showInactive" type="checkbox" class="rounded" />
          Mostrar desactivados<template v-if="inactiveCount"> ({{ inactiveCount }})</template>
        </label>
        <span class="text-xs text-slate-400 tabular-nums">{{ filtered.length }} productos</span>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto p-6">
      <!-- alta -->
      <form
        v-if="showForm"
        class="mb-4 rounded-xl border border-sky-200 bg-sky-50/50 p-3.5 dark:border-sky-900 dark:bg-sky-950/20"
        @submit.prevent="submitNew"
      >
        <div class="grid gap-2 sm:grid-cols-[1fr_140px]">
          <input
            v-model="draft.name"
            placeholder="Nombre del producto (ej: Sticker circular)"
            maxlength="80"
            required
            aria-label="Nombre del producto"
            class="rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
          />
          <select
            v-model="draft.category"
            aria-label="Categoría"
            class="rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
          >
            <option v-for="c in categories" :key="c.value" :value="c.value">{{ c.label }}</option>
          </select>
        </div>
        <input
          v-model="draft.aliases"
          placeholder="Alias separados por coma (ej: sticker, calca, vinilo)"
          aria-label="Alias"
          class="mt-2 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
        />
        <div class="mt-2 flex justify-end gap-1.5">
          <Button type="button" variant="ghost" size="sm" @click="showForm = false">
            <X class="h-3.5 w-3.5" /> Cancelar
          </Button>
          <Button type="submit" variant="primary" size="sm" :disabled="!draft.name.trim()">
            <Save class="h-3.5 w-3.5" /> Guardar
          </Button>
        </div>
      </form>

      <div v-if="insights.catalogLoading && !insights.catalog.length" class="space-y-1.5">
        <div v-for="i in 8" :key="i" class="h-9 animate-pulse rounded-lg bg-slate-100 dark:bg-slate-800/60" />
      </div>

      <p v-else-if="!filtered.length" class="py-8 text-center text-sm text-slate-500">
        No hay productos que coincidan.
      </p>

      <section v-for="[category, items] in grouped" v-else :key="category" class="mb-5">
        <h2 class="mb-1.5 text-[11px] font-semibold tracking-wider text-slate-400 uppercase dark:text-slate-500">
          {{ category }} · {{ items.length }}
        </h2>
        <ul class="divide-y divide-slate-100 overflow-hidden rounded-xl border border-slate-200 dark:divide-slate-800 dark:border-slate-800">
          <li
            v-for="item in items"
            :key="item.id"
            class="flex items-center gap-2 bg-white px-3 py-2 dark:bg-slate-900"
            :class="!item.active && 'opacity-55'"
          >
            <template v-if="editingId === item.id">
              <input
                v-model="draft.name"
                maxlength="80"
                aria-label="Nombre"
                class="min-w-0 flex-1 rounded-md border border-slate-200 px-2 py-1 text-sm dark:border-slate-700 dark:bg-slate-800"
              />
              <input
                v-model="draft.category"
                aria-label="Categoría"
                class="w-28 rounded-md border border-slate-200 px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
              />
              <input
                v-model="draft.aliases"
                placeholder="alias, alias"
                aria-label="Alias"
                class="w-40 rounded-md border border-slate-200 px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
              />
              <button
                class="rounded-md p-1.5 text-emerald-600 hover:bg-emerald-500/10"
                aria-label="Guardar"
                @click="saveEdit"
              >
                <Save class="h-3.5 w-3.5" />
              </button>
              <button class="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800" aria-label="Cancelar" @click="cancelEdit">
                <X class="h-3.5 w-3.5" />
              </button>
            </template>

            <template v-else>
              <span class="min-w-0 flex-1 truncate text-sm text-slate-800 dark:text-slate-100">{{ item.name }}</span>
              <span
                v-if="!item.active"
                class="rounded-full bg-slate-500/10 px-1.5 py-0.5 text-[10px] text-slate-500 dark:text-slate-400"
              >
                inactivo
              </span>
              <span
                v-if="item.aliases.length"
                class="hidden max-w-[45%] truncate text-[11px] text-slate-400 sm:block"
                :title="item.aliases.join(', ')"
              >
                {{ item.aliases.slice(0, 3).join(', ') }}<template v-if="item.aliases.length > 3">…</template>
              </span>
              <button
                class="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-sky-600 dark:hover:bg-slate-800"
                :aria-label="`Editar ${item.name}`"
                @click="startEdit(item)"
              >
                <Pencil class="h-3.5 w-3.5" />
              </button>
              <button
                class="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-amber-600 dark:hover:bg-slate-800"
                :aria-label="item.active ? `Desactivar ${item.name}` : `Activar ${item.name}`"
                @click="toggleActive(item)"
              >
                <RotateCcw class="h-3.5 w-3.5" />
              </button>
              <button
                class="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-red-600 dark:hover:bg-slate-800"
                :aria-label="`Eliminar ${item.name}`"
                @click="remove(item)"
              >
                <Trash2 class="h-3.5 w-3.5" />
              </button>
            </template>
          </li>
        </ul>
      </section>

      <p class="mt-4 text-[11px] text-slate-400">
        <template v-if="!insights.enabled">
          <span class="text-amber-600 dark:text-amber-500">El análisis está apagado</span>: el catálogo se puede
          editar igual, pero la IA no lo está usando.
        </template>
        <template v-else>
          {{ insights.catalog.filter(c => c.active).length }} productos activos reconocibles por la IA.
        </template>
      </p>
    </div>
  </div>
</template>
