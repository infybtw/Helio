<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

useHead({
  title: 'HelioBot — Dashboard',
  meta: [
    { name: 'description', content: 'See your Telegram chats, moderation activity and bot commands.' }
  ]
})

interface AuthConfig { authorize_url: string }
interface UserSession {
  uid: number
  usr?: string
  fnm?: string
  lnm?: string
  pic?: string
}
interface DashboardChat {
  name: string
  handle: string
  members: number
  participants?: number
  actions: number
  status: string
  initials: string
}
interface DashboardActivity {
  action: string
  actor: string
  chat: string
  created_at: string
}
interface DashboardData {
  protected_chats: number
  actions_this_week: number
  messages_cleaned: number
  chats: DashboardChat[]
  activity: DashboardActivity[]
}

const config = ref<AuthConfig | null>(null)
const session = ref<UserSession | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const dashboard = ref<DashboardData | null>(null)
const activeView = ref<'overview' | 'chats' | 'activity'>('overview')

const apiBase = useRuntimeConfig().public.apiBaseUrl as string | undefined
const baseURL = apiBase || ''

const displayName = computed(() => {
  if (!session.value) return ''
  return session.value.fnm || (session.value.usr ? `@${session.value.usr}` : 'Telegram user')
})

const loginUrl = computed(() => config.value?.authorize_url ? `${baseURL}${config.value.authorize_url}` : '#')

const chats = computed(() => dashboard.value?.chats.map((chat, index) => ({
  ...chat,
  members: chat.members.toLocaleString('ru-RU'),
  commands: chat.actions.toLocaleString('ru-RU'),
  color: ['bg-orange-400', 'bg-sky-400', 'bg-emerald-400'][index % 3]
})) || [])

function formatActivityDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 'Date unavailable' : date.toLocaleString('ru-RU')
}

const activity = computed(() => dashboard.value?.activity.map((item) => ({
  command: item.action,
  user: item.actor || 'Unknown user',
  chat: item.chat,
  time: formatActivityDate(item.created_at),
  tone: item.action === '!mute' ? 'text-amber-300' : item.action === '!delete' ? 'text-rose-300' : item.action === '!grant' ? 'text-emerald-300' : 'text-violet-300'
})) || [])

async function fetchAuthState() {
  try {
    const [configRes, meRes] = await Promise.all([
      $fetch<AuthConfig>(`${baseURL}/api/auth/config`, { credentials: 'include' }),
      $fetch<UserSession>(`${baseURL}/api/auth/me`, { credentials: 'include' }).catch(() => null)
    ])
    config.value = configRes
    session.value = meRes
    if (meRes) {
      dashboard.value = await $fetch<DashboardData>(`${baseURL}/api/dashboard/overview`, { credentials: 'include' })
    }
  } catch {
    error.value = 'Не удалось загрузить дашборд. Попробуйте еще раз позже.'
  } finally {
    loading.value = false
  }
}

async function logout() {
  try {
    await $fetch(`${baseURL}/api/auth/logout`, { method: 'POST', credentials: 'include' })
  } catch {
    // The local session still needs to disappear when the network is unavailable.
  }
  session.value = null
}

onMounted(fetchAuthState)
</script>

<template>
  <main class="min-h-screen bg-[#0b0d10] text-zinc-100">
    <div v-if="loading" class="flex min-h-screen items-center justify-center text-sm text-zinc-500">Loading dashboard...</div>

    <div v-else-if="error" class="flex min-h-screen items-center justify-center px-6">
      <div class="rounded-2xl border border-rose-500/20 bg-rose-500/10 p-5 text-sm text-rose-200">{{ error }}</div>
    </div>

    <template v-else-if="!session">
      <div class="flex min-h-screen items-center justify-center px-6">
        <div class="w-full max-w-md text-center">
          <div class="mx-auto mb-6 flex h-12 w-12 items-center justify-center rounded-2xl bg-sky-400 text-xl font-black text-zinc-950">H</div>
          <p class="text-xs font-semibold uppercase tracking-[0.22em] text-sky-300">HelioBot workspace</p>
          <h1 class="mt-4 text-4xl font-bold tracking-tight">Твой центр управления чатами</h1>
          <p class="mt-4 leading-7 text-zinc-400">Войди через Telegram, чтобы видеть свои группы и действия модерации.</p>
          <a :href="loginUrl" class="mt-8 inline-flex items-center gap-3 rounded-xl bg-sky-400 px-5 py-3 text-sm font-semibold text-zinc-950 transition hover:bg-sky-300">Sign in with Telegram</a>
          <p v-if="!config?.authorize_url" class="mt-3 text-xs text-zinc-600">Telegram sign-in is not configured.</p>
          <NuxtLink to="/" class="mt-8 block text-sm text-zinc-600 hover:text-zinc-300">← Back to home</NuxtLink>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="min-h-screen lg:flex">
        <aside class="border-b border-white/[0.07] bg-[#0f1115] lg:fixed lg:inset-y-0 lg:left-0 lg:flex lg:w-64 lg:flex-col lg:border-b-0 lg:border-r">
          <div class="flex items-center justify-between px-5 py-4 lg:block lg:px-6 lg:py-7">
            <div class="flex items-center gap-3">
              <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-300 text-zinc-950">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" /></svg>
              </div>
              <div><span class="font-semibold tracking-tight">HelioBot</span><span class="block text-xs text-zinc-600">workspace</span></div>
            </div>
          </div>

          <nav class="flex gap-1 overflow-x-auto px-3 pb-3 lg:block lg:flex-1 lg:space-y-1 lg:overflow-visible lg:px-3 lg:pt-8">
            <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition lg:w-full" :class="activeView === 'overview' ? 'bg-amber-300/10 text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="activeView = 'overview'">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><rect x="3" y="3" width="7" height="7" rx="1" /><rect x="14" y="3" width="7" height="7" rx="1" /><rect x="3" y="14" width="7" height="7" rx="1" /><rect x="14" y="14" width="7" height="7" rx="1" /></svg>
              Overview
            </button>
            <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition lg:w-full" :class="activeView === 'chats' ? 'bg-amber-300/10 font-medium text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="activeView = 'chats'">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><path d="M20 11.5a7.5 7.5 0 0 1-8 7.5 8.8 8.8 0 0 1-3.3-.6L4 20l1.6-3.5A7.3 7.3 0 0 1 4.5 12 7.5 7.5 0 0 1 12 4.5c4.4 0 8 3.1 8 7Z" /></svg>
              Chats
            </button>
            <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition lg:w-full" :class="activeView === 'activity' ? 'bg-amber-300/10 font-medium text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="activeView = 'activity'">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><path d="M4 17h4V7H4v10Zm6 0h4V4h-4v13Zm6 0h4v-7h-4v7Z" /></svg>
              Activity
            </button>
          </nav>

          <div class="border-t border-white/[0.07] p-4 lg:p-5">
            <div class="mb-3 px-1 text-[10px] uppercase tracking-[0.18em] text-zinc-700">Signed in as</div>
            <div class="flex items-center justify-between gap-3">
              <span class="min-w-0 truncate text-sm text-zinc-400">{{ displayName }}</span>
              <button class="shrink-0 text-xs text-zinc-600 transition hover:text-zinc-200" type="button" @click="logout">Log out</button>
            </div>
          </div>
        </aside>

        <div class="min-w-0 flex-1 lg:ml-64">
          <div id="overview" class="w-full px-5 py-8 sm:px-8 lg:px-10 lg:py-10 xl:px-14">
            <div class="mb-9">
              <p class="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-600">Command center</p>
              <h1 v-if="activeView === 'overview'" class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Good morning, {{ displayName }}.</h1>
              <h1 v-else-if="activeView === 'chats'" class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Your chats</h1>
              <h1 v-else class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Recent activity</h1>
              <p v-if="activeView === 'overview'" class="mt-3 max-w-xl text-zinc-500">Все важное о твоих чатах, командах и модерации в одном месте.</p>
              <p v-else-if="activeView === 'chats'" class="mt-3 max-w-xl text-zinc-500">Connected Telegram groups and their moderation status.</p>
              <p v-else class="mt-3 max-w-xl text-zinc-500">A complete log of moderation actions in your workspace.</p>
            </div>

            <div v-if="activeView === 'overview'" class="space-y-6">
              <div class="grid gap-4 sm:grid-cols-3">
                <div class="rounded-2xl border border-amber-300/20 bg-gradient-to-br from-amber-300/[0.12] to-transparent p-5"><p class="text-xs text-amber-200/70">Protected chats</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.protected_chats || 0 }}</p><p class="mt-2 text-xs text-zinc-500">All systems operational</p></div>
                <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Actions this week</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.actions_this_week || 0 }}</p><p class="mt-2 text-xs text-emerald-300">From the action log</p></div>
                <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Messages cleaned</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.messages_cleaned || 0 }}</p><p class="mt-2 text-xs text-zinc-600">All time</p></div>
              </div>

              <div class="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.65fr)]">
                <section id="chats" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div class="flex items-center justify-between"><div><h2 class="font-semibold">Your chats</h2><p class="mt-1 text-sm text-zinc-600">Connected Telegram groups</p></div><button class="rounded-lg border border-white/10 px-3 py-2 text-xs text-zinc-400 hover:text-white">Manage chats</button></div><div class="mt-6 divide-y divide-white/[0.06]"><div v-for="chat in chats" :key="chat.name" class="flex items-center justify-between gap-4 py-4 first:pt-0"><div class="flex min-w-0 items-center gap-3"><div :class="chat.color" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-xs font-bold text-zinc-950">{{ chat.initials }}</div><div class="min-w-0"><p class="truncate text-sm font-medium">{{ chat.name }}</p><p class="truncate text-xs text-zinc-600">{{ chat.handle }} · {{ chat.members }} participants</p></div></div><div class="hidden text-right sm:block"><p class="font-mono text-sm text-zinc-300">{{ chat.commands }}</p><p class="text-[10px] uppercase tracking-wider text-zinc-600">actions / 7d</p></div><span class="h-2 w-2 shrink-0 rounded-full" :class="chat.status === 'Healthy' ? 'bg-emerald-400' : 'bg-amber-300'"></span></div></div></section>
                <section id="activity" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div class="flex items-center justify-between"><div><h2 class="font-semibold">Recent activity</h2><p class="mt-1 text-sm text-zinc-600">Moderation across your workspace</p></div><span class="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_12px_theme(colors.emerald.400)]"></span></div><div class="mt-6 space-y-5"><div v-for="item in activity" :key="item.command + item.time" class="flex gap-3"><div class="mt-1 h-7 w-7 shrink-0 rounded-lg bg-white/[0.06] text-center text-[10px] leading-7 text-zinc-500">↗</div><div class="min-w-0"><p class="text-sm"><code :class="item.tone" class="font-mono text-xs">{{ item.command }}</code> <span class="text-zinc-500">by {{ item.user }}</span></p><p class="mt-1 truncate text-xs text-zinc-600">{{ item.chat }} · {{ item.time }}</p></div></div></div><button class="mt-6 w-full border-t border-white/[0.06] pt-4 text-left text-xs text-zinc-500 hover:text-zinc-200">View all activity →</button></section>
              </div>
            </div>

            <div v-else-if="activeView === 'chats'" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              <article v-for="chat in chats" :key="chat.name" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6 transition hover:border-white/[0.14]">
                <div class="flex items-start justify-between gap-4">
                  <div :class="chat.color" class="flex h-11 w-11 items-center justify-center rounded-xl text-xs font-bold text-zinc-950">{{ chat.initials }}</div>
                  <span class="flex items-center gap-2 text-xs text-zinc-500"><span class="h-2 w-2 rounded-full" :class="chat.status === 'Healthy' ? 'bg-emerald-400' : 'bg-amber-300'"></span>{{ chat.status }}</span>
                </div>
                <h2 class="mt-7 truncate text-lg font-semibold">{{ chat.name }}</h2>
                <p class="mt-1 truncate text-sm text-zinc-600">{{ chat.handle || 'Private Telegram group' }}</p>
                <div class="mt-7 grid grid-cols-2 gap-3 border-t border-white/[0.07] pt-4">
                  <div><p class="text-xl font-semibold text-zinc-200">{{ chat.members }}</p><p class="mt-1 text-[10px] uppercase tracking-wider text-zinc-600">participants</p></div>
                  <div><p class="text-xl font-semibold text-zinc-200">{{ chat.commands }}</p><p class="mt-1 text-[10px] uppercase tracking-wider text-zinc-600">actions / 7d</p></div>
                </div>
              </article>
              <div v-if="!chats.length" class="rounded-2xl border border-dashed border-white/[0.1] p-8 text-sm text-zinc-600 md:col-span-2 xl:col-span-3">No tracked chats yet.</div>
            </div>

            <div v-else class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6">
              <div class="divide-y divide-white/[0.06]">
                <div v-for="item in activity" :key="item.command + item.time" class="flex items-center gap-4 py-5 first:pt-0">
                  <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white/[0.06] text-xs text-zinc-500">↗</div>
                  <div class="min-w-0 flex-1"><p class="text-sm"><code :class="item.tone" class="font-mono text-xs">{{ item.command }}</code> <span class="text-zinc-500">by {{ item.user }}</span></p><p class="mt-1 truncate text-xs text-zinc-600">{{ item.chat }}</p></div>
                  <time class="shrink-0 text-xs text-zinc-600">{{ item.time }}</time>
                </div>
                <div v-if="!activity.length" class="py-8 text-sm text-zinc-600">No activity recorded yet.</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </main>
</template>
