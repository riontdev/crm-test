<script setup lang="ts">
import { RouterView, RouterLink } from 'vue-router'
</script>

<template>
  <div class="min-h-screen bg-white">
    <!-- Top nav -->
    <nav class="border-b border-neutral-200 px-6 py-3">
      <div class="flex items-center gap-8">
        <h1 class="text-lg font-semibold">CRM Multicanal</h1>
        <div class="flex gap-4 text-sm">
          <RouterLink
            to="/inbox"
            class="text-neutral-500 hover:text-black transition-colors"
            active-class="text-black font-medium"
          >
            Bandeja
          </RouterLink>
          <RouterLink
            to="/agents"
            class="text-neutral-500 hover:text-black transition-colors"
            active-class="text-black font-medium"
          >
            Agentes
          </RouterLink>
        </div>
      </div>
    </nav>

    <!-- Main content -->
    <RouterView />
  </div>
</template>
