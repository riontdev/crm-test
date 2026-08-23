<script setup lang="ts">
import { onMounted, reactive, watch } from 'vue'
import { useAgentsStore } from '@/stores/agents'
import type { AgentConfig } from '@/lib/api'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import IconButton from '@/components/ui/IconButton.vue'
import ChannelBadge from '@/components/ui/ChannelBadge.vue'

const store = useAgentsStore()

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

onMounted(() => {
  store.fetchAgents()
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
