<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Button from '@/components/ui/Button.vue'
import IconButton from '@/components/ui/IconButton.vue'
import EmojiPicker from '@/components/chat/EmojiPicker.vue'

export interface ComposerPayload {
  text: string
  file: File | null
}

interface Props {
  disabled?: boolean
  sending?: boolean
  uploading?: boolean
  accountIdMissing?: boolean
  seed?: string
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  sending: false,
  uploading: false,
  accountIdMissing: false,
  seed: '',
})

const emit = defineEmits<{ send: [payload: ComposerPayload]; 'seed-applied': [] }>()

watch(
  () => props.seed,
  (val) => {
    if (!val) return
    newText.value = newText.value ? `${newText.value} ${val}` : val
    nextTick(() => {
      autosize()
      textarea.value?.focus()
    })
    emit('seed-applied')
  },
)

const MAX_FILE_SIZE = 10 * 1024 * 1024

const newText = ref('')
const selectedFile = ref<File | null>(null)
const previewUrl = ref<string | null>(null)
const showEmoji = ref(false)
const fileError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const textarea = ref<HTMLTextAreaElement | null>(null)

const canSend = computed(
  () =>
    !props.disabled &&
    !props.sending &&
    !props.uploading &&
    (!!newText.value.trim() || !!selectedFile.value),
)

watch(selectedFile, (file) => {
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
  }
  if (file && file.type.startsWith('image/')) {
    previewUrl.value = URL.createObjectURL(file)
  }
})

onBeforeUnmount(() => {
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
})

function autosize(): void {
  const el = textarea.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 144)}px`
}

watch(newText, () => nextTick(autosize))

function pickFile(): void {
  fileInput.value?.click()
}

function onFileChange(e: Event): void {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > MAX_FILE_SIZE) {
    fileError.value = 'El archivo no puede superar 10MB'
    return
  }
  fileError.value = ''
  selectedFile.value = file
}

function clearFile(): void {
  selectedFile.value = null
  fileError.value = ''
}

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${Math.max(1, Math.round(bytes / 1024))} KB`
}

function onEmojiSelect(emoji: string): void {
  newText.value += emoji
  nextTick(() => {
    autosize()
    textarea.value?.focus()
  })
}

function trySend(): void {
  const text = newText.value.trim()
  if ((!text && !selectedFile.value) || props.sending || props.uploading || props.disabled) return
  emit('send', { text, file: selectedFile.value })
  newText.value = ''
  clearFile()
  showEmoji.value = false
  nextTick(autosize)
}
</script>

<template>
  <div class="border-t border-slate-200 bg-white px-4 py-3 dark:border-slate-800 dark:bg-[#101828]">
    <p v-if="accountIdMissing" class="mb-2 text-xs text-amber-600 dark:text-amber-400">
      No hay account_id configurado para esta conversación.
    </p>

    <p v-if="fileError" class="mb-2 text-xs text-red-500" role="alert">{{ fileError }}</p>

    <div v-if="selectedFile" class="mb-2 flex items-center gap-3 rounded-lg bg-slate-50 p-2 dark:bg-slate-800/60">
      <img v-if="previewUrl" :src="previewUrl" :alt="`Vista previa de ${selectedFile.name}`" class="h-12 w-12 shrink-0 rounded object-cover" />
      <div v-else class="flex h-12 w-12 shrink-0 items-center justify-center rounded bg-slate-200 dark:bg-slate-700">
        <span class="material-symbols-outlined text-slate-500 dark:text-slate-300" aria-hidden="true">insert_drive_file</span>
      </div>
      <div class="min-w-0 flex-1">
        <p class="truncate text-xs font-medium text-slate-700 dark:text-slate-200">{{ selectedFile.name }}</p>
        <p class="text-[10px] text-slate-400 dark:text-slate-500">{{ formatSize(selectedFile.size) }}</p>
      </div>
      <IconButton icon="close" size="sm" aria-label="Quitar archivo adjunto" @click="clearFile" />
    </div>

    <div class="flex items-end gap-2">
      <IconButton
        icon="attach_file"
        variant="outline"
        aria-label="Adjuntar archivo"
        :disabled="disabled"
        @click="pickFile"
      />

      <div class="relative flex-1">
        <EmojiPicker v-if="showEmoji" @select="onEmojiSelect" @close="showEmoji = false" />
        <textarea
          ref="textarea"
          v-model="newText"
          rows="1"
          placeholder="Escribí un mensaje..."
          class="w-full resize-none rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 pr-11 text-sm text-slate-700 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-sky-400 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500"
          :disabled="disabled"
          aria-label="Escribí un mensaje"
          @keydown.enter.exact.prevent="trySend"
          @input="autosize"
        ></textarea>
        <IconButton
          icon="mood"
          size="sm"
          class="absolute bottom-1.5 right-1.5"
          :aria-label="showEmoji ? 'Cerrar selector de emojis' : 'Insertar emoji'"
          @click="showEmoji = !showEmoji"
        />
      </div>

      <Button
        variant="primary"
        class="h-11 w-11 !rounded-full !p-0"
        :disabled="!canSend"
        :loading="sending || uploading"
        aria-label="Enviar mensaje"
        @click="trySend"
      >
        <span v-if="!(sending || uploading)" class="material-symbols-outlined" aria-hidden="true">send</span>
      </Button>
    </div>

    <input
      ref="fileInput"
      type="file"
      accept="image/*,video/*,audio/*,.pdf"
      class="hidden"
      aria-label="Seleccionar archivo para adjuntar"
      @change="onFileChange"
    />
  </div>
</template>
