<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  AlertTriangle,
  Check,
  History,
  Lock,
  LockOpen,
  Mic,
  Pencil,
  Plus,
  RotateCcw,
  Trash2,
  Undo2,
  X,
} from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import OrderBadge from '@/components/chat/OrderBadge.vue'
import { useInsightsStore } from '@/stores/insights'
import type { InsightIntent, Order, OrderStatus } from '@/lib/api'

interface Props {
  order: Order | null
  loading?: boolean
}

const props = withDefaults(defineProps<Props>(), { loading: false })

const insights = useInsightsStore()

const editing = ref(false)
const saving = ref(false)
const showRaw = ref(false)
const showRevisions = ref(false)

/** Transicion de pedido en curso (aplicar/reabrir): deshabilita los dos botones. */
const transitioning = computed(() =>
  insights.orderPendingIds.has(props.order?.conversation_id || ''),
)

/** El pedido nuevo del cliente que todavia no llego al vigente. */
const pending = computed(() =>
  props.order ? insights.pendingFor(props.order.conversation_id) : null,
)

/** Historial de revisiones. La [0] es la vigente, que ya se muestra arriba. */
const revisions = computed(() =>
  props.order ? insights.revisionsFor(props.order.conversation_id) : [],
)

/** Detalle de un analysis para el banner: solo los que el operador necesita. */
const pendingDetails = computed(() => {
  const d = pending.value?.detalles as Record<string, unknown> | undefined
  if (!d) return []
  return Object.entries(d)
    .filter(([, v]) => v != null && String(v).trim() !== '')
    .map(([k, v]) => `${k}: ${v}`)
})

const pendingWhen = computed(() => {
  if (!pending.value) return ''
  const mins = Math.round((Date.now() - new Date(pending.value.created_at).getTime()) / 60000)
  if (mins < 1) return 'recién'
  if (mins < 60) return `hace ${mins} min`
  const hs = Math.round(mins / 60)
  return hs < 24 ? `hace ${hs} h` : `hace ${Math.round(hs / 24)} d`
})

async function applyPending() {
  if (!props.order || transitioning.value) return
  await insights.acceptPending(props.order.conversation_id)
}

async function reopen() {
  if (!props.order || transitioning.value) return
  await insights.reopenOrder(props.order.conversation_id)
}

const draft = ref({
  resumen: '',
  intent: 'pedido' as InsightIntent,
  productos: [] as string[],
  cantidades: [] as number[],
  material: '',
  status: 'nuevo' as OrderStatus,
  needsReview: false,
})

/** Copia del pedido para editar sin mutar lo que se ve mientras se escribe. */
function loadDraft(o: Order) {
  draft.value = {
    resumen: o.resumen || '',
    intent: o.intent || 'otro',
    productos: [...(o.productos || [])],
    cantidades: [...(o.cantidades || [])],
    material: o.detalles?.material || '',
    status: o.status,
    needsReview: o.needs_review,
  }
}

watch(
  () => props.order,
  (o) => {
    if (o) loadDraft(o)
    editing.value = false
  },
  { immediate: true, deep: false },
)

const confidencePct = computed(() =>
  props.order?.confianza != null ? Math.round(props.order.confianza * 100) : null,
)

/** Confianza baja = el modelo no está seguro. Menos de 70% se marca. */
const lowConfidence = computed(() => (confidencePct.value ?? 100) < 70)

const statusOptions: { value: OrderStatus; label: string }[] = [
  { value: 'nuevo', label: 'Nuevo' },
  { value: 'confirmado', label: 'Confirmado' },
  { value: 'entregado', label: 'Entregado' },
  { value: 'descartado', label: 'Descartado' },
]

const intentOptions: { value: InsightIntent; label: string }[] = [
  { value: 'pedido', label: 'Pedido' },
  { value: 'info', label: 'Info' },
  { value: 'reclamo', label: 'Reclamo' },
  { value: 'otro', label: 'Otro' },
]

function addProduct() {
  draft.value.productos = [...draft.value.productos, '']
  draft.value.cantidades = [...draft.value.cantidades, 1]
}

function removeProduct(i: number) {
  draft.value.productos.splice(i, 1)
  draft.value.cantidades.splice(i, 1)
}

/** Compara nombres de producto sin que estorben mayusculas ni espacios. */
function mismoProducto(a: string, b: string) {
  return a.trim().toLowerCase() === b.trim().toLowerCase()
}

/**
 * Trae los productos de otra fuente (una versión vieja del pedido, un análisis
 * del historial, el pedido pendiente) al pedido que se está editando.
 *
 * Si el producto YA está en el pedido, SUMA la cantidad en vez de reemplazarla:
 * es la misma regla que aplica la IA con tipo_cantidad='delta' (ver MergeOrder
 * en el backend). Traer "3 llaveros" de una versión vieja a un pedido que ya
 * tiene 2 tiene que dar 5. Reemplazar acá sería el mismo bug que se acaba de
 * corregir en el consolidado: el pedido pierde lo que ya tenía y nadie lo ve.
 *
 * No guarda. Abre el formulario con el resultado para que el operador lo mire y
 * confirme: guardar sella el pedido (edited), y sellar es decisión del humano.
 */
function traerAlPedido(origen: { productos?: string[] | null; cantidades?: number[] | null }) {
  const productos = [...draft.value.productos]
  const cantidades = [...draft.value.cantidades]
  let agregados = 0

  ;(origen.productos || []).forEach((p, i) => {
    const nombre = (p || '').trim()
    if (!nombre) return
    const q = Number(origen.cantidades?.[i] ?? 0) || 0
    const pos = productos.findIndex((x) => mismoProducto(x, nombre))
    if (pos === -1) {
      productos.push(nombre)
      cantidades.push(q)
    } else {
      cantidades[pos] = (cantidades[pos] || 0) + q
    }
    agregados++
  })

  if (!agregados) return
  draft.value.productos = productos
  draft.value.cantidades = cantidades
  syncQuantities()
  editing.value = true
  showRevisions.value = false
  showRaw.value = false
}

/** Si hay más productos que cantidades, la IA no acertó las cantidades. */
function syncQuantities() {
  while (draft.value.cantidades.length < draft.value.productos.length) draft.value.cantidades.push(1)
  draft.value.cantidades = draft.value.cantidades.slice(0, draft.value.productos.length)
}

const canSave = computed(() => {
  if (!props.order) return false
  if (!draft.value.resumen.trim()) return false
  return draft.value.productos.every((p) => p.trim().length > 0)
})

async function save() {
  if (!props.order || !canSave.value) return
  saving.value = true
  const productos = draft.value.productos.map((p) => p.trim()).filter(Boolean)
  // El formulario solo edita el material, pero el pedido tiene medidas,
  // personalizacion, fecha de entrega y envio. Mandar solo {material} borraba
  // todo lo demas en cada guardado: el operador corrigia el material y sin
  // querer perdia la medida de 15 cm.
  const detalles: Record<string, string> = { ...(props.order.detalles || {}) }
  const material = draft.value.material.trim()
  if (material) detalles.material = material
  else delete detalles.material

  const ok = await insights.saveOrder(props.order.id, props.order.conversation_id, {
    resumen: draft.value.resumen.trim(),
    intent: draft.value.intent,
    productos,
    cantidades: draft.value.cantidades.slice(0, productos.length),
    detalles,
    status: draft.value.status,
    needs_review: draft.value.needsReview,
  })
  saving.value = false
  if (ok) editing.value = false
}

function cancel() {
  if (props.order) loadDraft(props.order)
  editing.value = false
}

const hasAudio = computed(() =>
  insights.analysisForConversation(props.order?.conversation_id || '').some((a) => a.asr_text),
)
</script>

<template>
  <section
    v-if="loading"
    class="rounded-xl border border-slate-200 dark:border-slate-800 p-4 animate-pulse"
    aria-busy="true"
  >
    <div class="h-4 w-1/3 rounded bg-slate-200 dark:bg-slate-800" />
    <div class="mt-3 h-3 w-3/4 rounded bg-slate-100 dark:bg-slate-800/60" />
  </section>

  <section
    v-else-if="!order"
    class="rounded-xl border border-dashed border-slate-300 dark:border-slate-700 p-4"
  >
    <p class="text-sm text-slate-500 dark:text-slate-400">
      Todavía no hay un pedido en este hilo.
    </p>
    <p class="mt-1 text-xs text-slate-400 dark:text-slate-500">
      Cuando el cliente pida algo, el resumen aparece acá.
    </p>
  </section>

  <section
    v-else
    class="rounded-xl border bg-white dark:bg-slate-900 p-4 space-y-3"
    :class="pending
      ? 'border-amber-400 dark:border-amber-600/70'
      : order.needs_review || lowConfidence
        ? 'border-amber-300 dark:border-amber-700/60'
        : 'border-slate-200 dark:border-slate-800'"
  >
    <!-- encabezado -->
    <header class="flex items-start justify-between gap-2">
      <div class="flex flex-wrap items-center gap-2">
        <OrderBadge :intent="order.intent" :needs-review="order.needs_review" />
        <span
          v-if="revisions.length > 1"
          class="inline-flex items-center gap-1 rounded-full bg-slate-500/10 px-2 py-0.5 text-[11px] font-medium tabular-nums text-slate-500 dark:bg-slate-400/10 dark:text-slate-400"
          :title="`Revisión ${order.revision} de ${revisions.length}`"
        >
          v{{ order.revision }}
        </span>
        <span
          v-if="order.edited"
          class="inline-flex items-center gap-1 rounded-full bg-slate-500/10 px-2 py-0.5 text-[11px] font-medium text-slate-600 dark:bg-slate-400/10 dark:text-slate-300"
          title="Editado a mano: la IA ya no lo modifica"
        >
          <Lock class="h-3 w-3" aria-hidden="true" /> Editado
        </span>
        <span
          v-if="hasAudio"
          class="inline-flex items-center gap-1 text-[11px] text-slate-400 dark:text-slate-500"
          title="El pedido se armó a partir de notas de voz"
        >
          <Mic class="h-3 w-3" aria-hidden="true" /> incluye audio
        </span>
      </div>

      <div class="flex items-center gap-1">
        <select
          v-if="!editing"
          :value="order.status"
          class="rounded-md border border-slate-200 bg-white px-1.5 py-1 text-[11px] text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
          :aria-label="`Estado del pedido: ${order.status}`"
          @change="insights.setOrderStatus(order.id, order.conversation_id, ($event.target as HTMLSelectElement).value as OrderStatus)"
        >
          <option v-for="s in statusOptions" :key="s.value" :value="s.value">{{ s.label }}</option>
        </select>
        <Button
          v-if="!editing && order.edited && !pending"
          variant="ghost"
          size="sm"
          aria-label="Reabrir pedido para que la IA lo vuelva a actualizar"
          title="Reabrir: la IA vuelve a actualizar este pedido"
          :disabled="transitioning"
          @click="reopen"
        >
          <LockOpen class="h-3.5 w-3.5" />
        </Button>
        <Button
          v-if="!editing"
          variant="ghost"
          size="sm"
          :aria-label="editing ? 'Cancelar edición' : 'Editar pedido'"
          @click="editing = true"
        >
          <Pencil class="h-3.5 w-3.5" />
        </Button>
      </div>
    </header>

    <!-- modo lectura -->
    <template v-if="!editing">
      <!--
        Pedido pendiente: el cliente pidio algo despues de que este pedido
        quedara fijado y la IA no lo pudo consolidar. Sin este cartel el
        operador mira un pedido viejo y no tiene ni idea de que hay algo sin
       entrar. Por eso va arriba de todo y en color de alarma, no como una nota
        al pie.
      -->
      <div
        v-if="pending"
        class="rounded-lg border border-amber-400 bg-amber-50 p-2.5 text-xs dark:border-amber-600/60 dark:bg-amber-950/30"
        role="status"
      >
        <p class="flex items-center gap-1.5 font-semibold text-amber-900 dark:text-amber-200">
          <Undo2 class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          El cliente pidió algo más {{ pendingWhen }}
        </p>
        <p v-if="pending.resumen" class="mt-1 text-amber-800 dark:text-amber-300/90">
          “{{ pending.resumen }}”
        </p>
        <ul v-if="pending.productos?.length" class="mt-1.5 flex flex-wrap gap-1">
          <li
            v-for="(p, i) in pending.productos"
            :key="`${p}-${i}`"
            class="rounded bg-amber-200/70 px-1.5 py-0.5 font-medium tabular-nums text-amber-900 dark:bg-amber-900/50 dark:text-amber-200"
          >
            <span v-if="pending.cantidades?.[i] != null" class="font-bold">{{ pending.cantidades[i] }}×</span>
            {{ p }}
          </li>
        </ul>
        <p v-if="pendingDetails.length" class="mt-1 text-[11px] text-amber-700 dark:text-amber-400/80">
          {{ pendingDetails.join(' · ') }}
        </p>

        <div class="mt-2 flex flex-wrap gap-1.5">
          <Button
            variant="primary"
            size="sm"
            :disabled="transitioning"
            @click="applyPending"
          >
            <RotateCcw v-if="transitioning" class="h-3.5 w-3.5 animate-spin" />
            <Check v-else class="h-3.5 w-3.5" />
            Aplicar como pedido vigente
          </Button>
          <Button
            variant="ghost"
            size="sm"
            :disabled="transitioning"
            title="Abrir la edición con estos productos ya sumados, para revisarlos antes de aplicar"
            @click="traerAlPedido(pending)"
          >
            <Plus class="h-3.5 w-3.5" />
            Traer al pedido
          </Button>
          <Button
            variant="ghost"
            size="sm"
            :disabled="transitioning"
            title="Dejar que la IA actualice este pedido, sin cambiar de revisión"
            @click="reopen"
          >
            <LockOpen class="h-3.5 w-3.5" />
            Reabrir este
          </Button>
        </div>
        <p class="mt-1.5 text-[10px] leading-relaxed text-amber-700/80 dark:text-amber-500/70">
          Aplicar guarda este pedido en el historial y crea la revisión siguiente.
          Traer los suma a la edición para que los revises antes de decidir.
          Reabrir deja que la IA lo actualice sobre esta misma revisión.
        </p>
      </div>

      <p class="text-sm leading-relaxed text-slate-800 dark:text-slate-100">{{ order.resumen }}</p>

      <ul v-if="order.productos.length" class="flex flex-wrap gap-1.5">
        <li
          v-for="(p, i) in order.productos"
          :key="`${p}-${i}`"
          class="inline-flex items-center gap-1.5 rounded-lg bg-indigo-500/10 px-2 py-1 text-xs font-medium text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300"
        >
          <span v-if="order.cantidades[i] != null" class="font-bold tabular-nums">{{ order.cantidades[i] }}×</span>
          {{ p }}
        </li>
      </ul>

      <dl v-if="order.detalles && Object.keys(order.detalles).length" class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
        <div v-for="(v, k) in order.detalles" :key="k" class="flex gap-1.5">
          <dt class="text-slate-400 dark:text-slate-500 capitalize">{{ k }}:</dt>
          <dd class="text-slate-700 dark:text-slate-200">{{ v }}</dd>
        </div>
      </dl>

      <footer class="flex flex-wrap items-center gap-3 border-t border-slate-100 pt-2 text-[11px] text-slate-400 dark:border-slate-800 dark:text-slate-500">
        <span v-if="confidencePct != null" :class="lowConfidence ? 'text-amber-600 dark:text-amber-500' : ''">
          {{ confidencePct }}% de confianza
        </span>
        <span v-if="order.model">{{ order.model }}</span>
        <span>{{ new Date(order.updated_at).toLocaleString('es', { dateStyle: 'short', timeStyle: 'short' }) }}</span>
        <div class="ml-auto flex items-center gap-3">
          <button
            v-if="revisions.length > 1"
            class="inline-flex items-center gap-1 underline-offset-2 hover:underline"
            @click="showRevisions = !showRevisions"
          >
            <History class="h-3 w-3" aria-hidden="true" />
            {{ showRevisions ? 'Ocultar' : 'Ver' }} versiones ({{ revisions.length }})
          </button>
          <button
            class="underline-offset-2 hover:underline"
            @click="showRaw = !showRaw"
          >
            {{ showRaw ? 'Ocultar' : 'Ver' }} historial
          </button>
        </div>
      </footer>

      <!--
        Timeline de revisiones. Se muestra solo si hay mas de una: con una sola
        el boton no tiene nada que decir y ocupa espacio en la pantalla mas
        usada del producto.
      -->
      <ol v-if="showRevisions && revisions.length > 1" class="space-y-1.5 border-t border-slate-100 pt-2 dark:border-slate-800">
        <li
          v-for="r in revisions"
          :key="r.id"
          class="rounded-lg p-2 text-xs"
          :class="r.is_current
            ? 'bg-indigo-500/10 ring-1 ring-indigo-500/30 dark:bg-indigo-500/10'
            : 'bg-slate-50 dark:bg-slate-800/50'"
        >
          <div class="flex items-center gap-2">
            <span class="font-semibold tabular-nums text-slate-600 dark:text-slate-300">v{{ r.revision }}</span>
            <span
              v-if="r.is_current"
              class="rounded-full bg-indigo-500/20 px-1.5 py-0.5 text-[10px] font-medium text-indigo-700 dark:text-indigo-300"
            >Vigente</span>
            <span
              v-if="r.edited"
              class="inline-flex items-center gap-0.5 text-[10px] text-slate-400"
              title="Editado a mano"
            >
              <Lock class="h-2.5 w-2.5" aria-hidden="true" /> editado
            </span>
            <button
              v-if="!r.is_current && r.productos?.length"
              class="ml-auto inline-flex items-center gap-1 rounded border border-slate-200 px-1.5 py-0.5 text-[10px] text-slate-500 hover:bg-white hover:text-indigo-600 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800"
              title="Sumar los productos de esta versión al pedido vigente y abrir la edición"
              @click="traerAlPedido(r)"
            >
              <Plus class="h-2.5 w-2.5" aria-hidden="true" /> Traer al pedido
            </button>
            <span v-else class="ml-auto text-[10px] text-slate-400">
              {{ new Date(r.created_at).toLocaleString('es', { dateStyle: 'short', timeStyle: 'short' }) }}
            </span>
          </div>
          <p v-if="r.resumen" class="mt-0.5 text-slate-600 dark:text-slate-300">{{ r.resumen }}</p>
          <p v-if="r.productos?.length" class="mt-0.5 text-[11px] tabular-nums text-slate-500 dark:text-slate-400">
            <span v-for="(p, i) in r.productos" :key="`${p}-${i}`">
              <span v-if="i > 0"> · </span><span v-if="r.cantidades?.[i] != null" class="font-semibold">{{ r.cantidades[i] }}×</span>{{ p }}
            </span>
          </p>
          <p v-if="!r.is_current && r.superseded_at" class="mt-0.5 text-[10px] text-slate-400">
            Reemplazada el {{ new Date(r.superseded_at).toLocaleString('es', { dateStyle: 'short', timeStyle: 'short' }) }}
          </p>
        </li>
      </ol>

      <p
        v-if="order.needs_review || lowConfidence"
        class="flex items-start gap-1.5 rounded-lg bg-amber-500/10 px-2.5 py-2 text-xs text-amber-800 dark:text-amber-300"
      >
        <AlertTriangle class="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <span>
          Revisalo antes de confirmar: la IA no está segura de este pedido.
          <template v-if="!order.edited">Si lo corregís, queda fijado y la IA no lo vuelve a tocar.</template>
        </span>
      </p>

      <ol v-if="showRaw" class="space-y-1.5 border-t border-slate-100 pt-2 dark:border-slate-800">
        <li
          v-for="a in insights.analysisForConversation(order.conversation_id)"
          :key="a.id"
          class="rounded-lg bg-slate-50 p-2 text-xs dark:bg-slate-800/50"
        >
          <div class="flex items-center gap-2">
            <OrderBadge :intent="a.intent" size="sm" />
            <span v-if="a.asr_text" class="inline-flex items-center gap-1 text-[10px] text-slate-400">
              <Mic class="h-3 w-3" aria-hidden="true" /> voz
            </span>
            <button
              v-if="a.intent === 'pedido' && a.productos?.length"
              class="ml-auto inline-flex items-center gap-1 rounded border border-slate-200 px-1.5 py-0.5 text-[10px] text-slate-500 hover:bg-white hover:text-indigo-600 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800"
              title="Sumar estos productos al pedido y abrir la edición"
              @click="traerAlPedido(a)"
            >
              <Plus class="h-2.5 w-2.5" aria-hidden="true" /> Traer al pedido
            </button>
            <span v-else class="ml-auto text-[10px] text-slate-400">{{ new Date(a.created_at).toLocaleTimeString('es', { hour: '2-digit', minute: '2-digit' }) }}</span>
          </div>
          <p class="mt-1 text-slate-600 dark:text-slate-300">{{ a.resumen }}</p>
          <p v-if="a.productos?.length" class="mt-0.5 text-[11px] tabular-nums text-slate-500 dark:text-slate-400">
            <span v-for="(p, i) in a.productos" :key="`${p}-${i}`">
              <span v-if="i > 0"> · </span><span v-if="a.cantidades?.[i] != null" class="font-semibold">{{ a.cantidades[i] }}×</span>{{ p }}
            </span>
          </p>
          <p v-if="a.asr_text" class="mt-1 text-[11px] italic text-slate-400">“{{ a.asr_text }}”</p>
          <p v-if="a.error" class="mt-1 text-[11px] text-red-500">{{ a.error }}</p>
        </li>
      </ol>
    </template>

    <!-- modo edición -->
    <form v-else class="space-y-3" @submit.prevent="save">
      <label class="block">
        <span class="text-xs font-medium text-slate-600 dark:text-slate-300">Resumen</span>
        <textarea
          v-model="draft.resumen"
          rows="3"
          maxlength="2000"
          class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2.5 py-2 text-sm dark:border-slate-700 dark:bg-slate-800"
        />
      </label>

      <div class="grid grid-cols-2 gap-2">
        <label class="block">
          <span class="text-xs font-medium text-slate-600 dark:text-slate-300">Intención</span>
          <select
            v-model="draft.intent"
            class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
          >
            <option v-for="i in intentOptions" :key="i.value" :value="i.value">{{ i.label }}</option>
          </select>
        </label>
        <label class="block">
          <span class="text-xs font-medium text-slate-600 dark:text-slate-300">Material</span>
          <input
            v-model="draft.material"
            maxlength="200"
            class="mt-1 w-full rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
          />
        </label>
      </div>

      <div>
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-600 dark:text-slate-300">Productos</span>
          <Button type="button" variant="ghost" size="sm" @click="addProduct">
            <Plus class="h-3.5 w-3.5" /> Agregar
          </Button>
        </div>
        <div v-for="(_, i) in draft.productos" :key="i" class="mt-1.5 flex items-center gap-1.5">
          <input
            v-model.number="draft.cantidades[i]"
            type="number"
            min="0"
            class="w-20 shrink-0 rounded-lg border border-slate-200 px-2 py-1.5 text-sm tabular-nums dark:border-slate-700 dark:bg-slate-800"
            :aria-label="`Cantidad del producto ${i + 1}`"
            @change="syncQuantities"
          />
          <input
            v-model="draft.productos[i]"
            maxlength="80"
            class="min-w-0 flex-1 rounded-lg border border-slate-200 px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-800"
            :aria-label="`Producto ${i + 1}`"
          />
          <button
            type="button"
            class="rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
            :aria-label="`Quitar producto ${i + 1}`"
            @click="removeProduct(i)"
          >
            <Trash2 class="h-3.5 w-3.5" />
          </button>
        </div>
        <p v-if="!draft.productos.length" class="mt-1 text-xs text-slate-400">
          Sin productos: el hilo fue una consulta, no un pedido.
        </p>
      </div>

      <label class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
        <input v-model="draft.needsReview" type="checkbox" class="rounded" />
        Marcar para revisar
      </label>

      <div class="flex items-center justify-between gap-2 border-t border-slate-100 pt-2 dark:border-slate-800">
        <p class="text-[11px] text-slate-400">Al guardar, la IA deja de tocar este pedido.</p>
        <div class="flex gap-1.5">
          <Button type="button" variant="ghost" size="sm" @click="cancel">
            <X class="h-3.5 w-3.5" /> Cancelar
          </Button>
          <Button type="submit" variant="primary" size="sm" :disabled="!canSave || saving">
            <Check v-if="!saving" class="h-3.5 w-3.5" />
            <RotateCcw v-else class="h-3.5 w-3.5 animate-spin" />
            Guardar
          </Button>
        </div>
      </div>
    </form>
  </section>
</template>
