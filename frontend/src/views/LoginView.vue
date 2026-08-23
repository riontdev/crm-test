<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()

const email = ref('')
const password = ref('')
const errorMsg = ref<string | null>(null)

onMounted(async () => {
  if (!auth.initialized) await auth.init()
  if (auth.isAuthenticated) router.replace('/inbox')
})

async function onSubmit() {
  errorMsg.value = null
  const err = await auth.login(email.value.trim(), password.value)
  if (err === null) {
    const redirect = router.currentRoute.value.query.redirect
    router.replace(typeof redirect === 'string' && redirect.startsWith('/') ? redirect : '/inbox')
  } else {
    errorMsg.value = err
  }
}
</script>

<template>
  <div class="flex min-h-screen">
    <!-- Panel de marca (solo desktop) -->
    <div
      class="relative hidden flex-1 items-center justify-center overflow-hidden bg-slate-900 p-12 lg:flex dark:bg-[#0B1220]"
    >
      <div
        class="absolute -top-24 -left-24 h-96 w-96 rounded-full bg-sky-500/10 blur-3xl"
        aria-hidden="true"
      />
      <div
        class="absolute right-0 bottom-0 h-[28rem] w-[28rem] translate-x-1/3 translate-y-1/3 rounded-full bg-sky-500/10 blur-3xl"
        aria-hidden="true"
      />

      <div class="relative z-10 flex flex-col items-center">
        <div
          class="flex h-14 w-14 items-center justify-center rounded-xl bg-sky-400 shadow-lg shadow-sky-400/20"
        >
          <span class="material-symbols-outlined text-3xl text-slate-900" aria-hidden="true">hub</span>
        </div>
        <h2 class="mt-6 text-3xl font-bold tracking-tight text-white">SocialCRM</h2>
        <p class="mt-2 text-xs uppercase tracking-widest text-slate-400">Enterprise Hub</p>

        <div class="mt-8 flex items-center gap-6 text-sm text-slate-400">
          <span class="flex items-center gap-1.5">
            <span class="material-symbols-outlined text-lg" aria-hidden="true">inbox</span>
            Bandeja unificada
          </span>
          <span class="flex items-center gap-1.5">
            <span class="material-symbols-outlined text-lg" aria-hidden="true">hub</span>
            Multicanal
          </span>
          <span class="flex items-center gap-1.5">
            <span class="material-symbols-outlined text-lg" aria-hidden="true">smart_toy</span>
            Agentes IA
          </span>
        </div>
      </div>
    </div>

    <!-- Panel del formulario -->
    <div class="flex flex-1 items-center justify-center bg-[#F8F9FF] p-6 dark:bg-[#0B1220]">
      <div
        class="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-8 shadow-xl dark:border-slate-800 dark:bg-[#101828]"
      >
        <!-- Logo compacto (solo mobile) -->
        <div class="mb-6 flex items-center gap-2.5 lg:hidden">
          <div
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-900 dark:bg-sky-400"
          >
            <span
              class="material-symbols-outlined text-white dark:text-slate-900"
              aria-hidden="true"
              >hub</span
            >
          </div>
          <div>
            <p class="text-sm font-bold text-slate-900 dark:text-white">SocialCRM</p>
            <p class="text-[11px] uppercase tracking-wider text-slate-400">Enterprise Hub</p>
          </div>
        </div>

        <h1 class="text-xl font-semibold text-slate-900 dark:text-white">Ingresá a tu cuenta</h1>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Gestioná tus conversaciones multicanal
        </p>

        <form class="mt-8 space-y-4" @submit.prevent="onSubmit">
          <div>
            <label
              for="login-email"
              class="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400"
              >Email</label
            >
            <input
              id="login-email"
              v-model="email"
              type="email"
              required
              autocomplete="email"
              placeholder="tu@empresa.com"
              class="w-full rounded-lg border border-slate-300 bg-slate-50 px-4 py-2.5 text-sm text-slate-800 transition-all duration-200 placeholder:text-slate-400 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500"
            />
          </div>

          <div>
            <label
              for="login-password"
              class="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400"
              >Contraseña</label
            >
            <input
              id="login-password"
              v-model="password"
              type="password"
              required
              autocomplete="current-password"
              placeholder="••••••••"
              class="w-full rounded-lg border border-slate-300 bg-slate-50 px-4 py-2.5 text-sm text-slate-800 transition-all duration-200 placeholder:text-slate-400 focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/40 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500"
            />
          </div>

          <p v-if="errorMsg" role="alert" class="text-xs text-red-500">{{ errorMsg }}</p>

          <button
            type="submit"
            :disabled="auth.loading"
            class="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-sky-400 px-4 py-2.5 text-sm font-medium text-white transition-colors duration-150 shadow-sm hover:bg-sky-500 active:bg-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 dark:focus-visible:ring-offset-[#101828]"
          >
            <span
              v-if="auth.loading"
              class="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white"
              aria-hidden="true"
            />
            Iniciar sesión
          </button>
        </form>

        <p class="mt-6 text-center text-[11px] text-slate-400 dark:text-slate-500">
          ¿Problemas para ingresar? Contactá al administrador.
        </p>
      </div>
    </div>
  </div>
</template>
