<script setup lang="ts">
import { onMounted, ref } from 'vue'

useHead({
  title: 'Helio — Login Successful',
  meta: [
    { name: 'description', content: 'Telegram authentication successful. Redirecting to dashboard.' }
  ]
})

const loading = ref(true)
const error = ref<string | null>(null)

const route = useRoute()
const router = useRouter()
const apiBase = useRuntimeConfig().public.apiBaseUrl as string | undefined
const baseURL = apiBase || ''

const redirectTo = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'

onMounted(async () => {
  try {
    await $fetch(`${baseURL}/api/auth/me`, { credentials: 'include' })
    await router.replace(redirectTo)
  } catch (err) {
    loading.value = false
    error.value = 'Could not confirm your session. Please try signing in again.'
  }
})
</script>

<template>
  <main class="flex min-h-screen items-center justify-center px-6">
    <div class="text-center">
      <template v-if="loading">
        <div class="mx-auto mb-6 inline-flex h-12 w-12 items-center justify-center rounded-full border border-zinc-700 bg-zinc-800">
          <svg class="h-6 w-6 animate-spin text-sky-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
        </div>
        <h1 class="text-2xl font-semibold tracking-tight">
          Successful
        </h1>
        <p class="mt-3 text-zinc-400">
          Redirecting to the dashboard…
        </p>
      </template>

      <template v-else>
        <div class="mx-auto mb-6 inline-flex h-12 w-12 items-center justify-center rounded-full border border-red-900/50 bg-red-900/20">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="h-6 w-6 text-red-400">
            <path fill-rule="evenodd" d="M12 2.25c-5.385 0-9.75 4.365-9.75 9.75s4.365 9.75 9.75 9.75 9.75-4.365 9.75-9.75S17.385 2.25 12 2.25zm-1.72 6.97a.75.75 0 10-1.06 1.06L10.94 12l-1.72 1.72a.75.75 0 101.06 1.06L12 13.06l1.72 1.72a.75.75 0 101.06-1.06L13.06 12l1.72-1.72a.75.75 0 10-1.06-1.06L12 10.94l-1.72-1.72z" clip-rule="evenodd" />
          </svg>
        </div>
        <h1 class="text-2xl font-semibold tracking-tight">
          Login failed
        </h1>
        <p class="mt-3 text-zinc-400">
          {{ error }}
        </p>
        <div class="mt-6">
          <NuxtLink
            to="/dashboard"
            class="rounded-lg bg-sky-400 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition-colors hover:bg-sky-300"
          >
            Try again
          </NuxtLink>
        </div>
      </template>
    </div>
  </main>
</template>
