<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, type Conversation } from '@/lib/api'
import { relativeTime } from '@/lib/utils'
import { useClickOutside } from '@/composables/useClickOutside'
import Avatar from '@/components/ui/Avatar.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'

const router = useRouter()

const query = ref('')
const open = ref(false)
const loading = ref(false)
const results = ref<Conversation[]>([])
const boxRef = ref<HTMLElement | null>(null)

let debounceTimer: ReturnType<typeof setTimeout> | null = null

useClickOutside(boxRef, () => {
  open.value = false
})

function onInput() {
  if (debounceTimer) clearTimeout(debounceTimer)
  const q = query.value.trim()
  if (!q) {
    open.value = false
    results.value = []
    return
  }
  debounceTimer = setTimeout(() => runSearch(q), 300)
}

async function runSearch(q: string) {
  loading.value = true
  open.value = true
  try {
    const res = await api.searchConversations(q)
    results.value = res.data || []
  } catch {
    results.value = []
  } finally {
    loading.value = false
  }
}

function preview(c: Conversation): string {
  const text = c.last_message?.text
  if (!text) return 'Sin mensajes'
  return c.last_message?.direction === 'outgoing' ? `Vos: ${text}` : text
}

function contactName(c: Conversation): string {
  return c.contact?.name || c.contact?.phone || 'Desconocido'
}

function go(c: Conversation) {
  open.value = false
  query.value = ''
  results.value = []
  router.push(`/inbox/${c.id}`)
}

function clear() {
  query.value = ''
  results.value = []
  open.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    open.value = false
    ;(e.target as HTMLInputElement)?.blur()
  }
}

const hasQuery = computed(() => query.value.trim().length > 0)

watch(open, (v) => {
  if (v && !hasQuery.value) open.value = false
})
</script>

<template>
  <div ref="boxRef" class="relative hidden lg:block">
    <div
     
      class="flex items-center gap-2 rounded-lg bg-slate-100 px-3 py-1.5 focus-within:ring-2 focus-within:ring-sky-400 dark:bg-slate-800"
    >
      <span class="material-symbols-outlined text-xl text-slate-400" aria-hidden="true">search</span>
      <input
        v-model="query"
        type="text"
        placeholder="Buscar conversaciones..."
        class="w-64 bg-transparent text-sm text-slate-700 outline-none placeholder:text-slate-400 dark:text-slate-200"
        aria-label="Buscar conversaciones"
        @input="onInput"
        @focus="hasQuery && (open = true)"
        @keydown="onKeydown"
      />
      <span
        v-if="loading"
        class="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-sky-400"
        aria-hidden="true"
      />
      <button
        v-else-if="hasQuery"
        type="button"
        aria-label="Limpiar búsqueda"
        class="text-slate-400 transition-colors hover:text-slate-600 dark:hover:text-slate-200"
        @click="clear"
      >
        <span class="material-symbols-outlined text-base" aria-hidden="true">close</span>
      </button>
    </div>

    <!-- Panel de resultados -->
    <div
      v-if="open"
      class="absolute right-0 top-full z-40 mt-2 w-[26rem] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xl dark:border-slate-800 dark:bg-[#101828]"
    >
      <p
        v-if="!loading && results.length === 0"
        class="px-4 py-6 text-center text-sm text-slate-400 dark:text-slate-500"
      >
        Sin resultados para "{{ query }}"
      </p>

      <div v-else class="max-h-96 divide-y divide-slate-100 overflow-y-auto dark:divide-slate-800">
        <button
          v-for="c in results"
          :key="c.id"
          type="button"
          class="flex w-full items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/60"
          @click="go(c)"
        >
          <Avatar size="sm" :name="contactName(c)" :src="c.contact?.avatar_url" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="truncate text-sm font-medium text-slate-800 dark:text-slate-100">{{
                contactName(c)
              }}</span>
              <ChannelBadge :channel="c.channel" />
              <span class="ml-auto shrink-0 text-[11px] text-slate-400">{{
                relativeTime(c.last_message?.sent_at || c.updated_at)
              }}</span>
            </div>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400">
              {{ preview(c) }}
            </p>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>
