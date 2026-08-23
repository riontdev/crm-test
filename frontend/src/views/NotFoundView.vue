<script setup lang="ts">
import { RouterLink } from 'vue-router'
import Button from '@/components/ui/Button.vue'
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center gap-4 p-8 text-center">
    <p class="text-7xl font-bold text-slate-200 dark:text-slate-700">404</p>
    <h1 class="text-xl font-semibold text-slate-900 dark:text-white">Página no encontrada</h1>
    <p class="max-w-md text-sm text-slate-500 dark:text-slate-400">
      La página que buscás no existe o fue movida.
    </p>
    <RouterLink to="/inbox">
      <Button variant="primary">Volver a la Bandeja</Button>
    </RouterLink>
  </div>
</template>
