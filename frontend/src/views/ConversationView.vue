<script setup lang="ts">
import { onMounted, ref, nextTick, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useMessagesStore } from '@/stores/messages'

const route = useRoute()
const store = useMessagesStore()

const newMessage = ref('')
const messagesContainer = ref<HTMLElement | null>(null)

const conversationId = route.params.id as string

const accountId = computed(() => store.conversation?.zernio_account_id || '')

onMounted(async () => {
  await store.fetchConversation(conversationId)
  scrollToBottom()
})

watch(() => store.messages.length, () => {
  nextTick(scrollToBottom)
})

function scrollToBottom() {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

async function handleSend() {
  if (!newMessage.value.trim() || !accountId.value) return

  const text = newMessage.value
  newMessage.value = ''

  try {
    await store.sendMessage(conversationId, text, accountId.value)
  } catch {
    newMessage.value = text
  }
}

const channelEmojis: Record<string, string> = {
  whatsapp: '💬',
  instagram: '📸',
  facebook: '👤',
}

function openAttachment(url?: string) {
  if (url) window.open(url, '_blank')
}
</script>

<template>
  <div class="flex h-[calc(100vh-53px)]">
    <!-- Messages area -->
    <div class="flex-1 flex flex-col">
      <!-- Header -->
      <div v-if="store.conversation" class="border-b border-neutral-200 px-6 py-3 flex items-center gap-3">
        <div class="w-8 h-8 rounded-full bg-neutral-200 flex items-center justify-center text-sm">
          {{ store.conversation.contact?.name?.charAt(0) || '?' }}
        </div>
        <div>
          <div class="font-medium text-sm">
            {{ store.conversation.contact?.name || store.conversation.contact?.phone || 'Desconocido' }}
          </div>
          <div class="text-xs text-neutral-500 flex items-center gap-1">
            <span>{{ channelEmojis[store.conversation.channel] || '📱' }}</span>
            <span class="capitalize">{{ store.conversation.channel }}</span>
          </div>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="store.loading" class="flex-1 flex items-center justify-center text-neutral-400 text-sm">
        Cargando mensajes...
      </div>

      <!-- Messages -->
      <div
        ref="messagesContainer"
        class="flex-1 overflow-y-auto px-6 py-4 space-y-3"
      >
        <div
          v-for="msg in store.messages"
          :key="msg.id"
          :class="[
            'flex',
            msg.direction === 'outgoing' ? 'justify-end' : 'justify-start'
          ]"
        >
          <div
            :class="[
              'max-w-xs lg:max-w-md px-4 py-2 rounded-2xl text-sm',
              msg.direction === 'outgoing'
                ? 'bg-black text-white rounded-br-md'
                : 'bg-neutral-100 text-black rounded-bl-md'
            ]"
          >
            <p v-if="msg.text" class="whitespace-pre-wrap break-words">{{ msg.text }}</p>
            <div v-if="msg.attachments && msg.attachments.length > 0" class="mt-1 space-y-1">
              <template v-for="(att, i) in msg.attachments" :key="i">
                <img
                  v-if="att.type === 'image' || att.url?.match(/\.(jpg|jpeg|png|gif|webp)/i)"
                  :src="`/api/media?url=${encodeURIComponent(att.url)}`"
                  alt="attachment"
                  class="rounded-lg max-w-full cursor-pointer hover:opacity-90 transition-opacity"
                  @click="openAttachment(`/api/media?url=${encodeURIComponent(att.url)}`)"
                />
                <a
                  v-else
                  :href="att.url"
                  target="_blank"
                  class="text-xs underline opacity-70 hover:opacity-100 block"
                >
                  Archivo adjunto
                </a>
              </template>
            </div>
            <div
              v-if="msg.sent_at"
              :class="[
                'text-[10px] mt-1',
                msg.direction === 'outgoing' ? 'text-neutral-400' : 'text-neutral-500'
              ]"
            >
              {{ new Date(msg.sent_at).toLocaleTimeString('es', { hour: '2-digit', minute: '2-digit' }) }}
            </div>
          </div>
        </div>

        <!-- Typing indicator -->
        <div v-if="store.sending" class="flex justify-end">
          <div class="bg-neutral-100 px-4 py-2 rounded-2xl rounded-br-md text-sm text-neutral-500">
            Enviando...
          </div>
        </div>
      </div>

      <!-- Input -->
      <div class="border-t border-neutral-200 px-6 py-4">
        <div class="flex gap-3">
          <input
            v-model="newMessage"
            @keyup.enter="handleSend"
            placeholder="Escribe un mensaje..."
            class="flex-1 border border-neutral-200 rounded-lg px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-black focus:border-transparent"
            :disabled="store.sending"
          />
          <button
            @click="handleSend"
            :disabled="!newMessage.trim() || !accountId || store.sending"
            class="bg-black text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-neutral-800 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Enviar
          </button>
        </div>
        <p v-if="store.conversation && !accountId" class="text-xs text-amber-600 mt-2">
          No hay account_id configurado para esta conversación.
        </p>
      </div>
    </div>
  </div>
</template>
