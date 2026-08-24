<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useChannelsStore } from '@/stores/channels'
import { useUiStore } from '@/stores/ui'
import { relativeTime } from '@/lib/utils'
import type { ChannelStatus } from '@/lib/api'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import IconButton from '@/components/ui/IconButton.vue'

const store = useChannelsStore()
const ui = useUiStore()

interface ChannelMeta {
  name: string
  icon: string
  circleClass: string
}

const CHANNEL_META: Record<string, ChannelMeta> = {
  whatsapp: { name: 'WhatsApp', icon: 'chat', circleClass: 'bg-[#25D366]/10 text-[#25D366]' },
  instagram: { name: 'Instagram', icon: 'photo_camera', circleClass: 'bg-[#E1306C]/10 text-[#E1306C]' },
  facebook: { name: 'Facebook', icon: 'thumb_up', circleClass: 'bg-[#1877F2]/10 text-[#1877F2]' },
}

const GUIDE_STEPS = [
  'Ingresá a tu cuenta de Zernio',
  'Sección Canales → Agregar canal',
  'Autorizá los permisos solicitados',
  'Volvé acá y presioná Sincronizar',
]

const guideChannel = ref<ChannelStatus | null>(null)

function metaFor(channel: string): ChannelMeta {
  return (
    CHANNEL_META[channel] ?? {
      name: channel.charAt(0).toUpperCase() + channel.slice(1),
      icon: 'hub',
      circleClass: 'bg-slate-500/10 text-slate-500 dark:text-slate-400',
    }
  )
}

function guideStep(channelName: string): Array<string> {
  const steps = [...GUIDE_STEPS]
  steps[1] = `${steps[1]} ${channelName}`
  return steps
}

async function copyWebhook() {
  try {
    await navigator.clipboard.writeText(store.webhookUrl)
    ui.success('URL copiada')
  } catch {
    ui.error('No se pudo copiar la URL')
  }
}

onMounted(() => {
  store.fetchStatus()
})
</script>

<template>
  <div class="h-full overflow-y-auto px-6 py-6">
    <header class="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 class="text-2xl font-semibold tracking-[-0.01em] text-slate-900 dark:text-white">
          Canales
        </h1>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Conectá y configurá tus canales de comunicación.
        </p>
      </div>
      <Button variant="secondary" size="sm" :loading="store.loading" @click="store.fetchStatus()">
        <span class="material-symbols-outlined text-base" aria-hidden="true">refresh</span>
        Sincronizar
      </Button>
    </header>

    <div
      class="mt-5 flex items-start gap-2 rounded-xl border border-sky-500/20 bg-sky-500/10 px-4 py-3 text-sm text-sky-800 dark:text-sky-300"
    >
      <span class="material-symbols-outlined mt-0.5 shrink-0 text-lg" aria-hidden="true">info</span>
      <p>
        Los canales viajan por Zernio. Para conectar Instagram o Facebook seguí la guía de
        conexión de Zernio (Fase 11 pendiente).
      </p>
    </div>

    <div v-if="store.loading" class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <div
        v-for="i in 3"
        :key="i"
        class="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
      >
        <div class="mb-4 flex items-center gap-3">
          <div class="h-11 w-11 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
          <div class="h-5 w-24 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
        </div>
        <Skeleton :lines="5" />
      </div>
    </div>

    <EmptyState
      v-else-if="store.error"
      icon="cloud_off"
      title="No se pudieron cargar los canales"
      :description="store.error"
    >
      <template #action>
        <Button size="sm" @click="store.fetchStatus()">Reintentar</Button>
      </template>
    </EmptyState>

    <div v-else class="mt-6 grid items-stretch gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article
        v-for="ch in store.channels"
        :key="ch.channel"
        class="flex flex-col gap-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-all duration-200 hover:shadow-md dark:border-slate-800 dark:bg-[#101828]"
      >
        <div class="flex items-center gap-3">
          <div
            :class="[metaFor(ch.channel).circleClass, 'flex h-11 w-11 shrink-0 items-center justify-center rounded-full']"
          >
            <span class="material-symbols-outlined text-xl" aria-hidden="true">{{
              metaFor(ch.channel).icon
            }}</span>
          </div>
          <h2 class="font-semibold text-slate-900 dark:text-white">
            {{ metaFor(ch.channel).name }}
          </h2>
          <Badge class="ml-auto" :variant="ch.connected ? 'success' : 'default'">
            {{ ch.connected ? 'Conectado' : 'Sin actividad' }}
          </Badge>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <div class="rounded-lg bg-slate-50 p-2.5 text-center dark:bg-slate-800/60">
            <p class="text-lg font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
              {{ ch.conversations_count.toLocaleString('es') }}
            </p>
            <p
              class="text-[10px] font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
            >
              Conversaciones
            </p>
          </div>
          <div class="rounded-lg bg-slate-50 p-2.5 text-center dark:bg-slate-800/60">
            <p class="text-lg font-bold tracking-[-0.02em] text-slate-900 dark:text-white">
              {{ ch.messages_count.toLocaleString('es') }}
            </p>
            <p
              class="text-[10px] font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
            >
              Mensajes
            </p>
          </div>
        </div>

        <div class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          <span class="material-symbols-outlined text-sm" aria-hidden="true">schedule</span>
          Última actividad:
          <span class="font-medium">{{ relativeTime(ch.last_activity_at ?? undefined) || 'sin registros' }}</span>
        </div>

        <div class="flex items-center justify-between text-xs">
          <span class="font-medium text-slate-600 dark:text-slate-300">Agente IA</span>
          <Badge variant="accent" size="sm">
            {{ ch.agent_enabled ? 'Activado' : 'Apagado' }}
          </Badge>
        </div>

        <div
          v-if="ch.connected"
          class="mt-auto rounded-lg bg-slate-50 p-2.5 dark:bg-slate-800/60"
        >
          <p
            class="text-[10px] font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500"
          >
            Webhook
          </p>
          <div class="mt-1 flex items-center gap-1">
            <code
              class="min-w-0 flex-1 truncate font-mono text-[11px] text-slate-700 dark:text-slate-300"
              :title="store.webhookUrl"
            >
              {{ store.webhookUrl }}
            </code>
            <IconButton
              icon="content_copy"
              size="sm"
              title="Copiar URL del webhook"
              aria-label="Copiar URL del webhook"
              @click="copyWebhook"
            />
          </div>
        </div>

        <Button
          v-else
          variant="secondary"
          size="sm"
          class="mt-auto w-full"
          @click="guideChannel = ch"
        >
          Conectar canal
        </Button>
      </article>
    </div>

    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0"
        leave-active-class="transition duration-150 ease-in"
        leave-to-class="opacity-0"
      >
        <div
          v-if="guideChannel"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
          role="dialog"
          aria-modal="true"
          :aria-label="`Conectar ${metaFor(guideChannel.channel).name}`"
          @click.self="guideChannel = null"
        >
          <div
            class="w-full max-w-sm rounded-xl border border-slate-200 bg-white p-6 shadow-xl dark:border-slate-700 dark:bg-[#101828]"
          >
            <h3 class="text-base font-semibold text-slate-800 dark:text-slate-100">
              Conectar {{ metaFor(guideChannel.channel).name }}
            </h3>
            <ol class="mt-4 list-decimal space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
              <li v-for="(step, i) in guideStep(metaFor(guideChannel.channel).name)" :key="i">
                {{ step }}
              </li>
            </ol>
            <div class="mt-5 flex justify-end">
              <Button size="sm" @click="guideChannel = null">Entendido</Button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
