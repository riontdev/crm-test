<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessagesStore } from '@/stores/messages'
import { useConversationsStore } from '@/stores/conversations'
import { api } from '@/lib/api'
import { formatDayLabel } from '@/lib/utils'
import type { Message } from '@/lib/api'
import Avatar from '@/components/ui/Avatar.vue'
import Button from '@/components/ui/Button.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import IconButton from '@/components/ui/IconButton.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import ContactPanel from '@/components/contacts/ContactPanel.vue'
import MessageBubble from '@/components/chat/MessageBubble.vue'
import DayDivider from '@/components/chat/DayDivider.vue'
import TypingIndicator from '@/components/chat/TypingIndicator.vue'
import QuickActions from '@/components/chat/QuickActions.vue'
import MessageComposer from '@/components/chat/MessageComposer.vue'

const route = useRoute()
const router = useRouter()
const store = useMessagesStore()
const conversationsStore = useConversationsStore()

const uploading = ref(false)
const showFab = ref(false)
const composerSeed = ref('')
const showPanel = ref(true)
const scrollContainer = ref<HTMLElement | null>(null)

const conversationId = computed(() => route.params.id as string)
const contactName = computed(
  () => store.conversation?.contact?.name || store.conversation?.contact?.phone || 'Desconocido',
)
const accountId = computed(() => store.conversation?.zernio_account_id || '')

// Ventana de 24h de WhatsApp: si el canal es whatsapp y el último mensaje
// entrante es más viejo que 24h (o no existe), WhatsApp rechaza mensajes libres.
const WA_WINDOW_MS = 24 * 60 * 60 * 1000
const waWindowClosed = computed(() => {
  const conv = store.conversation
  if (!conv || conv.channel !== 'whatsapp') return false
  if (!conv.last_inbound_at) return true
  return Date.now() - new Date(conv.last_inbound_at).getTime() > WA_WINDOW_MS
})

interface MessageGroup {
  label: string
  items: Message[]
}

const groupedMessages = computed<MessageGroup[]>(() => {
  const groups: MessageGroup[] = []
  let currentLabel = ''
  for (const msg of store.messages) {
    const label = formatDayLabel(msg.sent_at || msg.created_at)
    if (label !== currentLabel) {
      groups.push({ label, items: [msg] })
      currentLabel = label
    } else {
      groups[groups.length - 1]?.items.push(msg)
    }
  }
  return groups
})

async function load() {
  if (!conversationId.value) return
  await store.fetchConversation(conversationId.value)
}

function scrollToBottom(smooth = false) {
  const el = scrollContainer.value
  if (!el) return
  el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' })
}

function onScroll() {
  const el = scrollContainer.value
  if (!el) return
  const distance = el.scrollHeight - el.scrollTop - el.clientHeight
  showFab.value = distance > 200
}

onMounted(async () => {
  await load()
  if (conversationId.value) store.subscribe(conversationId.value)
  nextTick(() => scrollToBottom())
})

onUnmounted(() => {
  store.unsubscribe()
})

watch(
  conversationId,
  async (id, oldId) => {
    if (id && id !== oldId) {
      await load()
      store.subscribe(id)
      nextTick(() => scrollToBottom())
    }
  },
)

watch(
  () => store.messages.length,
  () => {
    nextTick(() => scrollToBottom())
  },
)

function onQuickAction(action: { icon: string; label: string }) {
  composerSeed.value = `${action.label}: `
}

async function onArchived() {
  conversationsStore.fetchConversations()
  router.push('/inbox')
}

async function handleSend({ text, file }: { text: string; file: File | null }) {
  try {
    let attachmentUrl: string | undefined
    let attachmentType: string | undefined

    if (file) {
      uploading.value = true
      const uploadRes = await api.uploadFile(file)
      if (uploadRes.url) {
        attachmentUrl = uploadRes.url
        attachmentType = file.type.startsWith('image')
          ? 'image'
          : file.type.startsWith('video')
            ? 'video'
            : file.type.startsWith('audio')
              ? 'audio'
              : 'file'
      }
      uploading.value = false
    }

    await store.sendMessage(conversationId.value, text, accountId.value, attachmentUrl, attachmentType)
  } catch {
    uploading.value = false
  }
}
</script>

<template>
  <div class="flex h-full">
    <!-- Columna de chat -->
    <div class="relative flex min-w-0 flex-1 flex-col">
      <!-- Header -->
      <header
        v-if="store.conversation"
        class="flex h-14 shrink-0 items-center gap-3 border-b border-slate-200 bg-white px-4 dark:border-slate-800 dark:bg-[#101828]"
      >
        <IconButton
          icon="arrow_back"
          class="md:hidden"
          aria-label="Volver a la bandeja"
          @click="router.push('/inbox')"
        />
        <Avatar
          size="md"
          :name="contactName"
          :src="store.conversation.contact?.avatar_url"
        />
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-semibold text-slate-800 dark:text-slate-100">
            {{ contactName }}
          </p>
          <div class="mt-0.5 flex items-center gap-2">
            <ChannelBadge :channel="store.conversation.channel" />
            <span class="truncate text-[11px] text-slate-400 dark:text-slate-500">
              {{
                store.conversation.unread_count > 0
                  ? `${store.conversation.unread_count} sin leer`
                  : 'Conversación activa'
              }}
            </span>
          </div>
        </div>
        <IconButton
          icon="contacts"
          class="hidden xl:inline-flex"
          :aria-label="showPanel ? 'Ocultar panel de contacto' : 'Mostrar panel de contacto'"
          @click="showPanel = !showPanel"
        />
        <IconButton icon="more_vert" aria-label="Más opciones" />
      </header>
      <!-- Header fallback mientras carga -->
      <div
        v-else
        class="flex h-14 shrink-0 items-center gap-3 border-b border-slate-200 bg-white px-4 dark:border-slate-800 dark:bg-[#101828]"
      >
        <IconButton
          icon="arrow_back"
          class="md:hidden"
          aria-label="Volver a la bandeja"
          @click="router.push('/inbox')"
        />
        <div class="h-10 w-10 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
        <Skeleton :lines="1" class="w-40" />
      </div>

      <!-- Mensajes -->
      <div class="relative min-h-0 flex-1">
        <div
          ref="scrollContainer"
          class="absolute inset-0 overflow-y-auto px-4 py-4 md:px-6"
          @scroll.passive="onScroll"
        >
          <!-- Cargando -->
          <div v-if="store.loading" class="space-y-3">
            <template v-for="i in 5" :key="i">
              <div :class="['flex', i % 2 === 0 ? 'justify-end' : 'justify-start']">
                <div
                  :class="[
                    'rounded-2xl p-3',
                    i % 2 === 0 ? 'bg-sky-400/20' : 'bg-slate-100 dark:bg-slate-800',
                    i % 3 === 0 ? 'w-2/3' : 'w-3/4',
                  ]"
                >
                  <Skeleton :lines="1" />
                </div>
              </div>
            </template>
          </div>

          <!-- Error -->
          <EmptyState
            v-else-if="store.error && !store.loading"
            icon="cloud_off"
            title="No se pudieron cargar los mensajes"
            :description="store.error ?? ''"
          >
            <template #action>
              <Button variant="secondary" size="sm" @click="load">Reintentar</Button>
            </template>
          </EmptyState>

          <!-- Vacío -->
          <EmptyState
            v-else-if="store.messages.length === 0"
            icon="forum"
            title="No hay mensajes aún"
            description="Escribí el primer mensaje para comenzar la conversación."
          />

          <!-- Agrupados por día -->
          <template v-else>
            <template v-for="group in groupedMessages" :key="group.label">
              <DayDivider :label="group.label" />
              <div class="space-y-2">
                <MessageBubble
                  v-for="msg in group.items"
                  :key="msg.id"
                  :message="msg"
                  :channel="store.conversation?.channel"
                />
              </div>
            </template>
          </template>
        </div>

        <!-- FAB scroll al final -->
        <button
          v-show="showFab && !store.loading"
          type="button"
          class="absolute right-4 bottom-4 z-10 flex h-10 w-10 items-center justify-center rounded-full border border-slate-200 bg-white shadow-lg transition-transform hover:scale-105 dark:border-slate-700 dark:bg-slate-800"
          aria-label="Ir al último mensaje"
          @click="scrollToBottom(true)"
        >
          <span class="material-symbols-outlined text-slate-600 dark:text-slate-300" aria-hidden="true">keyboard_arrow_down</span>
        </button>
      </div>

      <!-- Zona inferior -->
      <div class="shrink-0">
        <div class="hidden px-4 pt-2 sm:block">
          <QuickActions @select="onQuickAction" />
        </div>

        <!-- Aviso ventana de 24h WhatsApp vencida -->
        <div
          v-if="waWindowClosed && !store.loading"
          class="mx-4 mb-1 flex items-center gap-2 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
          role="status"
        >
          <span class="material-symbols-outlined text-base" aria-hidden="true">info</span>
          <span>
            La ventana de 24h de WhatsApp está vencida. Solo podés responder con una plantilla
            aprobada.
          </span>
        </div>

        <!-- Banner error envío -->
        <div
          v-if="store.error && store.lastFailed && !store.loading"
          :class="[
            'mx-4 mb-1 flex items-center justify-between gap-2 rounded-lg px-3 py-2 text-xs',
            store.errorCode === 'WINDOW_CLOSED'
              ? 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
              : 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400',
          ]"
          role="alert"
        >
          <span>{{ store.error }}</span>
          <button
            v-if="store.errorCode !== 'WINDOW_CLOSED'"
            type="button"
            class="shrink-0 font-medium underline hover:no-underline"
            @click="store.retrySend(conversationId, accountId)"
          >
            Reintentar
          </button>
        </div>

        <TypingIndicator v-if="store.sending || uploading" />

        <MessageComposer
          class="w-full"
          :disabled="!accountId"
          :sending="store.sending"
          :uploading="uploading"
          :account-id-missing="!accountId"
          :seed="composerSeed"
          @seed-applied="composerSeed = ''"
          @send="handleSend"
        />
      </div>
    </div>

    <!-- Panel contacto -->
    <aside
      v-if="showPanel && store.conversation"
      class="hidden w-[320px] shrink-0 border-l border-slate-200 bg-white xl:block dark:border-slate-800 dark:bg-[#101828]"
    >
      <ContactPanel
        :conversation="store.conversation"
        @close="showPanel = false"
        @archived="onArchived"
      />
    </aside>
  </div>
</template>
