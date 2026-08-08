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

const apiBase = useRuntimeConfig().public.apiBaseUrl as string | undefined
const baseURL = apiBase || ''

const displayName = computed(() => {
  if (!session.value) return ''
  return session.value.fnm || (session.value.usr ? `@${session.value.usr}` : 'Telegram user')
})

const loginUrl = computed(() => config.value?.authorize_url ? `${baseURL}${config.value.authorize_url}` : '#')

const currentDesign = { id: 'command' } as const

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
      <header class="border-b border-white/[0.07] bg-[#0b0d10]/90 backdrop-blur-xl">
        <nav class="mx-auto flex max-w-[1440px] items-center justify-between px-5 py-4 lg:px-9">
          <div class="flex items-center gap-3">
            <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-300 text-sm font-black text-zinc-950">H</div>
            <div><span class="font-semibold tracking-tight">HelioBot</span><span class="ml-2 hidden text-xs text-zinc-600 sm:inline">/ workspace</span></div>
          </div>
          <div class="flex items-center gap-3">
            <span class="hidden text-sm text-zinc-500 sm:inline">{{ displayName }}</span>
            <button class="text-xs text-zinc-600 transition hover:text-zinc-300" type="button" @click="logout">Log out</button>
          </div>
        </nav>
      </header>

      <div class="mx-auto max-w-[1440px] px-5 py-8 lg:px-9 lg:py-10">
        <div class="mb-9 flex flex-col justify-between gap-6 xl:flex-row xl:items-end">
          <div>
            <p class="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-600">Command center</p>
            <h1 class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Good morning, {{ displayName }}.</h1>
            <p class="mt-3 max-w-xl text-zinc-500">Все важное о твоих чатах, командах и модерации в одном месте.</p>
          </div>
        </div>

        <div v-if="currentDesign.id === 'command'" class="space-y-6">
          <div class="grid gap-4 sm:grid-cols-3">
            <div class="rounded-2xl border border-amber-300/20 bg-gradient-to-br from-amber-300/[0.12] to-transparent p-5"><p class="text-xs text-amber-200/70">Protected chats</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.protected_chats || 0 }}</p><p class="mt-2 text-xs text-zinc-500">All systems operational</p></div>
            <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Actions this week</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.actions_this_week || 0 }}</p><p class="mt-2 text-xs text-emerald-300">From the action log</p></div>
            <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Messages cleaned</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.messages_cleaned || 0 }}</p><p class="mt-2 text-xs text-zinc-600">All time</p></div>
          </div>
          <div class="grid gap-6 lg:grid-cols-[1.25fr_0.75fr]">
            <section class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div class="flex items-center justify-between"><div><h2 class="font-semibold">Your chats</h2><p class="mt-1 text-sm text-zinc-600">Connected Telegram groups</p></div><button class="rounded-lg border border-white/10 px-3 py-2 text-xs text-zinc-400 hover:text-white">Manage chats</button></div><div class="mt-6 divide-y divide-white/[0.06]"><div v-for="chat in chats" :key="chat.name" class="flex items-center justify-between gap-4 py-4 first:pt-0"><div class="flex min-w-0 items-center gap-3"><div :class="chat.color" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-xs font-bold text-zinc-950">{{ chat.initials }}</div><div class="min-w-0"><p class="truncate text-sm font-medium">{{ chat.name }}</p><p class="truncate text-xs text-zinc-600">{{ chat.handle }} · {{ chat.members }} participants</p></div></div><div class="hidden text-right sm:block"><p class="font-mono text-sm text-zinc-300">{{ chat.commands }}</p><p class="text-[10px] uppercase tracking-wider text-zinc-600">actions / 7d</p></div><span class="h-2 w-2 shrink-0 rounded-full" :class="chat.status === 'Healthy' ? 'bg-emerald-400' : 'bg-amber-300'"></span></div></div></section>
            <section class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div class="flex items-center justify-between"><div><h2 class="font-semibold">Recent activity</h2><p class="mt-1 text-sm text-zinc-600">Moderation across your workspace</p></div><span class="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_12px_theme(colors.emerald.400)]"></span></div><div class="mt-6 space-y-5"><div v-for="item in activity" :key="item.command + item.time" class="flex gap-3"><div class="mt-1 h-7 w-7 shrink-0 rounded-lg bg-white/[0.06] text-center text-[10px] leading-7 text-zinc-500">↗</div><div class="min-w-0"><p class="text-sm"><code :class="item.tone" class="font-mono text-xs">{{ item.command }}</code> <span class="text-zinc-500">by {{ item.user }}</span></p><p class="mt-1 truncate text-xs text-zinc-600">{{ item.chat }} · {{ item.time }}</p></div></div></div><button class="mt-6 w-full border-t border-white/[0.06] pt-4 text-left text-xs text-zinc-500 hover:text-zinc-200">View all activity →</button></section>
          </div>
        </div>

        <div v-else-if="currentDesign.id === 'orbit'" class="grid gap-6 lg:grid-cols-[0.8fr_1.2fr]">
          <section class="relative overflow-hidden rounded-[2rem] border border-sky-300/20 bg-gradient-to-br from-sky-400/[0.18] via-[#111820] to-[#111418] p-7 sm:p-9"><div class="absolute -right-12 -top-12 h-48 w-48 rounded-full border border-sky-300/20"></div><div class="absolute -right-2 top-0 h-32 w-32 rounded-full border border-sky-300/10"></div><p class="text-sm text-sky-200/70">Workspace overview</p><h2 class="mt-16 max-w-xs text-4xl font-semibold leading-tight tracking-[-0.04em]">Your communities are in orbit.</h2><p class="mt-5 max-w-sm text-sm leading-6 text-zinc-400">3 connected chats are protected by HelioBot. Everything is quiet today.</p><div class="mt-10 flex items-end gap-8"><div><p class="text-5xl font-semibold">98.6%</p><p class="mt-2 text-xs text-zinc-500">protection score</p></div><div class="mb-1 text-xs text-emerald-300">↑ 4.2%</div></div></section>
          <section class="rounded-[2rem] border border-white/[0.07] bg-[#111418] p-7 sm:p-9"><div class="flex items-end justify-between"><div><p class="text-sm text-zinc-500">Connected chats</p><h2 class="mt-2 text-2xl font-semibold">A calm command center</h2></div><span class="text-sm text-sky-300">03 active</span></div><div class="mt-8 grid gap-3 sm:grid-cols-3"><div v-for="chat in chats" :key="chat.name" class="rounded-2xl bg-white/[0.04] p-4 transition hover:bg-white/[0.07]"><div :class="chat.color" class="mb-10 flex h-10 w-10 items-center justify-center rounded-full text-xs font-bold text-zinc-950">{{ chat.initials }}</div><p class="truncate text-sm font-medium">{{ chat.name }}</p><p class="mt-1 text-xs text-zinc-600">{{ chat.members }} members</p><div class="mt-5 flex justify-between border-t border-white/[0.07] pt-3 text-xs"><span class="text-zinc-600">actions</span><span class="text-zinc-300">{{ chat.commands }}</span></div></div></div><div class="mt-8 rounded-2xl border border-white/[0.07] p-5"><div class="flex justify-between text-sm"><span>Weekly moderation</span><span class="text-zinc-500">343 actions</span></div><div class="mt-5 flex h-20 items-end gap-2"> <span v-for="height in [32,46,40,64,52,78,58,88,70,92,76,100]" :key="height" class="flex-1 rounded-t-md bg-sky-300/70" :style="{ height: `${height}%` }"></span></div><div class="mt-2 flex justify-between text-[10px] text-zinc-700"><span>Mon</span><span>Today</span></div></div></section>
        </div>

        <div v-else class="space-y-6">
          <div class="grid gap-4 lg:grid-cols-[1.5fr_0.5fr_0.5fr]"><section class="rounded-2xl border border-violet-300/20 bg-violet-300/[0.1] p-6"><p class="text-xs uppercase tracking-wider text-violet-200/70">Moderation pulse</p><div class="mt-4 flex items-end justify-between"><div><p class="text-5xl font-semibold tracking-tight">343</p><p class="mt-2 text-sm text-zinc-400">actions in the last 7 days</p></div><div class="flex items-end gap-1"><span v-for="height in [30,48,35,65,52,78,58,90,70,80,65,100]" :key="height" class="w-2 rounded-full bg-violet-300" :style="{ height: `${height / 2}px` }"></span></div></div></section><div class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><p class="text-xs text-zinc-500">Response time</p><p class="mt-5 text-3xl font-semibold">1.2s</p><p class="mt-2 text-xs text-emerald-300">Excellent</p></div><div class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><p class="text-xs text-zinc-500">Commands</p><p class="mt-5 text-3xl font-semibold">5</p><p class="mt-2 text-xs text-zinc-600">available to team</p></div></div>
          <section class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div class="flex items-center justify-between"><div><h2 class="font-semibold">Chats & command access</h2><p class="mt-1 text-sm text-zinc-600">Who can moderate what</p></div><button class="rounded-lg bg-violet-300 px-3 py-2 text-xs font-semibold text-zinc-950 hover:bg-violet-200">Add chat</button></div><div class="mt-6 overflow-x-auto"><table class="w-full min-w-[620px] text-left text-sm"><thead class="text-[10px] uppercase tracking-wider text-zinc-600"><tr><th class="pb-3 font-medium">Chat</th><th class="pb-3 font-medium">Members</th><th class="pb-3 font-medium">7d actions</th><th class="pb-3 font-medium">Status</th><th class="pb-3 font-medium">Access</th></tr></thead><tbody class="divide-y divide-white/[0.06]"><tr v-for="chat in chats" :key="chat.name"><td class="py-4"><div class="flex items-center gap-3"><div :class="chat.color" class="flex h-8 w-8 items-center justify-center rounded-lg text-[10px] font-bold text-zinc-950">{{ chat.initials }}</div><span>{{ chat.name }}</span></div></td><td class="py-4 text-zinc-500">{{ chat.members }}</td><td class="py-4 font-mono text-zinc-300">{{ chat.commands }}</td><td class="py-4"><span class="rounded-full bg-emerald-400/10 px-2 py-1 text-[10px] text-emerald-300">{{ chat.status }}</span></td><td class="py-4 text-zinc-500">Owner + 4</td></tr></tbody></table></div></section>
        </div>
      </div>
    </template>
  </main>
</template>
