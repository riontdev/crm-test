import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastType = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  type: ToastType
  message: string
}

let nextId = 0

export const useUiStore = defineStore('ui', () => {
  const toasts = ref<Toast[]>([])

  function push(type: ToastType, message: string, durationMs = 4000) {
    const id = ++nextId
    toasts.value.push({ id, type, message })
    setTimeout(() => dismiss(id), durationMs)
  }

  function success(message: string) {
    push('success', message)
  }

  function error(message: string) {
    push('error', message, 6000)
  }

  function info(message: string) {
    push('info', message)
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  return { toasts, push, success, error, info, dismiss }
})
