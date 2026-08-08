<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'

useHead({
  title: 'HelioBot — Dashboard',
  meta: [
    { name: 'description', content: 'Sign in to the HelioBot dashboard with your Telegram account.' }
  ]
})

interface AuthConfig {
  authorize_url: string
}

interface UserSession {
  uid: number
  usr?: string
  fnm?: string
  lnm?: string
  pic?: string
}

const config = ref<AuthConfig | null>(null)
const session = ref<UserSession | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const apiBase = useRuntimeConfig().public.apiBaseUrl as string | undefined
const baseURL = apiBase || ''

const displayName = computed(() => {
  if (!session.value) return ''
  return session.value.fnm || session.value.usr || `User ${session.value.uid}`
})

const loginUrl = computed(() => {
  if (!config.value?.authorize_url) return '#'
  return `${baseURL}${config.value.authorize_url}`
})

async function fetchAuthState() {
  try {
    const [configRes, meRes] = await Promise.all([
      $fetch<AuthConfig>(`${baseURL}/api/auth/config`, { credentials: 'include' }),
      $fetch<UserSession>(`${baseURL}/api/auth/me`, { credentials: 'include' }).catch(() => null)
    ])
    config.value = configRes
    session.value = meRes
  } catch (err) {
    error.value = 'Failed to load dashboard. Please try again later.'
  } finally {
    loading.value = false
  }
}

async function logout() {
  try {
    await $fetch(`${baseURL}/api/auth/logout`, { method: 'POST', credentials: 'include' })
  } catch {
    // Ignore network errors.
  }
  session.value = null
}

onMounted(() => {
  fetchAuthState()
})
</script>

<template>
  <main class="flex min-h-screen items-center justify-center px-6">
    <div class="text-center">
      <div class="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border border-zinc-800 bg-zinc-900/60 px-4 py-1.5 text-sm text-zinc-400">
        <span class="h-2 w-2 rounded-full bg-emerald-400" />
        Group owners only
      </div>
      <h1 class="mx-auto max-w-3xl text-5xl font-bold leading-tight tracking-tight sm:text-6xl">
        Dashboard
      </h1>

      <div v-if="loading" class="mt-10 text-zinc-400">
        Loading…
      </div>

      <div v-else-if="error" class="mx-auto mt-10 max-w-md rounded-xl border border-red-900/50 bg-red-900/20 p-4 text-red-200">
        {{ error }}
      </div>

      <template v-else-if="session">
        <p class="mx-auto mt-6 max-w-2xl text-lg leading-relaxed text-zinc-400">
          Signed in as <span class="font-semibold text-zinc-200">{{ displayName }}</span>.
        </p>
        <div class="mt-10 flex items-center justify-center gap-4">
          <button
            type="button"
            class="rounded-lg border border-zinc-800 bg-zinc-900 px-5 py-2.5 text-sm font-medium text-zinc-300 transition-colors hover:border-zinc-700 hover:text-zinc-100"
            @click="logout"
          >
            Sign out
          </button>
        </div>
      </template>

      <template v-else>
        <p class="mx-auto mt-6 max-w-2xl text-lg leading-relaxed text-zinc-400">
          Sign in with your Telegram account to manage your groups and moderation settings.
        </p>
        <div class="mt-10 flex items-center justify-center">
          <a
            :href="loginUrl"
            class="flex items-center gap-3 rounded-lg bg-sky-400 px-6 py-3 text-sm font-semibold text-zinc-950 transition-colors hover:bg-sky-300"
          >
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="h-5 w-5">
              <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
            </svg>
            Sign in with Telegram
          </a>
        </div>
        <div v-if="!config?.authorize_url" class="mt-2 text-sm text-zinc-500">
          Telegram sign-in is not configured.
        </div>
      </template>

      <div class="mt-10">
        <NuxtLink to="/" class="text-sm text-zinc-500 transition-colors hover:text-zinc-300">
          &larr; Back to home
        </NuxtLink>
      </div>
    </div>
  </main>
</template>
