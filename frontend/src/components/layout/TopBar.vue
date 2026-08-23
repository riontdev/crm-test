<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import IconButton from '@/components/ui/IconButton.vue'
import Avatar from '@/components/ui/Avatar.vue'
import { useDarkMode } from '@/composables/useDarkMode'
import { useAuthStore } from '@/stores/auth'

defineEmits<{
  (e: 'toggle-sidebar'): void
  (e: 'search', value: string): void
}>()

const route = useRoute()
const { isDark, toggle } = useDarkMode()
const searchQuery = ref('')
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
      <div
        class="hidden items-center gap-2 rounded-lg bg-slate-100 px-3 py-1.5 focus-within:ring-2 focus-within:ring-sky-400 lg:flex dark:bg-slate-800"
      >
        <span
          class="material-symbols-outlined text-xl text-slate-400"
          aria-hidden="true"
          >search</span
        >
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Buscar conversaciones..."
          class="w-64 bg-transparent text-sm text-slate-700 outline-none placeholder:text-slate-400 dark:text-slate-200"
          @input="$emit('search', searchQuery)"
        />
      </div>

      <IconButton :icon="isDark ? 'light_mode' : 'dark_mode'" @click="toggle" />

      <div class="relative">
        <IconButton icon="notifications" />
        <span
          class="absolute right-1 top-1 h-2 w-2 rounded-full bg-red-500"
          aria-hidden="true"
        />
      </div>

      <Avatar :name="auth.user?.name || 'Operador'" size="sm" />
    </div>
  </header>
</template>
