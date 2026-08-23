<script setup lang="ts">
import { ref } from 'vue'
import { RouterView } from 'vue-router'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import TopBar from '@/components/layout/TopBar.vue'
import ToastHost from '@/components/ui/ToastHost.vue'
import { initDarkMode } from '@/composables/useDarkMode'

initDarkMode()
const sidebarOpen = ref(false)
</script>

<template>
  <div class="h-screen flex overflow-hidden">
    <AppSidebar v-model="sidebarOpen" />
    <div class="flex-1 flex flex-col min-w-0">
      <TopBar @toggle-sidebar="sidebarOpen = !sidebarOpen" />
      <main class="flex-1 min-h-0 overflow-hidden">
        <RouterView />
      </main>
    </div>
    <ToastHost />
  </div>
</template>
