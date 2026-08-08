<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'

useHead({
  title: 'Helio — Dashboard',
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
  chat_id: number
  name: string
  handle: string
  members: number
  participants?: number
  actions: number
  status: string
  initials: string
}
interface CustomCommand {
  id: number
  chat_id: number
  name: string
  response: string
  created_at: string
}
interface DashboardActivity {
  action: string
  event_type: 'custom' | 'moderation' | 'info'
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
type DashboardView = 'overview' | 'chats' | 'activity' | 'commands'
const route = useRoute()
const router = useRouter()
const initialView: DashboardView = route.query.view === 'chats' || route.query.view === 'activity' || route.query.view === 'commands' ? route.query.view : 'overview'
const activeView = ref<DashboardView>(initialView)
const commands = ref<CustomCommand[]>([])
const commandName = ref('')
const commandResponse = ref('')
const commandChatID = ref<number | null>(null)
const commandError = ref<string | null>(null)
const savingCommand = ref(false)
const commandModalOpen = ref(false)
const refreshingActivity = ref(false)
const activityUpdatedAt = ref<number | null>(null)
const activityClock = ref(Date.now())
const activityType = ref<'all' | DashboardActivity['event_type']>('all')
let activityTimer: ReturnType<typeof setInterval> | undefined

function navigateToView(view: DashboardView) {
  activeView.value = view
  router.replace({ query: view === 'overview' ? {} : { view } })
  if (view === 'activity' && session.value) {
    refreshActivity()
  }
}

watch(() => route.query.view, (view) => {
  const nextView = view === 'chats' || view === 'activity' || view === 'commands' ? view : 'overview'
  activeView.value = nextView
  if (nextView === 'activity' && session.value) {
    refreshActivity()
  }
})

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

const activity = computed(() => dashboard.value?.activity
  .filter((item) => activityType.value === 'all' || item.event_type === activityType.value)
  .map((item) => ({
  command: item.action,
  type: item.event_type,
  user: item.actor || 'Unknown user',
  chat: item.chat,
  time: formatActivityDate(item.created_at),
  tone: item.action === '!mute' ? 'text-amber-300' : item.action === '!delete' ? 'text-rose-300' : item.action === '!grant' ? 'text-emerald-300' : 'text-violet-300'
})) || [])

async function refreshDashboard() {
  try {
    dashboard.value = await $fetch<DashboardData>(`${baseURL}/api/dashboard/overview`, { credentials: 'include' })
    activityUpdatedAt.value = Date.now()
  } catch {
    error.value = 'Не удалось обновить активность. Попробуйте еще раз позже.'
  }
}

async function refreshActivity() {
  if (refreshingActivity.value) return
  refreshingActivity.value = true
  await refreshDashboard()
  refreshingActivity.value = false
}

const activityAge = computed(() => {
  if (!activityUpdatedAt.value) return 'нет данных'
  const seconds = Math.max(0, Math.floor((activityClock.value - activityUpdatedAt.value) / 1000))
  if (seconds < 5) return 'только что'
  if (seconds < 60) return `${seconds} сек. назад`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} мин. назад`
  return `${Math.floor(minutes / 60)} ч. назад`
})

async function fetchAuthState() {
  try {
    const [configRes, meRes] = await Promise.all([
      $fetch<AuthConfig>(`${baseURL}/api/auth/config`, { credentials: 'include' }),
      $fetch<UserSession>(`${baseURL}/api/auth/me`, { credentials: 'include' }).catch(() => null)
    ])
    config.value = configRes
    session.value = meRes
    if (meRes) {
      await refreshDashboard()
      commandChatID.value = dashboard.value?.chats[0]?.chat_id || null
      const commandData = await $fetch<{ commands: CustomCommand[] }>(`${baseURL}/api/dashboard/commands`, { credentials: 'include' })
      commands.value = commandData.commands
    }
  } catch {
    error.value = 'Не удалось загрузить дашборд. Попробуйте еще раз позже.'
  } finally {
    loading.value = false
  }
}

async function createCommand() {
  if (!commandChatID.value || !commandName.value.trim() || !commandResponse.value.trim()) return
  savingCommand.value = true
  commandError.value = null
  try {
    const command = await $fetch<CustomCommand>(`${baseURL}/api/dashboard/commands`, {
      method: 'POST', credentials: 'include',
      body: { chat_id: commandChatID.value, name: commandName.value, response: commandResponse.value }
    })
    commands.value.unshift(command)
    commandName.value = ''
    commandResponse.value = ''
    commandModalOpen.value = false
  } catch {
    commandError.value = 'Не удалось сохранить команду. Проверьте имя и попробуйте снова.'
  } finally {
    savingCommand.value = false
  }
}

async function deleteCommand(id: number) {
  try {
    await $fetch(`${baseURL}/api/dashboard/commands/${id}`, { method: 'DELETE', credentials: 'include' })
    commands.value = commands.value.filter((command) => command.id !== id)
  } catch {
    commandError.value = 'Не удалось удалить команду.'
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

onMounted(() => {
  activityTimer = setInterval(() => { activityClock.value = Date.now() }, 1000)
  fetchAuthState()
})

onUnmounted(() => {
  if (activityTimer) clearInterval(activityTimer)
})
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
          <p class="text-xs font-semibold uppercase tracking-[0.22em] text-sky-300">Helio workspace</p>
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
              <div><span class="font-semibold tracking-tight">Helio</span><span class="block text-xs text-zinc-600">workspace</span></div>
            </div>
          </div>

          <nav class="flex gap-1 overflow-x-auto px-3 pb-3 lg:block lg:flex-1 lg:space-y-1 lg:overflow-visible lg:px-3 lg:pt-8">
            <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition lg:w-full" :class="activeView === 'overview' ? 'bg-amber-300/10 text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="navigateToView('overview')">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><rect x="3" y="3" width="7" height="7" rx="1" /><rect x="14" y="3" width="7" height="7" rx="1" /><rect x="3" y="14" width="7" height="7" rx="1" /><rect x="14" y="14" width="7" height="7" rx="1" /></svg>
              Overview
            </button>
            <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition lg:w-full" :class="activeView === 'chats' ? 'bg-amber-300/10 font-medium text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="navigateToView('chats')">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><path d="M20 11.5a7.5 7.5 0 0 1-8 7.5 8.8 8.8 0 0 1-3.3-.6L4 20l1.6-3.5A7.3 7.3 0 0 1 4.5 12 7.5 7.5 0 0 1 12 4.5c4.4 0 8 3.1 8 7Z" /></svg>
              Chats
            </button>
            <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition lg:w-full" :class="activeView === 'activity' ? 'bg-amber-300/10 font-medium text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="navigateToView('activity')">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><path d="M4 17h4V7H4v10Zm6 0h4V4h-4v13Zm6 0h4v-7h-4v7Z" /></svg>
              Activity
            </button>
            <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition lg:w-full" :class="activeView === 'commands' ? 'bg-amber-300/10 font-medium text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="navigateToView('commands')">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><path d="m8 9 3 3-3 3M13 15h4M5 4h14a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z" /></svg>
              Commands
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
             <div class="mb-9 flex items-start justify-between gap-6">
               <div>
               <p class="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-600">Command center</p>
              <h1 v-if="activeView === 'overview'" class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Good morning, {{ displayName }}.</h1>
              <h1 v-else-if="activeView === 'chats'" class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Your chats</h1>
               <h1 v-else-if="activeView === 'activity'" class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Recent activity</h1>
               <h1 v-else class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">Custom commands</h1>
              <p v-if="activeView === 'overview'" class="mt-3 max-w-xl text-zinc-500">Все важное о твоих чатах, командах и модерации в одном месте.</p>
              <p v-else-if="activeView === 'chats'" class="mt-3 max-w-xl text-zinc-500">Connected Telegram groups and their moderation status.</p>
                <p v-else-if="activeView === 'activity'" class="mt-3 max-w-xl text-zinc-500">A complete log of moderation actions in your workspace.</p>
                <p v-else class="mt-3 max-w-xl text-zinc-500">Команды, которые бот будет выполнять в твоих Telegram-группах.</p>
               </div>
               <button v-if="activeView === 'commands'" type="button" class="inline-flex shrink-0 items-center gap-2 rounded-xl bg-amber-300 px-4 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-amber-200" @click="commandError = null; commandModalOpen = true">
                 <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4"><path d="M12 5v14M5 12h14" /></svg>
                 Add command
               </button>
             </div>

            <div v-if="activeView === 'overview'" class="space-y-6">
              <div class="grid gap-4 sm:grid-cols-3">
                <div class="rounded-2xl border border-amber-300/20 bg-gradient-to-br from-amber-300/[0.12] to-transparent p-5"><p class="text-xs text-amber-200/70">Protected chats</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.protected_chats || 0 }}</p><p class="mt-2 text-xs text-zinc-500">All systems operational</p></div>
                <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Actions this week</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.actions_this_week || 0 }}</p><p class="mt-2 text-xs text-emerald-300">From the action log</p></div>
                <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Messages cleaned</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.messages_cleaned || 0 }}</p><p class="mt-2 text-xs text-zinc-600">All time</p></div>
              </div>

              <div class="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.65fr)]">
                 <section id="chats" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div><h2 class="font-semibold">Your chats</h2><p class="mt-1 text-sm text-zinc-600">Connected Telegram groups</p></div><div class="mt-6 divide-y divide-white/[0.06]"><div v-for="chat in chats" :key="chat.name" class="flex items-center justify-between gap-4 py-4 first:pt-0"><div class="flex min-w-0 items-center gap-3"><div :class="chat.color" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-xs font-bold text-zinc-950">{{ chat.initials }}</div><div class="min-w-0"><p class="truncate text-sm font-medium">{{ chat.name }}</p><p class="truncate text-xs text-zinc-600">{{ chat.handle }} · {{ chat.members }} participants</p></div></div><div class="hidden text-right sm:block"><p class="font-mono text-sm text-zinc-300">{{ chat.commands }}</p><p class="text-[10px] uppercase tracking-wider text-zinc-600">actions / 7d</p></div><span class="h-2 w-2 shrink-0 rounded-full" :class="chat.status === 'Healthy' ? 'bg-emerald-400' : 'bg-amber-300'"></span></div></div></section>
                 <section id="activity" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div class="flex items-center justify-between"><div><h2 class="font-semibold">Recent activity</h2><p class="mt-1 text-sm text-zinc-600">Moderation across your workspace</p></div><span class="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_12px_theme(colors.emerald.400)]"></span></div><div class="mt-6 space-y-5"><div v-for="item in activity" :key="item.command + item.time" class="flex gap-3"><div class="mt-1 h-7 w-7 shrink-0 rounded-lg bg-white/[0.06] text-center text-[10px] leading-7 text-zinc-500">↗</div><div class="min-w-0"><p class="text-sm"><code :class="item.tone" class="font-mono text-xs">{{ item.command }}</code> <span class="text-zinc-500">by {{ item.user }}</span></p><p class="mt-1 truncate text-xs text-zinc-600">{{ item.chat }} · {{ item.time }}</p></div></div></div><button type="button" class="mt-6 w-full border-t border-white/[0.06] pt-4 text-left text-xs text-zinc-500 hover:text-zinc-200" @click="navigateToView('activity')">View all activity →</button></section>
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

              <div v-else-if="activeView === 'activity'" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6">
               <div class="mb-4 flex items-center justify-between gap-4 border-b border-white/[0.06] pb-4">
                 <div class="flex items-center gap-2 text-xs text-zinc-600"><span class="h-2 w-2 rounded-full bg-emerald-400"></span><span>Updated {{ activityAge }}</span></div>
                 <button type="button" class="inline-flex items-center gap-2 rounded-lg px-2.5 py-2 text-xs text-zinc-500 transition hover:bg-white/[0.05] hover:text-zinc-200 disabled:cursor-wait disabled:opacity-50" :disabled="refreshingActivity" @click="refreshActivity"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" class="h-4 w-4" :class="refreshingActivity ? 'animate-spin' : ''"><path d="M20 11a8 8 0 0 0-14.9-3M4 5v4h4M4 13a8 8 0 0 0 14.9 3M20 19v-4h-4" /></svg>Refresh</button>
               </div>
               <div class="mb-4 flex gap-2 overflow-x-auto">
                 <button v-for="type in ['all', 'custom', 'moderation', 'info'] as const" :key="type" type="button" class="rounded-lg px-3 py-1.5 text-xs font-medium capitalize transition" :class="activityType === type ? 'bg-amber-300/15 text-amber-200' : 'bg-white/[0.04] text-zinc-500 hover:text-zinc-200'" @click="activityType = type">{{ type }}</button>
               </div>
               <div class="divide-y divide-white/[0.06]">
                <div v-for="item in activity" :key="item.command + item.time" class="flex items-center gap-4 py-5 first:pt-0">
                  <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white/[0.06] text-xs text-zinc-500">↗</div>
                  <div class="min-w-0 flex-1"><p class="text-sm"><code :class="item.tone" class="font-mono text-xs">{{ item.command }}</code> <span class="ml-2 rounded bg-white/[0.05] px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-zinc-500">{{ item.type }}</span> <span class="text-zinc-500">by {{ item.user }}</span></p><p class="mt-1 truncate text-xs text-zinc-600">{{ item.chat }}</p></div>
                  <time class="shrink-0 text-xs text-zinc-600">{{ item.time }}</time>
                </div>
                <div v-if="!activity.length" class="py-8 text-sm text-zinc-600">No events for this type.</div>
               </div>
             </div>

              <div v-else class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6">
                <div class="mb-2 flex items-center justify-between gap-4 text-xs text-zinc-600">
                  <span>{{ commands.length }} command{{ commands.length === 1 ? '' : 's' }}</span>
                  <span>Available to chat members</span>
                </div>
                <div class="divide-y divide-white/[0.06]">
                  <article v-for="command in commands" :key="command.id" class="flex items-center gap-4 py-4 first:pt-3">
                    <code class="w-36 shrink-0 truncate font-mono text-sm text-amber-200">{{ command.name }}</code>
                    <p class="min-w-0 flex-1 truncate text-sm text-zinc-400">{{ command.response }}</p>
                    <span class="hidden w-40 shrink-0 truncate text-xs text-zinc-600 sm:block">{{ chats.find((chat) => chat.chat_id === command.chat_id)?.name || 'Unknown chat' }}</span>
                    <button type="button" class="shrink-0 text-xs text-zinc-600 transition hover:text-rose-300" @click="deleteCommand(command.id)">Delete</button>
                  </article>
                  <div v-if="!commands.length" class="py-12 text-center text-sm text-zinc-600">No custom commands yet.</div>
                </div>
              </div>
           </div>
         </div>
      </div>
    </template>

    <div v-if="commandModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-5 py-8 backdrop-blur-sm" @click.self="commandModalOpen = false">
      <form class="w-full max-w-lg rounded-2xl border border-white/[0.1] bg-[#15181d] p-6 shadow-2xl" @submit.prevent="createCommand">
        <div class="flex items-start justify-between gap-4">
          <div><h2 class="text-lg font-semibold">Add command</h2><p class="mt-1 text-sm text-zinc-500">Ответ будет доступен всем участникам чата.</p></div>
          <button type="button" class="text-zinc-600 hover:text-zinc-200" aria-label="Close" @click="commandModalOpen = false"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-5 w-5"><path d="m6 6 12 12M18 6 6 18" /></svg></button>
        </div>
        <label class="mt-6 block text-xs text-zinc-500">Chat<select v-model="commandChatID" class="mt-2 w-full rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 text-sm text-zinc-200 outline-none focus:border-amber-300/50"><option v-for="chat in chats" :key="chat.chat_id" :value="chat.chat_id">{{ chat.name }}</option></select></label>
        <label class="mt-4 block text-xs text-zinc-500">Command<input v-model="commandName" maxlength="32" placeholder="welcome" class="mt-2 w-full rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 font-mono text-sm text-zinc-200 outline-none placeholder:text-zinc-700 focus:border-amber-300/50"></label>
        <label class="mt-4 block text-xs text-zinc-500">Response<textarea v-model="commandResponse" maxlength="4096" rows="5" placeholder="Welcome to the group!" class="mt-2 w-full resize-none rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 text-sm text-zinc-200 outline-none placeholder:text-zinc-700 focus:border-amber-300/50"></textarea></label>
        <p v-if="commandError" class="mt-3 text-xs text-rose-300">{{ commandError }}</p>
        <div class="mt-6 flex justify-end gap-3"><button type="button" class="rounded-xl px-4 py-2.5 text-sm text-zinc-500 hover:text-zinc-200" @click="commandModalOpen = false">Cancel</button><button :disabled="savingCommand || !commandChatID" type="submit" class="rounded-xl bg-amber-300 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-amber-200 disabled:cursor-not-allowed disabled:opacity-50">{{ savingCommand ? 'Saving...' : 'Save command' }}</button></div>
      </form>
    </div>
  </main>
</template>
