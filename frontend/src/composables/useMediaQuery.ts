import { onScopeDispose, ref, type Ref } from 'vue'

export function useMediaQuery(query: string): Ref<boolean> {
  const matches = ref(false)

  if (typeof window !== 'undefined') {
    const mql = window.matchMedia(query)
    matches.value = mql.matches
    const handler = (e: MediaQueryListEvent) => {
      matches.value = e.matches
    }
    mql.addEventListener('change', handler)
    onScopeDispose(() => {
      mql.removeEventListener('change', handler)
    })
  }

  return matches
}

export function useIsMobile(): Ref<boolean> {
  return useMediaQuery('(max-width: 767px)')
}

export function useIsTabletUp(): Ref<boolean> {
  return useMediaQuery('(min-width: 768px)')
}

export function useIsDesktop(): Ref<boolean> {
  return useMediaQuery('(min-width: 1024px)')
}
