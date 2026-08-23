<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type ConversationDetail } from '@/lib/api'
import { relativeTime } from '@/lib/utils'
import { useUiStore } from '@/stores/ui'
import Avatar from '@/components/ui/Avatar.vue'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'

interface Props {
  conversation: ConversationDetail | null
}

const props = defineProps<Props>()

const emit = defineEmits<{ close: []; archived: [] }>()

const ui = useUiStore()

const showArchiveConfirm = ref(false)
const archiving = ref(false)
const archiveError = ref<string | null>(null)

const notesDraft = ref('')
const notesDirty = ref(false)
const savingNotes = ref(false)

watch(
  () => props.conversation?.contact?.id,
  () => {
    notesDraft.value = props.conversation?.contact?.notes ?? ''
    notesDirty.value = false
  },
  { immediate: true },
)

function onNotesInput() {
  notesDirty.value = true
}

async function saveNotes() {
  const contactId = props.conversation?.contact?.id
  if (!contactId || !notesDirty.value) return
  savingNotes.value = true
  try {
    await api.updateContactNotes(contactId, notesDraft.value)
    if (props.conversation?.contact) {
      props.conversation.contact.notes = notesDraft.value
    }
    notesDirty.value = false
    ui.success('Notas guardadas')
  } catch (e: any) {
    ui.error(e.message || 'No se pudieron guardar las notas')
  } finally {
    savingNotes.value = false
  }
}

watch(
  () => props.conversation?.id,
  () => {
    showArchiveConfirm.value = false
    archiveError.value = null
  },
)

const contact = computed(() => props.conversation?.contact)
const isArchived = computed(() => props.conversation?.status === 'archived')

const messageCount = computed(() => props.conversation?.messages?.length ?? 0)

const infoRows = computed(() => {
  const rows: Array<{ icon: string; label: string; value: string }> = []
  if (contact.value?.email) rows.push({ icon: 'mail', label: 'Email', value: contact.value.email })
  if (contact.value?.phone) rows.push({ icon: 'phone', label: 'Teléfono', value: contact.value.phone })
  return rows
})

async function handleArchive() {
  if (!props.conversation) return
  archiving.value = true
  archiveError.value = null
  try {
    const nextStatus = isArchived.value ? 'active' : 'archived'
    await api.updateConversation(props.conversation.id, { status: nextStatus })
    showArchiveConfirm.value = false
    emit('archived')
  } catch (e: any) {
    archiveError.value = e.message || 'No se pudo actualizar la conversación'
  } finally {
    archiving.value = false
  }
}
</script>

<template>
  <div v-if="conversation" class="flex h-full flex-col overflow-y-auto">
    <!-- Header -->
    <div class="flex items-start justify-between gap-2 px-5 pt-5">
      <h3 class="text-sm font-semibold text-slate-700 dark:text-slate-200">Detalles del contacto</h3>
      <button
        type="button"
        class="text-slate-400 transition-colors hover:text-slate-600 dark:hover:text-slate-300"
        aria-label="Cerrar panel"
        @click="emit('close')"
      >
        <span class="material-symbols-outlined text-lg" aria-hidden="true">close</span>
      </button>
    </div>

    <!-- Identidad -->
    <div class="flex flex-col items-center gap-2 px-5 pt-3 pb-4 text-center">
      <Avatar size="xl" :name="contact?.name || contact?.phone || '?'" :src="contact?.avatar_url" />
      <div>
        <p class="text-base font-semibold text-slate-800 dark:text-slate-100">
          {{ contact?.name || 'Desconocido' }}
        </p>
        <p v-if="contact?.company" class="mt-0.5 text-xs text-slate-400 dark:text-slate-500">
          {{ contact.company }}
        </p>
      </div>
      <ChannelBadge :channel="conversation.channel" />
      <Badge :variant="isArchived ? 'warning' : 'success'" size="sm">
        {{ isArchived ? 'Archivada' : 'Activa' }}
      </Badge>
    </div>

    <div class="space-y-4 px-5 pb-6">
      <!-- Información de contacto -->
      <section v-if="infoRows.length > 0">
        <h4 class="mb-2 text-[11px] font-semibold tracking-wider text-slate-400 uppercase dark:text-slate-500">
          Información de contacto
        </h4>
        <div class="space-y-1.5 rounded-xl border border-slate-100 bg-slate-50/60 p-3 dark:border-slate-800 dark:bg-slate-800/40">
          <div v-for="row in infoRows" :key="row.label" class="flex items-center gap-3">
            <span class="material-symbols-outlined text-base text-slate-400" aria-hidden="true">{{ row.icon }}</span>
            <div class="min-w-0">
              <p class="text-[10px] tracking-wide text-slate-400 uppercase dark:text-slate-500">{{ row.label }}</p>
              <p class="truncate text-sm text-slate-700 dark:text-slate-200">{{ row.value }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- Detalles CRM -->
      <section v-if="(contact?.tags && contact.tags.length > 0)">
        <h4 class="mb-2 text-[11px] font-semibold tracking-wider text-slate-400 uppercase dark:text-slate-500">
          Detalles CRM
        </h4>
        <div class="flex flex-wrap gap-1.5">
          <Badge v-for="tag in contact?.tags" :key="tag" variant="accent" size="sm">{{ tag }}</Badge>
        </div>
      </section>

      <!-- Info de la conversación -->
      <section>
        <h4 class="mb-2 text-[11px] font-semibold tracking-wider text-slate-400 uppercase dark:text-slate-500">
          Conversación
        </h4>
        <dl class="space-y-1.5 rounded-xl border border-slate-100 bg-slate-50/60 p-3 text-sm dark:border-slate-800 dark:bg-slate-800/40">
          <div class="flex items-center justify-between gap-2">
            <dt class="text-xs text-slate-400 dark:text-slate-500">Mensajes</dt>
            <dd class="font-medium text-slate-700 dark:text-slate-200">{{ messageCount }}</dd>
          </div>
          <div class="flex items-center justify-between gap-2">
            <dt class="text-xs text-slate-400 dark:text-slate-500">Último mensaje</dt>
            <dd class="font-medium text-slate-700 dark:text-slate-200">
              {{ relativeTime(conversation.last_inbound_at || conversation.updated_at) }}
            </dd>
          </div>
          <div class="flex items-center justify-between gap-2">
            <dt class="text-xs text-slate-400 dark:text-slate-500">Creada</dt>
            <dd class="font-medium text-slate-700 dark:text-slate-200">
              {{ new Date(conversation.created_at).toLocaleDateString('es') }}
            </dd>
          </div>
        </dl>
      </section>

      <!-- Notas -->
      <section>
        <div class="mb-2 flex items-center justify-between gap-2">
          <h4 class="text-[11px] font-semibold tracking-wider text-slate-400 uppercase dark:text-slate-500">
            Notas
          </h4>
          <button
            v-if="notesDirty"
            type="button"
            class="flex items-center gap-1 text-xs font-medium text-sky-600 transition-colors hover:text-sky-700 dark:text-sky-400"
            :disabled="savingNotes"
            @click="saveNotes"
          >
            <span
              v-if="savingNotes"
              class="h-3 w-3 animate-spin rounded-full border-2 border-current/30 border-t-current"
              aria-hidden="true"
            />
            Guardar
          </button>
        </div>
        <textarea
          v-model="notesDraft"
          rows="4"
          placeholder="Agregar notas sobre este contacto..."
          class="w-full resize-none rounded-xl border border-slate-200 bg-slate-50/60 px-3 py-2 text-sm text-slate-700 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-sky-400 dark:border-slate-800 dark:bg-slate-800/40 dark:text-slate-200 dark:placeholder:text-slate-500"
          @input="onNotesInput"
        ></textarea>
      </section>

      <!-- Acciones -->
      <section class="space-y-2 pt-1">
        <Button
          variant="secondary"
          size="md"
          class="w-full"
          :loading="archiving"
          @click="showArchiveConfirm = true"
        >
          {{ isArchived ? 'Desarchivar conversación' : 'Archivar conversación' }}
        </Button>
        <p v-if="archiveError" class="text-xs text-red-500" role="alert">{{ archiveError }}</p>
      </section>
    </div>

    <ConfirmDialog
      :show="showArchiveConfirm"
      title="Archivar conversación"
      message="La conversación saldrá de la bandeja activa. Podés desarchivarla más tarde desde este mismo panel."
      confirm-text="Archivar"
      variant="warning"
      @confirm="handleArchive"
      @cancel="showArchiveConfirm = false"
    />
  </div>
</template>
