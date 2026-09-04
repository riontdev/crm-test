<script setup lang="ts">
import { computed } from 'vue'
import type { Message } from '@/lib/api'
import { cn, formatTime } from '@/lib/utils'

interface Props {
  message: Message
  channel?: string
}

const props = defineProps<Props>()

const isOwn = computed(() => props.message.direction === 'outgoing')

const CHANNEL_ICONS: Record<string, string> = {
  whatsapp: 'chat',
  instagram: 'photo_camera',
  facebook: 'thumb_up',
}

const IMAGE_RE = /\.(jpe?g|png|gif|webp)/i
const VIDEO_RE = /\.(mp4|webm|mov|m4v)/i
const AUDIO_RE = /\.(mp3|wav|ogg|m4a|aac)/i

function resolveUrl(url: string): string {
  return url.includes('zernio.com') ? `/api/media?url=${encodeURIComponent(url)}` : url
}

function isImage(att: { type: string; url: string }): boolean {
  return att.type.startsWith('image') || IMAGE_RE.test(att.url)
}

function isVideo(att: { type: string; url: string }): boolean {
  return !isImage(att) && (att.type.startsWith('video') || VIDEO_RE.test(att.url))
}

function isAudio(att: { type: string; url: string }): boolean {
  return att.type.startsWith('audio') || AUDIO_RE.test(att.url)
}

function openAttachment(url: string): void {
  window.open(resolveUrl(url), '_blank', 'noopener')
}

const statusMeta = computed(() => {
  switch (props.message.status) {
    case 'read':
      return { icon: 'done_all', classes: 'text-sky-100 dark:text-[#7DD3FC]', title: 'Leído' }
    case 'delivered':
      return { icon: 'done_all', classes: 'opacity-60', title: 'Entregado' }
    case 'failed':
      return { icon: 'error', classes: 'text-red-300', title: 'Error' }
    case 'sending':
      return { icon: 'schedule', classes: 'opacity-70', title: 'Enviando…' }
    default:
      return { icon: 'check', classes: 'opacity-60', title: 'Enviado' }
  }
})

const bubbleClasses = computed(() =>
  cn(
    'max-w-[75%] rounded-2xl px-4 py-2.5 shadow-sm lg:max-w-[65%]',
    isOwn.value
      ? 'rounded-br-md bg-sky-400 text-white dark:bg-sky-600'
      : 'rounded-bl-md bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-100',
  ),
)

const timestamp = computed(() => formatTime(props.message.sent_at || props.message.created_at))
</script>

<template>
  <div :class="isOwn ? 'flex justify-end' : 'flex justify-start'">
    <div :class="bubbleClasses">
      <div v-if="message.attachments?.length" class="space-y-2">
        <template v-for="(att, i) in message.attachments ?? []" :key="i">
          <img
            v-if="isImage(att)"
            :src="resolveUrl(att.url)"
            :alt="`Imagen adjunta ${i + 1}`"
            class="max-h-64 w-auto cursor-zoom-in rounded-xl object-cover transition-opacity hover:opacity-90"
            loading="lazy"
            @click="openAttachment(att.url)"
          />
          <video
            v-else-if="isVideo(att)"
            :src="resolveUrl(att.url)"
            controls
            class="max-h-64 rounded-xl"
          ></video>
          <audio v-else-if="isAudio(att)" :src="resolveUrl(att.url)" controls class="block w-56 max-w-full" />
          <a
            v-else
            :href="resolveUrl(att.url)"
            target="_blank"
            rel="noopener"
            class="flex items-center gap-2 text-sm underline underline-offset-2"
          >
            <span class="material-symbols-outlined text-base" aria-hidden="true">attach_file</span>
            Archivo adjunto
          </a>
        </template>
      </div>

      <p v-if="message.text" class="whitespace-pre-wrap break-words text-sm">{{ message.text }}</p>

      <div class="mt-1 flex items-center justify-end gap-1">
        <span
          v-if="channel && CHANNEL_ICONS[channel]"
          class="material-symbols-outlined text-[12px] leading-none opacity-60"
          aria-hidden="true"
          >{{ CHANNEL_ICONS[channel] }}</span
        >
        <span class="text-[10px] opacity-70">{{ timestamp }}</span>
        <span
          v-if="isOwn"
          class="material-symbols-outlined text-sm leading-none"
          :class="statusMeta.classes"
          :title="statusMeta.title"
          role="img"
          :aria-label="statusMeta.title"
          >{{ statusMeta.icon }}</span
        >
      </div>
    </div>
  </div>
</template>
