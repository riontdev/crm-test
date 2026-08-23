<script setup lang="ts">
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()

const iconByType: Record<string, string> = {
  success: 'check_circle',
  error: 'error',
  info: 'info',
}

const styleByType: Record<string, string> = {
  success:
    'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-500/30 dark:bg-emerald-900/40 dark:text-emerald-300',
  error:
    'border-red-200 bg-red-50 text-red-700 dark:border-red-500/30 dark:bg-red-900/40 dark:text-red-300',
  info: 'border-sky-200 bg-sky-50 text-sky-800 dark:border-sky-500/30 dark:bg-sky-900/40 dark:text-sky-300',
}
</script>

<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed right-4 bottom-4 z-[60] flex w-full max-w-sm flex-col gap-2"
      aria-live="polite"
    >
      <TransitionGroup
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="translate-y-2 opacity-0"
        enter-to-class="translate-y-0 opacity-100"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-for="toast in ui.toasts"
          :key="toast.id"
          class="pointer-events-auto flex items-center gap-3 rounded-xl border px-4 py-3 shadow-lg backdrop-blur-sm"
          :class="styleByType[toast.type]"
          role="status"
        >
          <span class="material-symbols-outlined text-xl shrink-0" aria-hidden="true">{{
            iconByType[toast.type]
          }}</span>
          <p class="flex-1 text-sm font-medium">{{ toast.message }}</p>
          <button
            type="button"
            class="shrink-0 opacity-60 transition-opacity hover:opacity-100"
            aria-label="Cerrar notificación"
            @click="ui.dismiss(toast.id)"
          >
            <span class="material-symbols-outlined text-base" aria-hidden="true">close</span>
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>
