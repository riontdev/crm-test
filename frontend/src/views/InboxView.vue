<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useConversationsStore } from '@/stores/conversations'
import type { Conversation } from '@/lib/api'
import { relativeTime } from '@/lib/utils'
import Avatar from '@/components/ui/Avatar.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'

const route = useRoute()
const store = useConversationsStore()

const searchInput = ref('')
let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(searchInput, (val) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    store.setSearch(val)
  }, 300)
})

function clearSearch() {
  if (debounceTimer) clearTimeout(debounceTimer)
  searchInput.value = ''
  store.setSearch('')
}

const channelTabs: Array<{ label: string; value?: string }> = [
  { label: 'Todos' },
  { label: 'WhatsApp', value: 'whatsapp' },
  { label: 'Instagram', value: 'instagram' },
  { label: 'Facebook', value: 'facebook' },
]

const listEl = ref<HTMLElement | null>(null)

const headerCount = computed(() =>
  store.total > 0 ? store.total : store.filteredConversations.length,
)

function onListScroll() {
  const el = listEl.value
  if (!el) return
  if (
    el.scrollTop + el.clientHeight >= el.scrollHeight - 240 &&
    store.hasMore &&
    !store.loadingMore &&
    !store.loading
  ) {
    store.loadMore()
  }
}

const activeId = computed(() => (typeof route.params.id === 'string' ? route.params.id : ''))

function isChannelActive(value?: string): boolean {
  return store.activeFilter.channel === value
}

function preview(conv: Conversation): string {
  const text = conv.last_message?.text
  if (!text) return 'Sin mensajes'
  return conv.last_message?.direction === 'outgoing' ? `Vos: ${text}` : text
}

onMounted(() => {
  store.fetchFirstPage()
  store.subscribe()
})

onUnmounted(() => {
  store.unsubscribe()
})
</script>

<template>
  <div class="flex h-full">
    <!-- Panel izquierdo: lista de conversaciones -->
    <aside
      class="flex w-full shrink-0 flex-col border-r border-slate-200 bg-white md:w-[360px] dark:border-slate-800 dark:bg-[#101828]"
    >
      <!-- Header -->
      <div class="px-4 pt-4 pb-3">
        <h2 class="text-xl font-semibold text-slate-900 dark:text-slate-100">Inbox</h2>
        <p class="mt-0.5 text-xs text-slate-400">
          {{ headerCount }}
          {{ headerCount === 1 ? 'conversación' : 'conversaciones' }}
        </p>
      </div>

      <!-- Buscador -->
      <div class="px-4 pb-2">
        <div class="relative">
          <span
            class="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-xl text-slate-400"
            aria-hidden="true"
          >search</span>
          <input
            v-model="searchInput"
            type="text"
            placeholder="Buscar..."
            aria-label="Buscar conversaciones"
            class="w-full rounded-lg bg-slate-100 py-2 pr-8 pl-10 text-sm text-slate-900 outline-none transition-shadow placeholder:text-slate-400 focus-visible:ring-2 focus-visible:ring-sky-400/60 dark:bg-slate-800 dark:text-slate-100"
          />
          <button
            v-if="searchInput.length > 0"
            type="button"
            aria-label="Limpiar búsqueda"
            class="absolute top-1/2 right-1.5 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-200 hover:text-slate-600 dark:hover:bg-slate-700 dark:hover:text-slate-300"
            @click="clearSearch"
          >
            <span class="material-symbols-outlined text-base" aria-hidden="true">close</span>
          </button>
        </div>
      </div>

      <!-- Tabs de filtro -->
      <div class="flex gap-2 overflow-x-auto px-4 py-2">
        <button
          v-for="tab in channelTabs"
          :key="tab.label"
          type="button"
          :class="[
            'rounded-full border px-3 py-1 text-xs font-medium whitespace-nowrap transition-colors',
            isChannelActive(tab.value)
              ? 'border-slate-900 bg-slate-900 text-white dark:border-white dark:bg-white dark:text-slate-900'
              : 'border-slate-200 bg-transparent text-slate-600 hover:border-slate-400 dark:border-slate-700 dark:text-slate-300',
          ]"
          @click="store.setFilter(tab.value)"
        >
          {{ tab.label }}
        </button>
        <button
          type="button"
          :class="[
            'rounded-full border px-3 py-1 text-xs font-medium whitespace-nowrap transition-colors',
            store.showUnreadOnly
              ? 'border-slate-900 bg-slate-900 text-white dark:border-white dark:bg-white dark:text-slate-900'
              : 'border-slate-200 bg-transparent text-slate-600 hover:border-slate-400 dark:border-slate-700 dark:text-slate-300',
          ]"
          @click="store.toggleUnreadOnly()"
        >
          No leídos
        </button>
        <button
          type="button"
          :class="[
            'rounded-full border px-3 py-1 text-xs font-medium whitespace-nowrap transition-colors',
            store.mineOnly
              ? 'border-slate-900 bg-slate-900 text-white dark:border-white dark:bg-white dark:text-slate-900'
              : 'border-slate-200 bg-transparent text-slate-600 hover:border-slate-400 dark:border-slate-700 dark:text-slate-300',
          ]"
          @click="store.toggleMine()"
        >
          Asignadas a mí
        </button>
      </div>

      <!-- Lista -->
      <div
        ref="listEl"
        class="flex-1 divide-y divide-slate-100 overflow-y-auto dark:divide-slate-800"
        @scroll.passive="onListScroll"
      >
        <!-- Loading -->
        <template v-if="store.loading">
          <div
            v-for="i in 3"
            :key="`skeleton-${i}`"
            class="flex items-start gap-3 px-4 py-3"
          >
            <div class="h-10 w-10 shrink-0 animate-pulse rounded-full bg-slate-200 dark:bg-slate-700" />
            <Skeleton :lines="2" />
          </div>
        </template>

        <!-- Sin resultados de búsqueda -->
        <EmptyState
          v-else-if="store.conversations.length > 0 && store.filteredConversations.length === 0"
          icon="search_off"
          title="Sin resultados"
          description="No hay conversaciones que coincidan con la búsqueda."
        />

        <!-- Vacío -->
        <EmptyState
          v-else-if="store.filteredConversations.length === 0"
          icon="forum"
          title="No hay conversaciones"
          description="Las conversaciones aparecerán cuando tus contactos te escriban."
        />

        <!-- Conversaciones -->
        <RouterLink
          v-for="conv in store.filteredConversations"
          v-else
          :key="conv.id"
          :to="`/inbox/${conv.id}`"
          :class="[
            'relative flex cursor-pointer items-start gap-3 px-4 py-3 transition-colors',
            conv.id === activeId
              ? 'bg-sky-50 dark:bg-sky-500/10'
              : 'hover:bg-slate-50 dark:hover:bg-slate-800/50',
          ]"
        >
          <span
            v-if="conv.id === activeId"
            class="absolute inset-y-0 left-0 w-1 rounded-r-full bg-sky-400"
            aria-hidden="true"
          />

          <Avatar :name="conv.contact?.name || conv.contact?.phone || 'Desconocido'" size="md" />

          <div class="min-w-0 flex-1">
            <div class="flex items-baseline justify-between gap-2">
              <span
                :class="[
                  'truncate text-sm text-slate-900 dark:text-slate-100',
                  conv.unread_count > 0 ? 'font-semibold' : 'font-medium',
                ]"
              >
                {{ conv.contact?.name || conv.contact?.phone || 'Desconocido' }}
              </span>
              <span
                v-if="conv.assigned_to"
                class="inline-flex shrink-0 items-center gap-1 text-[10px] font-medium text-slate-500 dark:text-slate-400"
                :title="`Asignada a ${conv.assigned_to.name}`"
              >
                <span class="material-symbols-outlined text-xs" aria-hidden="true">person</span>
                <span class="max-w-24 truncate">{{ conv.assigned_to.name }}</span>
              </span>
              <span class="shrink-0 text-[11px] text-slate-400">
                {{ relativeTime(conv.last_message?.sent_at || conv.updated_at) }}
              </span>
            </div>

            <div class="mt-0.5">
              <ChannelBadge :channel="conv.channel" />
            </div>

            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400">
              {{ preview(conv) }}
            </p>
          </div>

          <div class="shrink-0 pt-0.5">
            <span
              v-if="conv.unread_count > 0"
              class="flex h-5 min-w-5 items-center justify-center rounded-full bg-sky-400 px-1.5 text-[10px] font-semibold text-white"
            >
              {{ conv.unread_count > 9 ? '9+' : conv.unread_count }}
            </span>
          </div>
        </RouterLink>

        <!-- Sentinel: paginación -->
        <div v-if="store.loadingMore" class="py-3" role="status" aria-label="Cargando más conversaciones">
          <div class="mx-auto my-3 h-5 w-5 animate-spin rounded-full border-2 border-slate-300 border-t-sky-400 dark:border-slate-600 dark:border-t-sky-400" />
        </div>
        <p
          v-else-if="!store.hasMore && store.filteredConversations.length > 0"
          class="py-3 text-center text-[11px] text-slate-400 dark:text-slate-500"
        >
          Mostrando todas las conversaciones
        </p>
      </div>
    </aside>

    <!-- Área derecha: placeholder desktop -->
    <div class="hidden flex-1 items-center justify-center md:flex">
      <EmptyState
        icon="forum"
        title="Seleccioná una conversación"
        description="Elegí una conversación de la lista y los mensajes aparecerán acá."
      />
    </div>
  </div>
</template>
