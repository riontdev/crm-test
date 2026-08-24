<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useTemplatesStore } from '@/stores/templates'
import type { Template } from '@/lib/api'
import { relativeTime } from '@/lib/utils'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import IconButton from '@/components/ui/IconButton.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'

type TemplateCategory = Template['category']

const store = useTemplatesStore()

// ---------- Filtros ----------
const categoryFilter = ref<'' | TemplateCategory>('')

const categories: Array<{ value: '' | TemplateCategory; label: string }> = [
  { value: '', label: 'Todas' },
  { value: 'general', label: 'General' },
  { value: 'marketing', label: 'Marketing' },
  { value: 'utility', label: 'Utilidad' },
  { value: 'soporte', label: 'Soporte' },
]

function setCategory(value: '' | TemplateCategory) {
  categoryFilter.value = value
  void store.fetchTemplates(searchInput.value.trim() || undefined, value || undefined)
}

// ---------- Búsqueda con debounce ----------
const searchInput = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

watch(searchInput, (q) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    void store.fetchTemplates(q.trim() || undefined, categoryFilter.value || undefined)
  }, 300)
})

onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})

const countLabel = computed(() => {
  const n = store.templates.length
  return n === 1 ? '1 plantilla' : `${n} plantillas`
})

// ---------- Variables {{N}} ----------
interface Segment {
  text: string
  isVar: boolean
}

const VAR_RE = /\{\{(\d+)\}\}/g

function renderSegments(content: string): Segment[] {
  const segments: Segment[] = []
  let last = 0
  for (const m of content.matchAll(VAR_RE)) {
    const idx = m.index ?? 0
    if (idx > last) segments.push({ text: content.slice(last, idx), isVar: false })
    segments.push({ text: m[0], isVar: true })
    last = idx + m[0].length
  }
  if (last < content.length) segments.push({ text: content.slice(last), isVar: false })
  return segments
}

function extractVars(content: string): string[] {
  const set = new Set<string>()
  for (const m of content.matchAll(VAR_RE)) set.add(m[0])
  return [...set].sort((a, b) => {
    const na = Number(a.replace(/\D/g, '')) || 0
    const nb = Number(b.replace(/\D/g, '')) || 0
    return na - nb
  })
}

// ---------- Categorías ----------
const CATEGORY_META: Record<
  TemplateCategory,
  { label: string; variant: 'default' | 'warning' | 'info' | 'success' }
> = {
  marketing: { label: 'Marketing', variant: 'warning' },
  utility: { label: 'Utilidad', variant: 'info' },
  soporte: { label: 'Soporte', variant: 'success' },
  general: { label: 'General', variant: 'default' },
}

function categoryMeta(category: string) {
  return CATEGORY_META[category as TemplateCategory] ?? CATEGORY_META.general
}

function langChip(lang: string): string {
  return (lang || '').slice(0, 2).toUpperCase()
}

// ---------- Modal crear / editar ----------
const modalOpen = ref(false)
const editing = ref<Template | null>(null)
const isEdit = computed(() => editing.value !== null)

const form = reactive<{
  name: string
  category: TemplateCategory
  language: string
  content: string
}>({
  name: '',
  category: 'general',
  language: 'es',
  content: '',
})

const detectedVars = computed(() => extractVars(form.content))
const formError = ref<string | null>(null)
const saving = ref(false)

function openCreate() {
  editing.value = null
  form.name = ''
  form.category = 'general'
  form.language = 'es'
  form.content = ''
  formError.value = null
  modalOpen.value = true
}

function openEdit(t: Template) {
  editing.value = t
  form.name = t.name
  form.category = t.category
  form.language = t.language || 'es'
  form.content = t.content
  formError.value = null
  modalOpen.value = true
}

function closeModal() {
  if (saving.value) return
  modalOpen.value = false
}

async function handleSubmit() {
  if (saving.value) return
  formError.value = null

  const name = form.name.trim()
  const content = form.content

  if (!name) {
    formError.value = 'El nombre es obligatorio'
    return
  }
  if (!content.trim()) {
    formError.value = 'El contenido es obligatorio'
    return
  }

  saving.value = true

  const err = isEdit.value && editing.value
    ? await store.updateTemplate(editing.value.id, {
        name,
        category: form.category,
        language: form.language,
        content,
      })
    : await store.createTemplate({
        name,
        category: form.category,
        language: form.language,
        content,
      })

  saving.value = false

  if (err === null) {
    modalOpen.value = false
  } else {
    formError.value = err
  }
}

// ---------- Eliminar ----------
const templateToDelete = ref<Template | null>(null)

const deleteConfirmMessage = computed(() =>
  templateToDelete.value
    ? `¿Eliminar "${templateToDelete.value.name}"? Esta acción no se puede deshacer.`
    : '',
)

async function handleDelete() {
  const t = templateToDelete.value
  templateToDelete.value = null
  if (!t) return
  await store.deleteTemplate(t.id)
}

function onKeydown(e: KeyboardEvent) {
  if (modalOpen.value && e.key === 'Escape') closeModal()
}

onMounted(() => {
  if (!store.loadedOnce) void store.fetchTemplates()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
})

const inputClasses =
  'mt-1 w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm text-slate-800 transition-all duration-200 placeholder:text-slate-400 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500'

const labelClasses =
  'block text-xs font-semibold uppercase tracking-[0.05em] text-slate-500 dark:text-slate-400'
</script>

<template>
  <div class="h-full overflow-y-auto">
    <div class="px-6 pb-10 pt-6">
      <!-- Header -->
      <header class="flex flex-wrap items-start gap-4">
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold tracking-[-0.01em] text-slate-900 dark:text-white">
            Plantillas
          </h1>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
            Mensajes rápidos reutilizables para responder al instante.
          </p>
        </div>
        <Button size="sm" class="ml-auto" @click="openCreate">
          <span class="material-symbols-outlined text-base" aria-hidden="true">add</span>
          Nueva plantilla
        </Button>
      </header>

      <!-- Toolbar -->
      <div class="mt-4 flex flex-wrap items-center gap-2">
        <button
          v-for="cat in categories"
          :key="cat.label"
          type="button"
          :class="[
            'rounded-full border px-3 py-1 text-xs font-medium whitespace-nowrap transition-colors',
            categoryFilter === cat.value
              ? 'border-slate-900 bg-slate-900 text-white dark:border-white dark:bg-white dark:text-slate-900'
              : 'border-slate-200 bg-transparent text-slate-600 hover:border-slate-400 dark:border-slate-700 dark:text-slate-300',
          ]"
          :aria-pressed="categoryFilter === cat.value"
          @click="setCategory(cat.value)"
        >
          {{ cat.label }}
        </button>

        <div class="relative w-64">
          <span
            class="material-symbols-outlined absolute top-1/2 left-3 -translate-y-1/2 text-slate-400"
            aria-hidden="true"
            >search</span
          >
          <input
            v-model="searchInput"
            type="text"
            placeholder="Buscar plantillas..."
            aria-label="Buscar plantillas"
            class="w-full rounded-lg bg-slate-100 py-2 pr-3 pl-9 text-sm text-slate-900 outline-none transition-shadow placeholder:text-slate-400 focus-visible:ring-2 focus-visible:ring-sky-400/60 dark:bg-slate-800 dark:text-slate-100"
          />
        </div>

        <p class="ml-auto text-xs text-slate-400 dark:text-slate-500">
          {{ countLabel }}
        </p>
      </div>

      <!-- Loading -->
      <div
        v-if="store.loading && !store.loadedOnce"
        class="mt-4 grid gap-4 lg:grid-cols-2"
        aria-hidden="true"
      >
        <div
          v-for="i in 4"
          :key="i"
          class="flex h-32 flex-col justify-center rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
        >
          <Skeleton :lines="3" />
        </div>
      </div>

      <!-- Error -->
      <EmptyState
        v-else-if="store.error"
        icon="cloud_off"
        title="No se pudieron cargar las plantillas"
        :description="store.error"
      >
        <template #action>
          <Button size="sm" @click="setCategory(categoryFilter)">Reintentar</Button>
        </template>
      </EmptyState>

      <!-- Vacío -->
      <EmptyState
        v-else-if="store.templates.length === 0"
        icon="description"
        title="Sin plantillas"
        description="Creá tu primera plantilla para responder más rápido."
      >
        <template #action>
          <Button size="sm" @click="openCreate">+ Nueva plantilla</Button>
        </template>
      </EmptyState>

      <!-- Grilla de plantillas -->
      <section
        v-else
        class="mt-4 grid items-stretch gap-4 transition-opacity duration-200 lg:grid-cols-2"
        :class="store.loading ? 'opacity-60' : 'opacity-100'"
      >
        <article
          v-for="t in store.templates"
          :key="t.id"
          class="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-all duration-200 hover:shadow-md dark:border-slate-800 dark:bg-[#101828]"
        >
          <!-- Nombre + categoría + idioma -->
          <div class="flex items-center gap-2">
            <h3 class="min-w-0 truncate text-sm font-semibold text-slate-900 dark:text-white">
              {{ t.name }}
            </h3>
            <Badge :variant="categoryMeta(t.category).variant">
              {{ categoryMeta(t.category).label }}
            </Badge>
            <span
              class="ml-auto shrink-0 rounded border border-slate-200 px-1.5 py-0.5 text-[10px] uppercase text-slate-400 dark:border-slate-700 dark:text-slate-400"
              >{{ langChip(t.language) }}</span
            >
          </div>

          <!-- Contenido con variables resaltadas -->
          <p
            class="line-clamp-3 text-sm whitespace-pre-wrap text-slate-600 dark:text-slate-300"
          ><template
              v-for="(seg, i) in renderSegments(t.content)"
              :key="i"
            ><mark
                v-if="seg.isVar"
                class="rounded bg-sky-500/15 px-1 font-mono text-xs text-sky-700 dark:text-sky-400"
                >{{ seg.text }}</mark
              ><template v-else>{{ seg.text }}</template></template></p>

          <!-- Footer -->
          <div
            class="mt-auto flex items-center gap-1 border-t border-slate-100 pt-2 dark:border-slate-800"
          >
            <p class="text-[11px] text-slate-400 dark:text-slate-500">
              Actualizada {{ relativeTime(t.updated_at) }}
            </p>
            <div class="ml-auto flex items-center gap-1">
              <IconButton
                icon="edit"
                size="sm"
                title="Editar plantilla"
                @click="openEdit(t)"
              />
              <IconButton
                icon="delete"
                size="sm"
                title="Eliminar plantilla"
                @click="templateToDelete = t"
              />
            </div>
          </div>
        </article>
      </section>
    </div>

    <!-- Modal crear / editar -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0"
        leave-active-class="transition duration-150 ease-in"
        leave-to-class="opacity-0"
      >
        <div
          v-if="modalOpen"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          :aria-label="isEdit ? 'Editar plantilla' : 'Nueva plantilla'"
          @click.self="closeModal"
        >
          <div
            class="w-full max-w-lg rounded-2xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-700 dark:bg-[#101828]"
          >
            <h2 class="text-lg font-semibold text-slate-900 dark:text-slate-100">
              {{ isEdit ? 'Editar plantilla' : 'Nueva plantilla' }}
            </h2>

            <form class="mt-5 space-y-4" @submit.prevent="handleSubmit">
              <div>
                <label for="tpl-name" :class="labelClasses">Nombre</label>
                <input
                  id="tpl-name"
                  v-model="form.name"
                  type="text"
                  autocomplete="off"
                  placeholder="Saludo inicial"
                  :class="inputClasses"
                />
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label for="tpl-category" :class="labelClasses">Categoría</label>
                  <select id="tpl-category" v-model="form.category" :class="inputClasses">
                    <option value="general">General</option>
                    <option value="marketing">Marketing</option>
                    <option value="utility">Utilidad</option>
                    <option value="soporte">Soporte</option>
                  </select>
                </div>
                <div>
                  <label for="tpl-language" :class="labelClasses">Idioma</label>
                  <select id="tpl-language" v-model="form.language" :class="inputClasses">
                    <option value="es">Español</option>
                    <option value="en">Inglés</option>
                    <option value="pt">Portugués</option>
                  </select>
                </div>
              </div>

              <div>
                <label for="tpl-content" :class="labelClasses">Contenido</label>
                <textarea
                  id="tpl-content"
                  v-model="form.content"
                  rows="5"
                  placeholder="¡Hola Carlos! Gracias por escribirnos..."
                  :class="inputClasses"
                />
                <p v-pre class="mt-1 text-[11px] text-slate-400 dark:text-slate-500">
                  Usá {{1}}, {{2}}... para variables que completarás al enviar
                </p>
                <div
                  v-if="detectedVars.length > 0"
                  class="mt-2 flex flex-wrap gap-1.5"
                >
                  <span
                    v-for="v in detectedVars"
                    :key="v"
                    class="rounded bg-sky-500/15 px-1 font-mono text-[10px] text-sky-700 dark:text-sky-400"
                    >{{ v }}</span
                  >
                </div>
              </div>

              <p v-if="formError" class="text-xs text-red-500" role="alert">{{ formError }}</p>

              <div class="flex justify-end gap-2 pt-2">
                <Button variant="ghost" @click="closeModal">Cancelar</Button>
                <Button type="submit" :loading="saving">Guardar</Button>
              </div>
            </form>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Confirmar eliminación -->
    <ConfirmDialog
      :show="templateToDelete !== null"
      title="Eliminar plantilla"
      :message="deleteConfirmMessage"
      confirm-text="Eliminar"
      cancel-text="Cancelar"
      variant="danger"
      @confirm="handleDelete"
      @cancel="templateToDelete = null"
    />
  </div>
</template>
