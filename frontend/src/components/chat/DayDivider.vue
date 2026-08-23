<script setup lang="ts">
defineProps<{ label: string }>()
</script>

<template>
  <div class="my-4 flex items-center gap-3" role="separator" :aria-label="`Separador de día: ${label}`">
    <div class="h-px flex-1 bg-slate-200 dark:bg-slate-700"></div>
    <span
      class="rounded-full bg-white/80 px-3 py-1 text-[11px] font-medium uppercase tracking-wider text-slate-400 backdrop-blur dark:bg-[#101828]/80 dark:text-slate-500"
      >{{ label }}</span
    >
    <div class="h-px flex-1 bg-slate-200 dark:bg-slate-700"></div>
  </div>
</template>
