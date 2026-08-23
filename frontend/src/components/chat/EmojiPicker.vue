<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const emit = defineEmits<{ select: [emoji: string]; close: [] }>()

interface Category {
  name: string
  emojis: string[]
}

const CATEGORIES: Category[] = [
  {
    name: 'Caras',
    emojis: [
      '😀', '😁', '😂', '🤣', '😊', '😍', '🥰', '😘', '😎', '🤩',
      '🥳', '😢', '😭', '😡', '🤔', '🤗', '🤫', '😴', '🤒', '🤯',
      '😅', '🙂', '🙃', '😉',
    ],
  },
  {
    name: 'Gestos',
    emojis: [
      '👍', '👎', '👏', '🙌', '🤝', '👋', '💪', '🙏', '✌️', '👌',
      '🤞', '✋', '🖐️', '👇', '☝️',
    ],
  },
  {
    name: 'Símbolos',
    emojis: [
      '❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '🤍', '💯', '✅',
      '❌', '⚡', '🔥', '✨', '🎉', '🎊', '🎁', '☕', '🍕', '🍺',
      '🚀', '⭐', '❤️‍🔥', '💔', '💌',
    ],
  },
  {
    name: 'Otros',
    emojis: [
      '📱', '💻', '📸', '🎵', '📎', '📅', '⏰', '💬', '📦', '🛒',
      '💰', '🏆', '🐶', '🐱', '🌞', '🌙',
    ],
  },
]

const KEYWORDS: Record<string, string> = {
  '😀': 'hola sonrisa feliz alegre',
  '😂': 'risa llorar gracioso jaja',
  '🤣': 'risa gracioso jaja',
  '😊': 'sonrisa feliz tierno',
  '😍': 'amor ojos corazón encantado',
  '🥰': 'amor enamorado corazones',
  '😘': 'beso amor',
  '😎': 'genial lentes cool',
  '🥳': 'fiesta celebrar cumpleaños',
  '😢': 'llorar triste',
  '😭': 'llorar triste',
  '😡': 'enojado rabia molesto',
  '😴': 'dormir sueño cansado',
  '👍': 'ok bien pulgar sí aprobado',
  '👎': 'mal pulgar no',
  '👏': 'aplauso felicidades bravo',
  '🤝': 'trato acuerdo manos',
  '👋': 'hola chao saludo',
  '💪': 'fuerza músculo ánimo',
  '🙏': 'gracias por favor rezar',
  '❤️': 'amor corazón rojo',
  '🔥': 'fuego genial caliente',
  '✨': 'brillo magia estrellas',
  '🎉': 'fiesta celebrar festejar',
  '✅': 'check listo sí correcto',
  '❌': 'cruz no incorrecto',
}

const query = ref('')

const filteredCategories = computed<Category[]>(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return CATEGORIES
  return CATEGORIES.map((cat) => ({
    ...cat,
    emojis: cat.emojis.filter((e) => e.includes(q) || (KEYWORDS[e] ?? '').includes(q)),
  })).filter((cat) => cat.emojis.length > 0)
})

function pick(emoji: string): void {
  emit('select', emoji)
  emit('close')
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div
    class="absolute bottom-full left-0 z-30 mb-2 w-72 rounded-xl border border-slate-200 bg-white shadow-lg dark:border-slate-800 dark:bg-[#101828]"
    role="dialog"
    aria-label="Selector de emojis"
  >
    <div class="border-b border-slate-100 p-2 dark:border-slate-800">
      <input
        v-model="query"
        type="text"
        placeholder="Buscar emoji..."
        class="w-full rounded-lg bg-slate-100 px-3 py-1.5 text-sm text-slate-700 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-sky-400 dark:bg-slate-800 dark:text-slate-200"
        aria-label="Buscar emoji"
      />
    </div>

    <div v-if="filteredCategories.length" class="max-h-48 space-y-1 overflow-y-auto p-2">
      <div v-for="cat in filteredCategories" :key="cat.name">
        <p class="px-1 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
          {{ cat.name }}
        </p>
        <div class="grid grid-cols-8 gap-1 p-1">
          <button
            v-for="emoji in cat.emojis"
            :key="emoji"
            type="button"
            class="h-8 w-8 rounded-md text-xl leading-none transition-colors hover:bg-slate-100 dark:hover:bg-slate-800"
            :aria-label="`Insertar ${KEYWORDS[emoji]?.split(' ')[0] ?? 'emoji'}`"
            @click="pick(emoji)"
          >
            {{ emoji }}
          </button>
        </div>
      </div>
    </div>
    <p v-else class="p-4 text-center text-xs text-slate-400 dark:text-slate-500">Sin resultados</p>
  </div>
</template>
