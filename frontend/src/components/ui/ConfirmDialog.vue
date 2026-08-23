<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import Button from '@/components/ui/Button.vue'

interface Props {
  show: boolean
  title: string
  message?: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'warning' | 'primary'
}

const props = withDefaults(defineProps<Props>(), {
  confirmText: 'Confirmar',
  cancelText: 'Cancelar',
  variant: 'primary',
})

const emit = defineEmits<{ confirm: []; cancel: [] }>()

function confirmButtonVariant() {
  return props.variant === 'primary' ? 'primary' : props.variant
}

function onKeydown(e: KeyboardEvent) {
  if (!props.show) return
  if (e.key === 'Escape') emit('cancel')
  if (e.key === 'Enter') emit('confirm')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0"
      leave-active-class="transition duration-150 ease-in"
      leave-to-class="opacity-0"
    >
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        @click.self="emit('cancel')"
      >
        <div
          class="w-full max-w-sm rounded-xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-700 dark:bg-[#101828]"
        >
          <h3 class="text-base font-semibold text-slate-800 dark:text-slate-100">{{ title }}</h3>
          <p v-if="message" class="mt-2 text-sm text-slate-500 dark:text-slate-400">{{ message }}</p>
          <div class="mt-5 flex justify-end gap-2">
            <Button variant="secondary" size="sm" @click="emit('cancel')">
              {{ cancelText }}
            </Button>
            <Button :variant="confirmButtonVariant()" size="sm" @click="emit('confirm')">
              {{ confirmText }}
            </Button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
