<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/lib/api'
import { relativeTime } from '@/lib/utils'
import { useClickOutside } from '@/composables/useClickOutside'
import Avatar from '@/components/ui/Avatar.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'

export interface UnreadItem {
  id: string
  channel: string
  contact_name: string
  preview_text?: string | null
  last_inbound_at?: string | null
  unread_count: number
}

const router = useRouter()

const open = ref(false)
const loading = ref(false)
const items = ref<UnreadItem[]>([])
const total = ref(0)
const boxRef = ref<HTMLElement | null>(null)

let eventSource: EventSource | null = null
let refreshTimer: ReturnType<typeof setTimeout> | null = null

useClickOutside(boxRef, () => {
  open.value = false
})

async function fetchUnread() {
  loading.value = true
  try {
    const res = await api.unreadFeed()
    items.value = (res.data || []) as UnreadItem[]
    total.value = res.total || 0
  } catch {
    // silencioso: la campanita degrada a estado vacío
  } finally {
    loading.value = false
  }
}

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  refreshTimer = setTimeout(fetchUnread, 1500)
}

onMounted(() => {
  fetchUnread()
  eventSource = new EventSource('/api/events')
  eventSource.addEventListener('message.received', scheduleRefresh)
})

onUnmounted(() => {
  eventSource?.close()
  if (refreshTimer) clearTimeout(refreshTimer)
})

function toggle() {
  open.value = !open.value
  if (open.value) fetchUnread()
}

function go(c: UnreadItem) {
  open.value = false
  router.push(`/inbox/${c.id}`)
}

const hasUnread = computed(() => total.value > 0)
</script>

<template>
  <div ref="boxRef" class="relative">
    <button
      type="button"
      class="relative inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-slate-500 transition-colors duration-150 hover:bg-slate-100 hover:text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
      :aria-label="hasUnread ? `${total} conversaciones sin leer` : 'Notificaciones'"
      @click="toggle"
    >
      <span class="material-symbols-outlined text-xl" aria-hidden="true">notifications</span>
      <span
        v-if="hasUnread"
        class="absolute right-1 top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[9px] font-bold text-white"
      >
        {{ total > 99 ? '99+' : total }}
      </span>
    </button>

    <div
      v-if="open"
      class="absolute right-0 top-full z-40 mt-2 w-[22rem] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xl dark:border-slate-800 dark:bg-[#101828]"
    >
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-2.5 dark:border-slate-800">
        <p class="text-xs font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
          Sin leer
        </p>
        <span v-if="hasUnread" class="text-[11px] font-medium text-sky-600 dark:text-sky-400">
          {{ total }} mensaje{{ total === 1 ? '' : 's' }}
        </span>
      </div>

      <p
        v-if="items.length === 0"
        class="px-4 py-8 text-center text-sm text-slate-400 dark:text-slate-500"
      >
        No tenés mensajes sin leer 🎉
      </p>

      <div v-else class="max-h-80 divide-y divide-slate-100 overflow-y-auto dark:divide-slate-800">
        <button
          v-for="c in items"
          :key="c.id"
          type="button"
          class="flex w-full items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/60"
          @click="go(c)"
        >
          <Avatar size="sm" :name="c.contact_name" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="truncate text-sm font-medium text-slate-800 dark:text-slate-100">{{
                c.contact_name
              }}</span>
              <ChannelBadge :channel="c.channel" />
              <span class="ml-auto shrink-0 text-[11px] text-slate-400">{{
                relativeTime(c.last_inbound_at ?? undefined)
              }}</span>
            </div>
            <p class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400">
              {{ c.preview_text || 'Sin mensajes' }}
            </p>
          </div>
          <span
            class="mt-1 flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-sky-400 px-1.5 text-[10px] font-semibold text-white"
          >
            {{ c.unread_count > 9 ? '9+' : c.unread_count }}
          </span>
        </button>
      </div>
    </div>
  </div>
</template>
