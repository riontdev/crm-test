import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type ChannelStatus } from '@/lib/api'

const CHANNEL_ORDER = ['whatsapp', 'instagram', 'facebook']

export const useChannelsStore = defineStore('channels', () => {
  const channels = ref<ChannelStatus[]>([])
  const webhookUrl = ref('')
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchStatus() {
    loading.value = true
    error.value = null
    try {
      const res = await api.channelsStatus()
      channels.value = [...(res.channels ?? [])].sort(
        (a, b) => CHANNEL_ORDER.indexOf(a.channel) - CHANNEL_ORDER.indexOf(b.channel),
      )
      webhookUrl.value = res.webhook_url || ''
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  return { channels, webhookUrl, loading, error, fetchStatus }
})
