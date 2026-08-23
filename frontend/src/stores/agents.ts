import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type AgentConfig } from '@/lib/api'
import { useUiStore } from '@/stores/ui'

export interface AgentPatch {
  enabled?: boolean
  model?: string
  system_prompt?: string | null
  temperature?: number
  max_tokens?: number
}

const channelLabels: Record<string, string> = {
  whatsapp: 'WhatsApp',
  instagram: 'Instagram',
  facebook: 'Messenger',
}

function channelLabel(channel: string): string {
  return (
    channelLabels[channel] || channel.charAt(0).toUpperCase() + channel.slice(1)
  )
}

export const useAgentsStore = defineStore('agents', () => {
  const ui = useUiStore()
  const agents = ref<AgentConfig[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const saving = ref<Record<string, boolean>>({})

  function setSaving(channel: string, value: boolean) {
    saving.value = { ...saving.value, [channel]: value }
  }

  async function fetchAgents() {
    loading.value = true
    error.value = null
    try {
      const res = await api.listAgents()
      agents.value = res.data || []
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function toggleAgent(agent: AgentConfig) {
    const channel = agent.channel
    const previous = agent.enabled
    const next = !previous

    agent.enabled = next
    setSaving(channel, true)

    try {
      await api.updateAgent(channel, { enabled: next })
      ui.success(
        `Agente de ${channelLabel(channel)} ${next ? 'activado' : 'desactivado'}`,
      )
    } catch (e: any) {
      agent.enabled = previous
      ui.error(e.message || 'No se pudo actualizar el agente')
    } finally {
      setSaving(channel, false)
    }
  }

  async function saveAgent(channel: string, patch: AgentPatch) {
    const index = agents.value.findIndex((a) => a.channel === channel)
    if (index === -1) return

    const previous = agents.value[index]
    const optimistic: AgentConfig = { ...previous }
    if (patch.model !== undefined) optimistic.model = patch.model
    if (patch.temperature !== undefined) optimistic.temperature = patch.temperature
    if (patch.max_tokens !== undefined) optimistic.max_tokens = patch.max_tokens
    if (patch.system_prompt !== undefined) {
      optimistic.system_prompt =
        patch.system_prompt === null ? undefined : patch.system_prompt
    }

    agents.value[index] = optimistic
    setSaving(channel, true)

    try {
      const updated = await api.updateAgent(
        channel,
        { ...patch } as Parameters<typeof api.updateAgent>[1],
      )
      agents.value[index] = updated
      ui.success('Configuración guardada')
    } catch (e: any) {
      agents.value[index] = previous
      ui.error(e.message || 'No se pudo guardar la configuración')
    } finally {
      setSaving(channel, false)
    }
  }

  return { agents, loading, error, saving, fetchAgents, toggleAgent, saveAgent }
})
