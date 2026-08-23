<script setup lang="ts">
withDefaults(defineProps<{ show?: boolean }>(), { show: true })

const DOT_DELAYS = ['0ms', '150ms', '300ms']
</script>

<template>
  <div v-if="show" class="flex justify-start" aria-label="El contacto está escribiendo" role="status">
    <div class="rounded-2xl rounded-bl-md bg-slate-100 px-4 py-3 shadow-sm dark:bg-slate-800">
      <div class="flex items-center gap-1.5">
        <span
          v-for="(delay, i) in DOT_DELAYS"
          :key="i"
          class="h-1.5 w-1.5 animate-bounce rounded-full bg-slate-400 dark:bg-slate-500"
          :style="{ animationDelay: delay }"
        />
      </div>
      <p class="mt-1 text-center text-xs text-slate-400 dark:text-slate-500">escribiendo...</p>
    </div>
  </div>
</template>
