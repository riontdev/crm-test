<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import TopBar from '@/components/layout/TopBar.vue'
import ToastHost from '@/components/ui/ToastHost.vue'
import { initDarkMode } from '@/composables/useDarkMode'
import { useAuthStore } from '@/stores/auth'

initDarkMode()
const sidebarOpen = ref(false)
const auth = useAuthStore()
const route = useRoute()

const showShell = computed(
  () => auth.initialized && auth.isAuthenticated && !route.meta.public,
)
</script>

<template>
  <!-- Splash mientras se valida la sesión -->
  <div
    v-if="!auth.initialized"
    class="flex h-screen flex-col items-center justify-center gap-5 bg-[#F8F9FF] dark:bg-[#0B1220]"
  >
    <div
      class="flex h-12 w-12 animate-pulse items-center justify-center rounded-xl bg-slate-900 shadow-lg dark:bg-sky-400"
    >
      <span
        class="material-symbols-outlined text-2xl text-white dark:text-slate-900"
        aria-hidden="true"
        >hub</span
      >
    </div>
    <span
      class="h-5 w-5 animate-spin rounded-full border-2 border-slate-300 border-t-sky-400 dark:border-slate-700 dark:border-t-sky-400"
      aria-hidden="true"
    />
  </div>

  <!-- Shell de la app: solo autenticado en rutas privadas -->
  <div v-else-if="showShell" class="flex h-screen overflow-hidden">
    <AppSidebar v-model="sidebarOpen" />
    <div class="flex min-w-0 flex-1 flex-col">
      <TopBar @toggle-sidebar="sidebarOpen = !sidebarOpen" />
      <main class="min-h-0 flex-1 overflow-hidden">
        <RouterView />
      </main>
    </div>
  </div>

  <!-- Rutas públicas (login): pantalla sola, sin chrome -->
  <RouterView v-else />

  <ToastHost />
</template>
