<script setup lang="ts">
import { useRoute } from 'vue-router'
import IconButton from '@/components/ui/IconButton.vue'
import Avatar from '@/components/ui/Avatar.vue'
import { useDarkMode } from '@/composables/useDarkMode'
import { useAuthStore } from '@/stores/auth'
import TopBarSearch from '@/components/topbar/TopBarSearch.vue'
import NotificationsBell from '@/components/topbar/NotificationsBell.vue'

defineEmits<{
  (e: 'toggle-sidebar'): void
}>()

const route = useRoute()
const { isDark, toggle } = useDarkMode()
const auth = useAuthStore()
</script>

<template>
  <header
    class="sticky top-0 z-20 flex h-14 items-center gap-3 border-b border-slate-200 bg-white/80 px-4 backdrop-blur dark:border-slate-800 dark:bg-[#101828]/80"
  >
    <IconButton icon="menu" class="md:hidden" @click="$emit('toggle-sidebar')" />

    <h1 class="text-lg font-semibold text-slate-900 dark:text-white">
      {{ typeof route.meta.title === 'string' ? route.meta.title : '' }}
    </h1>

    <div class="ml-auto flex items-center gap-2">
      <TopBarSearch />

      <IconButton :icon="isDark ? 'light_mode' : 'dark_mode'" @click="toggle" />

      <NotificationsBell />

      <Avatar :name="auth.user?.name || 'Operador'" size="sm" />
    </div>
  </header>
</template>
