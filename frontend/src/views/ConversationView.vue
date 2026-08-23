<script setup lang="ts">
import { onMounted, onUnmounted, ref, nextTick, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useMessagesStore } from '@/stores/messages'
import { api } from '@/lib/api'

const route = useRoute()
const store = useMessagesStore()

const newMessage = ref('')
const messagesContainer = ref<HTMLElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const filePreview = ref<string | null>(null)
const uploading = ref(false)

const conversationId = route.params.id as string

const accountId = computed(() => store.conversation?.zernio_account_id || '')

onMounted(async () => {
  await store.fetchConversation(conversationId)
  store.subscribe(conversationId)
  scrollToBottom()
})

onUnmounted(() => {
  store.unsubscribe()
  if (filePreview.value) URL.revokeObjectURL(filePreview.value)
})

watch(() => store.messages.length, () => {
  nextTick(scrollToBottom)
})

function scrollToBottom() {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

function onFileSelect(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  if (file.size > 10 * 1024 * 1024) {
    alert('El archivo no puede superar 10MB')
    return
  }

  selectedFile.value = file
  if (filePreview.value) URL.revokeObjectURL(filePreview.value)
  filePreview.value = URL.createObjectURL(file)
}

function removeFile() {
  selectedFile.value = null
  if (filePreview.value) URL.revokeObjectURL(filePreview.value)
  filePreview.value = null
  if (fileInput.value) fileInput.value.value = ''
}

async function handleSend() {
  if ((!newMessage.value.trim() && !selectedFile.value) || !accountId.value) return

  const text = newMessage.value
  newMessage.value = ''

  try {
    let attachmentUrl: string | undefined
    let attachmentType: string | undefined

    if (selectedFile.value) {
      uploading.value = true
      const uploadRes = await api.uploadFile(selectedFile.value)
      if (uploadRes.url) {
        attachmentUrl = uploadRes.url
        attachmentType = selectedFile.value.type.startsWith('image') ? 'image'
          : selectedFile.value.type.startsWith('video') ? 'video'
          : 'file'
      }
      removeFile()
      uploading.value = false
    }

    await store.sendMessage(conversationId, text || '', accountId.value, attachmentUrl, attachmentType)
  } catch {
    newMessage.value = text
    uploading.value = false
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

function isImage(type: string, url?: string): boolean {
  return type === 'image' || !!url?.match(/\.(jpg|jpeg|png|gif|webp)/i)
}

function isVideo(type: string, url?: string): boolean {
  return type === 'video' || !!url?.match(/\.(mp4|mov|webm)/i)
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
                  v-if="isImage(att.type, att.url)"
                  :src="att.url?.includes('zernio.com') ? `/api/media?url=${encodeURIComponent(att.url)}` : att.url"
                  alt="attachment"
                  class="rounded-lg max-w-full cursor-pointer hover:opacity-90 transition-opacity"
                  @click="openAttachment(att.url?.includes('zernio.com') ? `/api/media?url=${encodeURIComponent(att.url)}` : att.url)"
                />
                <video
                  v-else-if="isVideo(att.type, att.url)"
                  :src="att.url?.includes('zernio.com') ? `/api/media?url=${encodeURIComponent(att.url)}` : att.url"
                  controls
                  class="rounded-lg max-w-full"
                />
                <a
                  v-else
                  :href="att.url"
                  target="_blank"
                  class="text-xs underline opacity-70 hover:opacity-100 block"
                >
                  📎 Archivo adjunto
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
        <div v-if="store.sending || uploading" class="flex justify-end">
          <div class="bg-neutral-100 px-4 py-2 rounded-2xl rounded-br-md text-sm text-neutral-500">
            {{ uploading ? 'Subiendo archivo...' : 'Enviando...' }}
          </div>
        </div>
      </div>

      <!-- File preview -->
      <div v-if="selectedFile" class="border-t border-neutral-100 px-6 py-2 bg-neutral-50">
        <div class="flex items-center gap-3">
          <img
            v-if="filePreview && selectedFile.type.startsWith('image')"
            :src="filePreview"
            class="w-12 h-12 rounded object-cover"
          />
          <video
            v-else-if="filePreview && selectedFile.type.startsWith('video')"
            :src="filePreview"
            class="w-12 h-12 rounded object-cover"
          />
          <div v-else class="w-12 h-12 rounded bg-neutral-200 flex items-center justify-center text-lg">
            📎
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-xs font-medium truncate">{{ selectedFile.name }}</p>
            <p class="text-[10px] text-neutral-400">{{ (selectedFile.size / 1024).toFixed(0) }} KB</p>
          </div>
          <button @click="removeFile" class="text-neutral-400 hover:text-red-500 transition-colors p-1">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- Input -->
      <div class="border-t border-neutral-200 px-6 py-4">
        <div class="flex gap-3 items-end">
          <!-- Attach button -->
          <button
            @click="fileInput?.click()"
            :disabled="store.sending || uploading"
            class="shrink-0 w-10 h-10 flex items-center justify-center rounded-lg border border-neutral-200 hover:bg-neutral-50 transition-colors disabled:opacity-40"
            title="Adjuntar archivo"
          >
            <svg class="w-5 h-5 text-neutral-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"/>
            </svg>
          </button>

          <input
            ref="fileInput"
            type="file"
            accept="image/*,video/*,audio/*,.pdf"
            class="hidden"
            @change="onFileSelect"
          />

          <input
            v-model="newMessage"
            @keyup.enter="handleSend"
            placeholder="Escribe un mensaje..."
            class="flex-1 border border-neutral-200 rounded-lg px-4 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-black focus:border-transparent"
            :disabled="store.sending || uploading"
          />
          <button
            @click="handleSend"
            :disabled="(!newMessage.trim() && !selectedFile) || !accountId || store.sending || uploading"
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
