import { onScopeDispose, type Ref } from 'vue'

interface ClickOutsideOptions {
  ignore?: Ref<HTMLElement | null>[]
}

export function useClickOutside(
  target: Ref<HTMLElement | null>,
  callback: () => void,
  options?: ClickOutsideOptions,
) {
  function handler(event: PointerEvent) {
    const el = target.value
    if (!el) return
    if (el.contains(event.target as Node)) return

    for (const ignored of options?.ignore ?? []) {
      if (ignored.value?.contains(event.target as Node)) return
    }

    callback()
  }

  document.addEventListener('pointerdown', handler)
  onScopeDispose(() => {
    document.removeEventListener('pointerdown', handler)
  })
}
