<script setup lang="ts">
import { onMounted, reactive, watch } from 'vue'
import { useAgentsStore } from '@/stores/agents'
import { useInsightsStore } from '@/stores/insights'
import type { AgentConfig } from '@/lib/api'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import IconButton from '@/components/ui/IconButton.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'

const store = useAgentsStore()
const insights = useInsightsStore()

interface AgentDraft {
  model: string
  system_prompt: string
  temperature: number
  max_tokens: number
}

const expanded = reactive<Record<string, boolean>>({})
const drafts = reactive<Record<string, AgentDraft>>({})

watch(
  () => ({ ...expanded }),
  (val) => {
    for (const [channel, isOpen] of Object.entries(val)) {
      if (isOpen && !drafts[channel]) {
        const agent = store.agents.find((a) => a.channel === channel)
        if (agent) drafts[channel] = draftFrom(agent)
      }
    }
  },
)

function draftFrom(agent: AgentConfig): AgentDraft {
  return {
    model: agent.model,
    system_prompt: agent.system_prompt ?? '',
    temperature: Number(agent.temperature),
    max_tokens: Number(agent.max_tokens),
  }
}

function channelLabel(channel: string): string {
  const labels: Record<string, string> = {
    whatsapp: 'WhatsApp',
    instagram: 'Instagram',
    facebook: 'Messenger',
  }
  return labels[channel] || channel.charAt(0).toUpperCase() + channel.slice(1)
}

function toggleExpanded(agent: AgentConfig) {
  expanded[agent.channel] = !expanded[agent.channel]
}

function resetDraft(agent: AgentConfig) {
  drafts[agent.channel] = draftFrom(agent)
}

function isDirty(agent: AgentConfig): boolean {
  const d = drafts[agent.channel]
  if (!d) return false
  return (
    d.model.trim() !== agent.model ||
    d.system_prompt !== (agent.system_prompt ?? '') ||
    Number(d.temperature) !== Number(agent.temperature) ||
    Math.round(Number(d.max_tokens)) !== Number(agent.max_tokens)
  )
}

async function handleSave(agent: AgentConfig) {
  const d = drafts[agent.channel]
  if (!d || !isDirty(agent)) return
  await store.saveAgent(agent.channel, {
    model: d.model.trim() || agent.model,
    system_prompt: d.system_prompt.trim() === '' ? null : d.system_prompt,
    temperature: Number.isFinite(Number(d.temperature))
      ? Number(d.temperature)
      : Number(agent.temperature),
    max_tokens: Number.isFinite(Number(d.max_tokens))
      ? Math.round(Number(d.max_tokens))
      : Number(agent.max_tokens),
  })
  const updated = store.agents.find((a) => a.channel === agent.channel)
  if (updated) drafts[agent.channel] = draftFrom(updated)
}

onMounted(async () => {
  store.fetchAgents()
  // El status del analizador va aparte: si /insights/status falla (modulo
  // apagado en el servidor) no debe romper la carga de los agentes de chat.
  if (!insights.status) await insights.fetchStatus()
  // Recién con el status resuelto se puede saber si el modulo escucha. Antes
  // esta línea nunca corría y el panel no mostraba las colas en vivo.
  if (insights.enabled) insights.subscribe()
})
</script>

<template>
  <div class="mx-auto w-full max-w-6xl px-6 py-8">
    <header class="mb-6">
      <h1 class="text-2xl font-semibold tracking-[-0.01em] text-slate-900 dark:text-slate-100">
        Agentes IA
      </h1>
      <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
        Cada canal tiene su propio agente. Los canales nuevos arrancan apagados.
      </p>
    </header>

    <div
      class="mb-6 flex items-start gap-2 rounded-xl border border-sky-500/20 bg-sky-500/10 px-4 py-3 text-sm text-sky-800 dark:text-sky-300"
    >
      <span class="material-symbols-outlined mt-0.5 shrink-0 text-lg" aria-hidden="true">info</span>
      <p>
        Los agentes están construidos pero requieren configurar OPENROUTER_API_KEY para responder.
        Al activarlos sin la clave no contestarán.
      </p>
    </div>

    <!-- Análisis de pedidos: IA local, sin respuesta automática -->
    <section class="mb-6 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-[#101828]">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 class="text-base font-semibold text-slate-900 dark:text-slate-100">
            Análisis de pedidos
          </h2>
          <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            Lee las conversaciones y arma el pedido. <strong class="font-medium">No responde nunca</strong>: solo
            resume. Corre en esta máquina, ningún mensaje sale del servidor.
          </p>
        </div>
        <label class="flex shrink-0 cursor-pointer items-center gap-2">
          <span class="text-xs font-medium text-slate-600 dark:text-slate-300">
            {{ insights.status?.master_enabled ? 'Activo' : 'Pausado' }}
          </span>
          <input
            type="checkbox"
            class="peer sr-only"
            :checked="insights.status?.master_enabled ?? false"
            :disabled="!insights.status"
            aria-label="Interruptor general del análisis de pedidos"
            @change="insights.setMasterEnabled(($event.target as HTMLInputElement).checked)"
          />
          <span
            class="relative h-5 w-9 rounded-full bg-slate-300 transition-colors peer-checked:bg-indigo-500 peer-focus-visible:ring-2 peer-focus-visible:ring-indigo-400 dark:bg-slate-700"
            aria-hidden="true"
          >
            <span class="absolute top-0.5 left-0.5 h-4 w-4 rounded-full bg-white transition-transform peer-checked:translate-x-4" />
          </span>
        </label>
      </div>

      <div v-if="insights.statusLoading && !insights.status" class="mt-4 space-y-2">
        <div class="h-4 w-1/3 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
      </div>

      <div v-else-if="!insights.status" class="mt-4 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500 dark:bg-slate-800/40 dark:text-slate-400">
        El módulo de análisis no está habilitado en el servidor.
      </div>

      <template v-else>
        <!-- salud del modelo: si no está, el usuario tiene que saberlo -->
        <p
          class="mt-4 flex items-start gap-2 rounded-lg px-3 py-2 text-xs"
          :class="insights.health === 'ok'
            ? 'bg-emerald-500/10 text-emerald-800 dark:text-emerald-300'
            : insights.health === 'degraded'
              ? 'bg-amber-500/10 text-amber-800 dark:text-amber-300'
              : 'bg-slate-500/10 text-slate-600 dark:text-slate-300'"
        >
          <span
            class="material-symbols-outlined mt-px shrink-0"
            aria-hidden="true"
          >{{ insights.health === 'ok' ? 'check_circle' : insights.health === 'degraded' ? 'warning' : 'pause_circle' }}</span>
          <span v-if="insights.health === 'off'">El análisis está pausado. No se está leyendo ninguna conversación.</span>
          <span v-else-if="!insights.modelReady">
            El modelo <code class="font-mono">{{ insights.status.model }}</code> no está cargado en Ollama.
            Bajalo con <code class="font-mono">ollama pull {{ insights.status.model }}</code>; mientras tanto los
            mensajes quedan en cola.
          </span>
          <span v-else-if="insights.modelReady && insights.health === 'degraded'">
            Listo con <code class="font-mono">{{ insights.status.model }}</code>, pero hay
            {{ insights.counts?.error }} mensajes con error.
          </span>
          <span v-else>
            Listo con <code class="font-mono">{{ insights.status.model }}</code>
            <template v-if="insights.counts?.needs_review"> · {{ insights.counts.needs_review }} para revisar</template>
            <template v-if="insights.queueBusy"> · {{ insights.queueBusy }} en cola</template>
          </span>
        </p>

        <!-- contadores -->
        <dl class="mt-4 grid grid-cols-3 gap-2 sm:grid-cols-6">
          <div
            v-for="k in [
              { label: 'Analizados', value: insights.counts?.ok ?? 0, tone: '' },
              { label: 'Pendientes', value: insights.counts?.pending ?? 0, tone: '' },
              { label: 'Procesando', value: insights.counts?.processing ?? 0, tone: '' },
              { label: 'Omitidos', value: insights.counts?.skipped ?? 0, tone: '' },
              { label: 'Con error', value: insights.counts?.error ?? 0, tone: 'text-red-600 dark:text-red-400' },
              { label: 'Para revisar', value: insights.counts?.needs_review ?? 0, tone: 'text-amber-600 dark:text-amber-400' },
            ]"
            :key="k.label"
            class="rounded-lg bg-slate-50 px-2.5 py-2 text-center dark:bg-slate-800/50"
          >
            <dd class="text-lg font-semibold tabular-nums" :class="k.tone">{{ k.value }}</dd>
            <dt class="text-[10px] tracking-wide text-slate-500 uppercase dark:text-slate-400">{{ k.label }}</dt>
          </div>
        </dl>

        <!-- notas de voz -->
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-3 dark:border-slate-800">
          <div>
            <p class="text-sm font-medium text-slate-700 dark:text-slate-200">Notas de voz</p>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              Transcribe los audios con Whisper local ({{ insights.status.asr_model }}) y después analiza el texto.
            </p>
          </div>
          <label class="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              class="peer sr-only"
              :checked="insights.status.asr_enabled"
              aria-label="Transcribir notas de voz"
              @change="insights.setASREnabled(($event.target as HTMLInputElement).checked)"
            />
            <span
              class="relative h-5 w-9 rounded-full bg-slate-300 transition-colors peer-checked:bg-emerald-500 peer-focus-visible:ring-2 peer-focus-visible:ring-emerald-400 dark:bg-slate-700"
              aria-hidden="true"
            >
              <span class="absolute top-0.5 left-0.5 h-4 w-4 rounded-full bg-white transition-transform peer-checked:translate-x-4" />
            </span>
          </label>
        </div>

        <!-- por canal -->
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <span class="text-xs text-slate-500 dark:text-slate-400">Canales:</span>
          <button
            v-for="c in insights.status.channels"
            :key="c.channel"
            class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors"
            :class="c.enabled
              ? 'bg-indigo-500/10 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300'
              : 'bg-slate-500/10 text-slate-500 dark:text-slate-400'"
            :aria-pressed="c.enabled"
            @click="insights.setChannelEnabled(c.channel, !c.enabled)"
          >
            <span class="capitalize">{{ c.channel }}</span>
            <span class="material-symbols-outlined text-[13px]" aria-hidden="true">
              {{ c.enabled ? 'toggle_on' : 'toggle_off' }}
            </span>
          </button>

          <Button
            class="ml-auto"
            size="sm"
            variant="secondary"
            :disabled="!insights.status.master_enabled"
            @click="insights.backfill()"
          >
            Analizar lo que falta
          </Button>
        </div>
      </template>
    </section>

    <div v-if="store.loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <div
        v-for="i in 3"
        :key="i"
        class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-[#101828]"
      >
        <div class="mb-4 flex items-center justify-between">
          <div class="h-5 w-28 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
          <div class="h-6 w-11 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
        </div>
        <Skeleton :lines="3" />
      </div>
    </div>

    <EmptyState
      v-else-if="store.error"
      icon="cloud_off"
      title="No se pudieron cargar los agentes"
      :description="store.error"
    >
      <template #action>
        <Button size="sm" @click="store.fetchAgents()">Reintentar</Button>
      </template>
    </EmptyState>

    <div v-else class="grid items-stretch gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article
        v-for="agent in store.agents"
        :key="agent.id"
        class="flex flex-col rounded-2xl border border-slate-200 bg-white p-5 shadow-sm transition-all duration-200 hover:shadow-md dark:border-slate-800 dark:bg-[#101828]"
      >
        <div class="flex items-center gap-2">
          <ChannelBadge :channel="agent.channel" />
          <div class="ml-auto flex items-center gap-1.5">
            <div
              :class="['transition-transform duration-200', expanded[agent.channel] ? 'rotate-180' : '']"
            >
              <IconButton icon="expand_more" size="sm" @click="toggleExpanded(agent)" />
            </div>
            <button
              type="button"
              role="switch"
              :aria-checked="agent.enabled"
              :aria-label="`Activar agente de ${channelLabel(agent.channel)}`"
              :disabled="store.saving[agent.channel]"
              :class="[
                'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 focus-visible:ring-offset-2 disabled:opacity-50 dark:focus-visible:ring-offset-[#101828]',
                agent.enabled ? 'bg-sky-400' : 'bg-slate-300 dark:bg-slate-600',
              ]"
              @click="store.toggleAgent(agent)"
            >
              <span
                :class="[
                  'inline-block h-5 w-5 rounded-full bg-white shadow transition-transform duration-200',
                  agent.enabled ? 'translate-x-[22px]' : 'translate-x-0.5',
                ]"
              />
            </button>
          </div>
        </div>

        <div class="mt-3">
          <Badge :variant="agent.enabled ? 'success' : 'default'">
            {{ agent.enabled ? 'Activo' : 'Apagado' }}
          </Badge>
        </div>

        <div class="mt-4 min-w-0">
          <p class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-400 dark:text-slate-500">
            Modelo
          </p>
          <p
            class="truncate font-mono text-sm text-slate-700 dark:text-slate-300"
            :title="agent.model"
          >
            {{ agent.model }}
          </p>
        </div>

        <div
          v-if="expanded[agent.channel] && drafts[agent.channel]"
          class="mt-4 space-y-4 border-t border-slate-100 pt-4 dark:border-slate-800"
        >
          <div>
            <label
              :for="`prompt-${agent.channel}`"
              class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-500 dark:text-slate-400"
            >
              Prompt del sistema
            </label>
            <textarea
              :id="`prompt-${agent.channel}`"
              v-model="drafts[agent.channel].system_prompt"
              rows="5"
              placeholder="Instrucciones del agente para este canal..."
              class="mt-1 w-full resize-y rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm text-slate-800 transition-all duration-200 placeholder:text-slate-400 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500"
            />
          </div>

          <div>
            <label
              :for="`model-${agent.channel}`"
              class="text-xs font-semibold uppercase tracking-[0.05em] text-slate-500 dark:text-slate-400"
            >
              Modelo
            </label>
            <input
              :id="`model-${agent.channel}`"
              v-model="drafts[agent.channel].model"
              type="text"
              placeholder="openai/gpt-4o-mini"
              class="mt-1 w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 font-mono text-sm text-slate-800 transition-all duration-200 placeholder:text-slate-400 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label
                :for="`temp-${agent.channel}`"
                class="text-[11px] font-medium text-slate-500 dark:text-slate-400"
              >
                Temperatura
              </label>
              <input
                :id="`temp-${agent.channel}`"
                v-model.number="drafts[agent.channel].temperature"
                type="number"
                step="0.1"
                min="0"
                max="2"
                class="mt-1 w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm text-slate-800 transition-all duration-200 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
              />
            </div>
            <div>
              <label
                :for="`tokens-${agent.channel}`"
                class="text-[11px] font-medium text-slate-500 dark:text-slate-400"
              >
                Max tokens
              </label>
              <input
                :id="`tokens-${agent.channel}`"
                v-model.number="drafts[agent.channel].max_tokens"
                type="number"
                min="1"
                max="8192"
                class="mt-1 w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 text-sm text-slate-800 transition-all duration-200 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100"
              />
            </div>
          </div>

          <div class="flex items-center gap-2">
            <Button
              size="sm"
              :disabled="!isDirty(agent)"
              :loading="store.saving[agent.channel]"
              @click="handleSave(agent)"
            >
              Guardar
            </Button>
            <Button size="sm" variant="ghost" @click="resetDraft(agent)">Cancelar</Button>
          </div>
        </div>

        <p class="mt-auto pt-4 text-[11px] text-slate-400 dark:text-slate-500">
          El agente responde automáticamente a mensajes entrantes cuando está activo.
        </p>
      </article>
    </div>
  </div>
</template>
