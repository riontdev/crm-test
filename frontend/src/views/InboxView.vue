<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useConversationsStore } from '@/stores/conversations'

const store = useConversationsStore()

onMounted(() => {
  store.fetchConversations()
  store.subscribe()
})

onUnmounted(() => {
  store.unsubscribe()
})

const channelColors: Record<string, string> = {
  whatsapp: 'bg-green-100 text-green-700',
  instagram: 'bg-pink-100 text-pink-700',
  facebook: 'bg-blue-100 text-blue-700',
}

function formatDate(dateStr?: string) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return 'ahora'
  if (diffMin < 60) return `${diffMin}m`
  const diffH = Math.floor(diffMin / 60)
  if (diffH < 24) return `${diffH}h`
  const diffD = Math.floor(diffH / 24)
  return `${diffD}d`
}
</script>

<template>
  <div class="flex h-[calc(100vh-53px)]">
    <!-- Sidebar: conversation list -->
    <div class="w-80 border-r border-neutral-200 overflow-y-auto">
      <!-- Filters -->
      <div class="p-3 border-b border-neutral-100 flex gap-2">
        <button
          @click="store.setFilter()"
          :class="[
            'px-3 py-1 text-xs rounded-full border transition-colors',
            !store.activeFilter.channel
              ? 'bg-black text-white border-black'
              : 'bg-white text-neutral-600 border-neutral-200 hover:border-neutral-400'
          ]"
        >
          Todos
        </button>
        <button
          v-for="ch in ['whatsapp', 'instagram', 'facebook']"
          :key="ch"
          @click="store.setFilter(ch)"
          :class="[
            'px-3 py-1 text-xs rounded-full border transition-colors capitalize',
            store.activeFilter.channel === ch
              ? 'bg-black text-white border-black'
              : 'bg-white text-neutral-600 border-neutral-200 hover:border-neutral-400'
          ]"
        >
          {{ ch }}
        </button>
      </div>

      <!-- Loading -->
      <div v-if="store.loading" class="p-8 text-center text-neutral-400 text-sm">
        Cargando...
      </div>

      <!-- Empty state -->
      <div v-else-if="store.conversations.length === 0" class="p-8 text-center text-neutral-400 text-sm">
        No hay conversaciones
      </div>

      <!-- Conversation list -->
      <RouterLink
        v-for="conv in store.conversations"
        :key="conv.id"
        :to="`/inbox/${conv.id}`"
        class="flex items-start gap-3 p-4 border-b border-neutral-100 hover:bg-neutral-50 transition-colors cursor-pointer"
      >
        <!-- Avatar -->
        <div class="w-10 h-10 rounded-full bg-neutral-200 flex items-center justify-center text-sm font-medium text-neutral-600 shrink-0">
          {{ conv.contact?.name?.charAt(0) || '?' }}
        </div>

        <div class="flex-1 min-w-0">
          <div class="flex items-center justify-between gap-2">
            <span class="font-medium text-sm truncate">
              {{ conv.contact?.name || conv.contact?.phone || 'Desconocido' }}
            </span>
            <span class="text-xs text-neutral-400 shrink-0">
              {{ formatDate(conv.last_message?.sent_at || conv.updated_at) }}
            </span>
          </div>

          <div class="flex items-center gap-2 mt-0.5">
            <span
              :class="[
                'text-[10px] px-1.5 py-0.5 rounded font-medium capitalize',
                channelColors[conv.channel] || 'bg-neutral-100 text-neutral-600'
              ]"
            >
              {{ conv.channel }}
            </span>
            <span class="text-xs text-neutral-500 truncate">
              {{ conv.last_message?.text || 'Sin mensajes' }}
            </span>
          </div>
        </div>

        <!-- Unread badge -->
        <div
          v-if="conv.unread_count > 0"
          class="w-5 h-5 rounded-full bg-black text-white text-[10px] flex items-center justify-center shrink-0"
        >
          {{ conv.unread_count > 9 ? '9+' : conv.unread_count }}
        </div>
      </RouterLink>
    </div>

    <!-- Main area: empty state or redirect -->
    <div class="flex-1 flex items-center justify-center text-neutral-400">
      <div class="text-center">
        <div class="text-4xl mb-4">💬</div>
        <p class="text-sm">Selecciona una conversación</p>
      </div>
    </div>
  </div>
</template>
