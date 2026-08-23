<script setup lang="ts">
import { onMounted } from 'vue'
import { useAgentsStore } from '@/stores/agents'

const store = useAgentsStore()

onMounted(() => {
  store.fetchAgents()
})

const channelLabels: Record<string, string> = {
  whatsapp: 'WhatsApp',
  instagram: 'Instagram',
  facebook: 'Facebook Messenger',
}

const channelColors: Record<string, string> = {
  whatsapp: 'border-green-300 bg-green-50',
  instagram: 'border-pink-300 bg-pink-50',
  facebook: 'border-blue-300 bg-blue-50',
}
</script>

<template>
  <div class="max-w-3xl mx-auto p-8">
    <h2 class="text-xl font-semibold mb-6">Configuración de Agentes IA</h2>

    <div v-if="store.loading" class="text-neutral-400 text-sm">Cargando...</div>

    <div v-else-if="store.error" class="text-red-500 text-sm">{{ store.error }}</div>

    <div v-else class="space-y-4">
      <div
        v-for="agent in store.agents"
        :key="agent.id"
        :class="[
          'border rounded-lg p-6',
          channelColors[agent.channel] || 'border-neutral-200 bg-white'
        ]"
      >
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="font-medium text-lg capitalize">
              {{ channelLabels[agent.channel] || agent.channel }}
            </h3>
            <p class="text-xs text-neutral-500 mt-0.5">
              Modelo: {{ agent.model }}
            </p>
          </div>
          <div
            :class="[
              'px-3 py-1 rounded-full text-xs font-medium',
              agent.enabled
                ? 'bg-green-100 text-green-700'
                : 'bg-neutral-100 text-neutral-500'
            ]"
          >
            {{ agent.enabled ? 'Activo' : 'Inactivo' }}
          </div>
        </div>

        <div v-if="agent.system_prompt" class="mt-3">
          <label class="text-xs font-medium text-neutral-600 block mb-1">System Prompt</label>
          <div class="bg-white border border-neutral-200 rounded p-3 text-xs text-neutral-700 max-h-32 overflow-y-auto whitespace-pre-wrap">
            {{ agent.system_prompt }}
          </div>
        </div>

        <div class="flex gap-4 mt-3 text-xs text-neutral-500">
          <span>Temperatura: {{ agent.temperature }}</span>
          <span>Max tokens: {{ agent.max_tokens }}</span>
        </div>
      </div>
    </div>

    <p class="text-xs text-neutral-400 mt-6">
      Los agentes se configuran desde la base de datos (tabla agent_configs).
      Un agente apagado no responde a mensajes entrantes.
    </p>
  </div>
</template>
