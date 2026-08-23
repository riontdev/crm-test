<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useConversationsStore } from '@/stores/conversations'
import IconButton from '@/components/ui/IconButton.vue'
import Avatar from '@/components/ui/Avatar.vue'

interface Props {
  modelValue: boolean
}

defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const route = useRoute()
const store = useConversationsStore()

interface NavItem {
  icon: string
  label: string
  to?: string
}

const navItems: NavItem[] = [
  { icon: 'dashboard', label: 'Dashboard' },
  { icon: 'inbox', label: 'Inbox', to: '/inbox' },
  { icon: 'smart_toy', label: 'Agentes IA', to: '/agents' },
  { icon: 'group', label: 'Usuarios', to: '/users' },
  { icon: 'hub', label: 'Canales' },
  { icon: 'description', label: 'Plantillas' },
  { icon: 'bar_chart', label: 'Reportes' },
]

const unreadTotal = computed(() =>
  store.conversations.reduce((acc, c) => acc + (c.unread_count ?? 0), 0),
)

function isActive(to: string): boolean {
  return route.path === to || route.path.startsWith(`${to}/`)
}

function close() {
  emit('update:modelValue', false)
}
</script>

<template>
  <div>
    <!-- Backdrop móvil -->
    <div
      v-show="modelValue"
      class="fixed inset-0 z-30 bg-black/40 md:hidden"
      @click="close"
    />

    <aside
      class="fixed inset-y-0 left-0 z-40 flex h-full w-[260px] shrink-0 transform flex-col border-r border-slate-200 bg-white transition-transform duration-200 ease-in-out md:static md:z-auto md:translate-x-0 dark:border-slate-800 dark:bg-[#101828]"
      :class="modelValue ? 'translate-x-0' : '-translate-x-full'"
    >
      <!-- Marca -->
      <div
        class="flex items-center gap-3 border-b border-slate-200 px-5 py-4 dark:border-slate-800"
      >
        <div
          class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-900 dark:bg-sky-400"
        >
          <span
            class="material-symbols-outlined text-white dark:text-slate-900"
            aria-hidden="true"
            >hub</span
          >
        </div>
        <div class="min-w-0">
          <p class="truncate text-sm font-bold text-slate-900 dark:text-white">SocialCRM</p>
          <p class="text-[11px] uppercase tracking-wider text-slate-400">Enterprise Hub</p>
        </div>
        <IconButton icon="close" size="sm" class="ml-auto md:hidden" @click="close" />
      </div>

      <!-- Navegación -->
      <nav class="mt-4 flex flex-col gap-1 px-3">
        <template v-for="item in navItems" :key="item.label">
          <RouterLink
            v-if="item.to"
            :to="item.to"
            class="relative flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors"
            :class="
              isActive(item.to)
                ? 'bg-slate-900/5 text-slate-900 dark:bg-white/5 dark:text-white'
                : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
            "
          >
            <span
              v-if="isActive(item.to)"
              class="absolute left-0 top-1/2 h-5 w-1 -translate-y-1/2 rounded-r-full bg-sky-400"
              aria-hidden="true"
            />
            <span class="material-symbols-outlined text-xl" aria-hidden="true">{{
              item.icon
            }}</span>
            {{ item.label }}
            <span
              v-if="item.to === '/inbox' && unreadTotal > 0"
              class="ml-auto rounded-full bg-sky-400 px-1.5 text-[10px] font-semibold leading-5 text-white"
            >
              {{ unreadTotal > 99 ? '99+' : unreadTotal }}
            </span>
          </RouterLink>
          <button
            v-else
            type="button"
            title="Próximamente"
            disabled
            class="flex cursor-not-allowed items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-slate-600 opacity-50 dark:text-slate-300"
          >
            <span class="material-symbols-outlined text-xl" aria-hidden="true">{{
              item.icon
            }}</span>
            {{ item.label }}
          </button>
        </template>
      </nav>

      <!-- Pie -->
      <div class="mt-auto border-t border-slate-200 px-3 py-4 dark:border-slate-800">
        <button
          type="button"
          title="Próximamente"
          disabled
          class="flex w-full cursor-not-allowed items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-slate-600 opacity-50 dark:text-slate-300"
        >
          <span class="material-symbols-outlined text-xl" aria-hidden="true">help</span>
          Centro de ayuda
        </button>

        <div class="my-2 border-t border-slate-200 dark:border-slate-800" />

        <div class="flex items-center gap-3 rounded-lg px-2 py-1.5">
          <Avatar name="Operador" size="sm" />
          <span class="text-sm font-medium text-slate-700 dark:text-slate-200">Operador</span>
          <IconButton icon="logout" size="sm" class="ml-auto" />
        </div>
      </div>
    </aside>
  </div>
</template>
