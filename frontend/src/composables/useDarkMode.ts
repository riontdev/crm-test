import { ref, type Ref } from 'vue'

const isDark = ref(false)
let initialized = false

function applyTheme(dark: boolean) {
  isDark.value = dark
  document.documentElement.classList.toggle('dark', dark)
}

function storedPreference(): boolean | null {
  try {
    const stored = localStorage.getItem('crm-theme')
    if (stored === 'dark') return true
    if (stored === 'light') return false
  } catch {
    // localStorage no disponible
  }
  return null
}

function systemPreference(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function initDarkMode() {
  if (initialized) return
  initialized = true
  applyTheme(storedPreference() ?? systemPreference())
}

export function useDarkMode(): { isDark: Ref<boolean>; toggle: () => void; setTheme: (dark: boolean) => void } {
  if (!initialized) initDarkMode()

  function setTheme(dark: boolean) {
    try {
      localStorage.setItem('crm-theme', dark ? 'dark' : 'light')
    } catch {
      // noop
    }
    applyTheme(dark)
  }

  function toggle() {
    setTheme(!isDark.value)
  }

  return { isDark, toggle, setTheme }
}
