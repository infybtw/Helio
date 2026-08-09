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
  aliases: string[]
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
  actions: CustomCommandAction[]
  enabled: boolean
  permission: 'user' | 'moderator' | 'owner'
  created_at: string
}
interface CustomCommandAction {
  type: 'send_message' | 'reply_message' | 'mute' | 'delete_message'
  payload: string
}
interface BuiltInCommand {
  name: string
  description: string
  permission: 'user' | 'moderator' | 'owner'
  enabled: boolean
  mute_duration: string
  reply_message: string
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
interface ActivityPage {
  items: DashboardActivity[]
  total: number
  page: number
  per_page: number
}

const config = ref<AuthConfig | null>(null)
const session = ref<UserSession | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const dashboard = ref<DashboardData | null>(null)
const availableChats = ref<DashboardChat[]>([])
const apiBase = useRuntimeConfig().public.apiBaseUrl as string | undefined
const baseURL = apiBase || ''
type DashboardView = 'overview' | 'chats' | 'activity' | 'commands'
type CommandCategory = 'custom' | 'built-in'
const route = useRoute()
const router = useRouter()
const initialView: DashboardView = route.query.chat_id ? (route.query.view === 'activity' ? 'activity' : 'commands') : 'overview'
const activeView = ref<DashboardView>(initialView)
const commandsExpanded = ref(initialView === 'commands')
const commandCategory = ref<CommandCategory>('custom')
const selectedChatID = ref<number | null>(route.query.chat_id ? Number(route.query.chat_id) : null)
const commands = ref<CustomCommand[]>([])
const builtInCommands = ref<BuiltInCommand[]>([])
const builtInCommandsLoading = ref(false)
const builtInCommandsError = ref<string | null>(null)
const updatingBuiltInCommand = ref<string | null>(null)
const resettingBuiltInCommands = ref(false)
const builtInCommandModalOpen = ref(false)
const editingBuiltInCommand = ref<BuiltInCommand | null>(null)
const builtInCommandEnabled = ref(true)
const builtInCommandPermission = ref<BuiltInCommand['permission']>('moderator')
const builtInCommandMuteDuration = ref('')
const builtInCommandReplyMessage = ref('')
const savingBuiltInCommand = ref(false)
const groupDataLoading = ref(false)
const commandName = ref('')
const commandAliases = ref('')
const commandPermission = ref<CustomCommand['permission']>('user')
const commandActions = ref<CustomCommandAction[]>([{ type: 'send_message', payload: '' }])
const commandChatID = ref<number | null>(null)
const commandError = ref<string | null>(null)
const commandListError = ref<string | null>(null)
const savingCommand = ref(false)
const commandModalOpen = ref(false)
const variablesModalOpen = ref(false)
const editingCommandID = ref<number | null>(null)
const draggedActionIndex = ref<number | null>(null)
const toastMessage = ref<string | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | undefined
const refreshingActivity = ref(false)
const activityUpdatedAt = ref<number | null>(null)
const activityClock = ref(Date.now())
const activityType = ref<'all' | DashboardActivity['event_type']>('all')
const activityPage = ref(1)
const activityItems = ref<DashboardActivity[]>([])
const activityTotal = ref(0)
let activityTimer: ReturnType<typeof setInterval> | undefined
let selectedChatRequest = 0

function navigateToView(view: DashboardView) {
  if (view === 'commands') {
    commandsExpanded.value = !commandsExpanded.value
    return
  }
  activeView.value = view
  const query: Record<string, string> = {}
  if (selectedChatID.value) query.chat_id = String(selectedChatID.value)
  if (view !== 'overview') query.view = view
  router.replace({ query })
  if (view === 'activity' && session.value) {
    refreshActivity()
  }
}

function selectCommandCategory(category: CommandCategory) {
  commandCategory.value = category
  commandsExpanded.value = true
  activeView.value = 'commands'
  const query: Record<string, string> = {}
  if (selectedChatID.value) query.chat_id = String(selectedChatID.value)
  query.view = 'commands'
  router.replace({ query })
  if (category === 'built-in') loadBuiltInCommands()
}

async function loadBuiltInCommands() {
  if (!selectedChatID.value || builtInCommandsLoading.value) return
  builtInCommandsLoading.value = true
  builtInCommandsError.value = null
  try {
    const data = await $fetch<{ commands: BuiltInCommand[] }>(`${baseURL}/api/dashboard/built-in-commands?chat_id=${selectedChatID.value}`, { credentials: 'include' })
    builtInCommands.value = data.commands
  } catch {
    builtInCommandsError.value = 'Не удалось загрузить встроенные команды.'
  } finally {
    builtInCommandsLoading.value = false
  }
}

async function toggleBuiltInCommand(command: BuiltInCommand) {
  if (!selectedChatID.value || updatingBuiltInCommand.value) return
  const enabled = !command.enabled
  updatingBuiltInCommand.value = command.name
  builtInCommandsError.value = null
  try {
    await $fetch(`${baseURL}/api/dashboard/built-in-commands/${encodeURIComponent(command.name)}/enabled?chat_id=${selectedChatID.value}`, {
      method: 'PATCH', credentials: 'include', body: { enabled }
    })
    command.enabled = enabled
  } catch {
    builtInCommandsError.value = 'Не удалось изменить статус встроенной команды.'
  } finally {
    updatingBuiltInCommand.value = null
  }
}

async function resetBuiltInCommands() {
  if (!selectedChatID.value || resettingBuiltInCommands.value) return
  if (!window.confirm('Reset all built-in command settings for this chat to defaults?')) return
  resettingBuiltInCommands.value = true
  builtInCommandsError.value = null
  try {
    await $fetch(`${baseURL}/api/dashboard/built-in-commands/reset?chat_id=${selectedChatID.value}`, {
      method: 'POST', credentials: 'include'
    })
    await loadBuiltInCommands()
    builtInCommandModalOpen.value = false
  } catch {
    builtInCommandsError.value = 'Не удалось сбросить настройки встроенных команд.'
  } finally {
    resettingBuiltInCommands.value = false
  }
}

function openEditBuiltInCommand(command: BuiltInCommand) {
  editingBuiltInCommand.value = command
  builtInCommandEnabled.value = command.enabled
  builtInCommandPermission.value = command.permission
  builtInCommandMuteDuration.value = command.mute_duration
  builtInCommandReplyMessage.value = command.reply_message
  builtInCommandsError.value = null
  builtInCommandModalOpen.value = true
}

async function saveBuiltInCommand() {
  const command = editingBuiltInCommand.value
  if (!command || !selectedChatID.value) return
  savingBuiltInCommand.value = true
  builtInCommandsError.value = null
  try {
    const updated = await $fetch<BuiltInCommand>(`${baseURL}/api/dashboard/built-in-commands/${encodeURIComponent(command.name)}?chat_id=${selectedChatID.value}`, {
      method: 'PUT', credentials: 'include', body: {
        enabled: builtInCommandEnabled.value,
        permission: builtInCommandPermission.value,
        mute_duration: command.name === '!mute' ? builtInCommandMuteDuration.value : '',
        reply_message: builtInCommandReplyMessage.value
      }
    })
    builtInCommands.value = builtInCommands.value.map((item) => item.name === updated.name ? { ...item, ...updated } : item)
    builtInCommandModalOpen.value = false
  } catch {
    builtInCommandsError.value = 'Не удалось сохранить настройки встроенной команды.'
  } finally {
    savingBuiltInCommand.value = false
  }
}

async function selectChat(chatID: number | null) {
  selectedChatID.value = chatID
  activeView.value = chatID ? 'commands' : 'overview'
  commandsExpanded.value = Boolean(chatID)
  commands.value = []
  builtInCommands.value = []
  builtInCommandsError.value = null
  commandListError.value = null
  groupDataLoading.value = true
  const query: Record<string, string> = {}
  if (chatID) query.chat_id = String(chatID)
  await router.replace({ query })
  if (session.value) {
    await refreshSelectedChat(chatID)
    if (chatID && commandCategory.value === 'built-in') loadBuiltInCommands()
  }
}

watch(() => route.query.view, (view) => {
  const nextView = route.query.chat_id ? (view === 'activity' ? 'activity' : 'commands') : 'overview'
  activeView.value = nextView
  if (nextView === 'commands') commandsExpanded.value = true
  if (nextView === 'activity' && session.value) {
    refreshActivity()
  }
})

watch(() => route.query.chat_id, (chatID) => {
  const nextChatID = chatID ? Number(chatID) : null
  if (selectedChatID.value === nextChatID) return
  selectedChatID.value = nextChatID
  commands.value = []
  builtInCommands.value = []
  builtInCommandsError.value = null
  commandListError.value = null
  if (session.value) {
    refreshSelectedChat().catch(() => {
      commandListError.value = 'Не удалось загрузить команды этой группы.'
    })
    if (nextChatID && commandCategory.value === 'built-in') loadBuiltInCommands()
  }
})

const displayName = computed(() => {
  if (!session.value) return ''
  return session.value.fnm || (session.value.usr ? `@${session.value.usr}` : 'Telegram user')
})

const loginUrl = computed(() => config.value?.authorize_url ? `${baseURL}${config.value.authorize_url}` : '#')

const chats = computed(() => (availableChats.value.length ? availableChats.value : dashboard.value?.chats || []).map((chat, index) => ({
  ...chat,
  members: chat.members.toLocaleString('ru-RU'),
  commands: chat.actions.toLocaleString('ru-RU'),
  color: ['bg-orange-400', 'bg-sky-400', 'bg-emerald-400'][index % 3]
})) || [])
const selectedChat = computed(() => chats.value.find((chat) => chat.chat_id === selectedChatID.value))

function formatActivityDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 'Date unavailable' : date.toLocaleString('ru-RU')
}

function formatActivity(items: DashboardActivity[]) {
  return items.map((item) => ({
    command: item.action,
    type: item.event_type,
    user: item.actor || 'Unknown user',
    chat: item.chat,
    time: formatActivityDate(item.created_at),
    tone: item.action === '!mute' ? 'text-amber-300' : item.action === '!delete' ? 'text-rose-300' : item.action === '!grant' ? 'text-emerald-300' : 'text-violet-300'
  }))
}

const overviewActivity = computed(() => formatActivity(dashboard.value?.activity || []))
const activity = computed(() => formatActivity(activityItems.value))
const activityPageCount = computed(() => Math.max(1, Math.ceil(activityTotal.value / 50)))

async function refreshActivity() {
  if (refreshingActivity.value) return
  refreshingActivity.value = true
  try {
    const type = activityType.value === 'all' ? '' : `&type=${activityType.value}`
    const separator = selectedChatID.value ? `&chat_id=${selectedChatID.value}` : ''
    const data = await $fetch<ActivityPage>(`${baseURL}/api/dashboard/activity?page=${activityPage.value}${type}${separator}`, { credentials: 'include' })
    activityItems.value = data.items
    activityTotal.value = data.total
    activityUpdatedAt.value = Date.now()
  } catch {
    error.value = 'Не удалось обновить активность. Попробуйте еще раз позже.'
  } finally {
    refreshingActivity.value = false
  }
}

function setActivityType(type: 'all' | DashboardActivity['event_type']) {
  activityType.value = type
  activityPage.value = 1
  refreshActivity()
}

function setActivityPage(page: number) {
  if (page < 1 || page > activityPageCount.value || page === activityPage.value) return
  activityPage.value = page
  refreshActivity()
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

async function refreshSelectedChat(chatID = selectedChatID.value) {
  const requestID = ++selectedChatRequest
  const query = chatID ? `?chat_id=${chatID}` : ''
  const overviewURL = `${baseURL}/api/dashboard/overview${query}`
  const commandsURL = `${baseURL}/api/dashboard/commands${query}`
  groupDataLoading.value = true
  try {
    const nextDashboard = await $fetch<DashboardData>(overviewURL, { credentials: 'include' })
    activityUpdatedAt.value = Date.now()
    const commandData = chatID
      ? await $fetch<{ commands: CustomCommand[] }>(commandsURL, { credentials: 'include' })
      : { commands: [] as CustomCommand[] }
    if (requestID !== selectedChatRequest || chatID !== selectedChatID.value) {
      return
    }
    dashboard.value = nextDashboard
    commands.value = commandData.commands
    commandListError.value = null
    commandChatID.value = chatID || dashboard.value?.chats[0]?.chat_id || null
  } finally {
    if (requestID === selectedChatRequest) groupDataLoading.value = false
  }
}

async function fetchAuthState() {
  try {
    const [configRes, meRes] = await Promise.all([
      $fetch<AuthConfig>(`${baseURL}/api/auth/config`, { credentials: 'include' }),
      $fetch<UserSession>(`${baseURL}/api/auth/me`, { credentials: 'include' }).catch(() => null)
    ])
    config.value = configRes
    session.value = meRes
    if (meRes) {
      const allDashboard = await $fetch<DashboardData>(`${baseURL}/api/dashboard/overview`, { credentials: 'include' })
      availableChats.value = allDashboard.chats
      if (selectedChatID.value && !availableChats.value.some((chat) => chat.chat_id === selectedChatID.value)) selectedChatID.value = null
      dashboard.value = allDashboard
      if (selectedChatID.value) {
        await refreshSelectedChat(selectedChatID.value)
      } else {
         commands.value = []
         builtInCommands.value = []
         builtInCommandsError.value = null
        commandChatID.value = null
      }
      if (activeView.value === 'activity') {
        await refreshActivity()
      }
    }
  } catch {
    error.value = 'Не удалось загрузить дашборд. Попробуйте еще раз позже.'
  } finally {
    loading.value = false
  }
}

async function createCommand() {
  if (!commandChatID.value || !commandName.value.trim()) {
    showToast('Укажи название команды.')
    return
  }
  if (commandActions.value.some((action) => (action.type === 'send_message' || action.type === 'reply_message') && !action.payload.trim())) {
    showToast('Введи текст сообщения для действия.')
    return
  }
  savingCommand.value = true
  commandError.value = null
  try {
    const isEditing = editingCommandID.value !== null
    const command = await $fetch<CustomCommand>(`${baseURL}/api/dashboard/commands${isEditing ? `/${editingCommandID.value}` : ''}`, {
      method: isEditing ? 'PUT' : 'POST', credentials: 'include',
       body: { chat_id: selectedChatID.value || commandChatID.value, name: commandName.value, permission: commandPermission.value, aliases: commandAliases.value.split(',').map((alias) => alias.trim()).filter(Boolean), actions: commandActions.value }
    })
    if (isEditing) {
      commands.value = commands.value.map((item) => item.id === command.id ? command : item)
    } else {
      commands.value.unshift(command)
    }
    commandName.value = ''
    commandActions.value = [{ type: 'send_message', payload: '' }]
    editingCommandID.value = null
    commandModalOpen.value = false
  } catch {
    commandError.value = 'Не удалось сохранить команду. Проверьте имя и попробуйте снова.'
  } finally {
    savingCommand.value = false
  }
}

function showToast(message: string) {
  toastMessage.value = message
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toastMessage.value = null }, 3500)
}

async function deleteCommand(id: number) {
  commandListError.value = null
  try {
    await $fetch(`${baseURL}/api/dashboard/commands/${id}`, { method: 'DELETE', credentials: 'include' })
    commands.value = commands.value.filter((command) => command.id !== id)
  } catch {
    commandListError.value = 'Не удалось удалить команду. Попробуйте еще раз.'
  }
}

async function toggleCommand(command: CustomCommand) {
  commandListError.value = null
  const enabled = !command.enabled
  try {
    await $fetch(`${baseURL}/api/dashboard/commands/${command.id}/enabled`, { method: 'PATCH', credentials: 'include', body: { enabled } })
    command.enabled = enabled
  } catch {
    commandListError.value = 'Не удалось изменить статус команды.'
  }
}

function openCreateCommand() {
  editingCommandID.value = null
  commandName.value = ''
  commandAliases.value = ''
  commandPermission.value = 'user'
  commandActions.value = [{ type: 'send_message', payload: '' }]
  commandError.value = null
  commandModalOpen.value = true
}

function openEditCommand(command: CustomCommand) {
  editingCommandID.value = command.id
  commandChatID.value = command.chat_id
  commandName.value = command.name.replace(/^!/, '')
  commandAliases.value = command.aliases.join(', ')
  commandPermission.value = command.permission
  commandActions.value = command.actions.map((action) => ({ ...action }))
  commandError.value = null
  commandModalOpen.value = true
}

function addCommandAction() {
  commandActions.value.push({ type: 'send_message', payload: '' })
}

function removeCommandAction(index: number) {
  if (commandActions.value.length > 1) commandActions.value.splice(index, 1)
}

function moveCommandAction(from: number, to: number) {
  if (from === to || from < 0 || to < 0 || from >= commandActions.value.length || to >= commandActions.value.length) return
  const [action] = commandActions.value.splice(from, 1)
  commandActions.value.splice(to, 0, action)
}

function dropCommandAction(index: number) {
  if (draggedActionIndex.value !== null) moveCommandAction(draggedActionIndex.value, index)
  draggedActionIndex.value = null
}

function dragOverCommandAction(index: number) {
  if (draggedActionIndex.value === null || draggedActionIndex.value === index) return
  moveCommandAction(draggedActionIndex.value, index)
  draggedActionIndex.value = index
}

function normalizeAction(action: CustomCommandAction) {
  if (action.type === 'mute' && !action.payload) action.payload = '30m'
  if (action.type !== 'send_message' && action.type !== 'reply_message') {
    if (action.type === 'delete_message') action.payload = ''
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
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <main class="min-h-screen bg-[#0b0d10] text-zinc-100">
    <div v-if="loading" class="flex min-h-screen items-center justify-center bg-[#0b0d10] px-5 py-8 text-zinc-100 sm:px-8 lg:px-10 lg:py-10 xl:px-14">
      <div class="w-full max-w-3xl">
        <div class="flex items-center gap-3 text-sm text-zinc-400">
          <span class="h-4 w-4 animate-spin rounded-full border-2 border-amber-300/30 border-t-amber-300"></span>
          Loading dashboard...
        </div>
        <div class="mt-10 animate-pulse space-y-6">
          <div class="h-12 w-2/3 rounded-xl bg-white/[0.06]"></div>
          <div class="grid gap-4 sm:grid-cols-3">
            <div v-for="card in 3" :key="card" class="h-32 rounded-2xl bg-white/[0.04]"></div>
          </div>
          <div class="h-72 rounded-2xl bg-white/[0.04]"></div>
        </div>
      </div>
    </div>

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
             <NuxtLink to="/" class="flex items-center gap-3 rounded-xl outline-none transition hover:opacity-80 focus-visible:ring-2 focus-visible:ring-amber-300/50">
              <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-amber-300 text-zinc-950">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" /></svg>
              </div>
              <div><span class="font-semibold tracking-tight">Helio</span><span class="block text-xs text-zinc-600">workspace</span></div>
             </NuxtLink>
           </div>

           <div v-if="selectedChatID" class="mx-5 mb-2 border-t border-white/[0.07] pt-5 lg:mx-6">
             <p class="truncate text-sm font-semibold text-zinc-200">{{ selectedChat?.name }}</p>
             <button type="button" class="mt-3 inline-flex items-center gap-2 text-xs text-zinc-500 transition hover:text-amber-200" @click="selectChat(null)">
               <span aria-hidden="true">←</span>
               Back to groups
             </button>
           </div>

            <nav class="flex gap-1 overflow-x-auto px-3 pb-3 lg:flex-1 lg:flex-col lg:space-y-1 lg:overflow-y-auto lg:px-3 lg:pt-8">
             <button v-if="!selectedChatID" type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition lg:w-full" :class="activeView === 'overview' ? 'bg-amber-300/10 text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="navigateToView('overview')">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" class="h-4 w-4"><rect x="3" y="3" width="7" height="7" rx="1" /><rect x="14" y="3" width="7" height="7" rx="1" /><rect x="3" y="14" width="7" height="7" rx="1" /><rect x="14" y="14" width="7" height="7" rx="1" /></svg>
              Overview
            </button>
              <button v-if="!selectedChatID" v-for="chat in chats" :key="chat.chat_id" type="button" class="flex min-w-0 shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm transition lg:w-full" @click="selectChat(chat.chat_id)">
               <span :class="chat.color" class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-[10px] font-bold text-zinc-950">{{ chat.initials }}</span>
               <span class="min-w-0 truncate">{{ chat.name }}</span>
              </button>
              <template v-if="selectedChatID">
                <button type="button" class="flex shrink-0 items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition lg:w-full" :class="activeView === 'activity' ? 'bg-amber-300/10 font-medium text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="navigateToView('activity')">
                  <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-white/[0.05] text-[10px] font-bold">A</span>
                  Activity
                </button>
                <div class="flex shrink-0 flex-col gap-1 lg:w-full">
                  <button type="button" class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition lg:w-full" :class="activeView === 'commands' ? 'bg-amber-300/10 font-medium text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" :aria-expanded="commandsExpanded" aria-controls="command-category-nav" @click="navigateToView('commands')">
                    <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-white/[0.05] text-[10px] font-bold">C</span>
                    Commands
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="ml-auto h-4 w-4 transition-transform" :class="commandsExpanded ? 'rotate-180' : ''"><path d="m6 9 6 6 6-6" /></svg>
                  </button>
                  <div v-if="commandsExpanded" id="command-category-nav" class="ml-5 flex gap-1 border-l border-white/[0.08] pl-3 lg:flex-col">
                    <button type="button" class="rounded-lg px-3 py-2 text-left text-xs transition lg:w-full" :class="commandCategory === 'built-in' && activeView === 'commands' ? 'bg-white/[0.06] text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="selectCommandCategory('built-in')">Built-In</button>
                    <button type="button" class="rounded-lg px-3 py-2 text-left text-xs transition lg:w-full" :class="commandCategory === 'custom' && activeView === 'commands' ? 'bg-white/[0.06] text-amber-200' : 'text-zinc-500 hover:bg-white/[0.04] hover:text-zinc-200'" @click="selectCommandCategory('custom')">Custom</button>
                  </div>
                </div>
              </template>
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
                <h1 v-else class="mt-3 text-4xl font-bold tracking-[-0.04em] sm:text-5xl">{{ commandCategory === 'custom' ? 'Custom commands' : 'Built-In commands' }}</h1>
              <p v-if="activeView === 'overview'" class="mt-3 max-w-xl text-zinc-500">Все важное о твоих чатах, командах и модерации в одном месте.</p>
              <p v-else-if="activeView === 'chats'" class="mt-3 max-w-xl text-zinc-500">Connected Telegram groups and their moderation status.</p>
                <p v-else-if="activeView === 'activity'" class="mt-3 max-w-xl text-zinc-500">A complete log of moderation actions in your workspace.</p>
               <p v-else class="mt-3 max-w-xl text-zinc-500">Команды и настройки группы {{ selectedChat?.name || '' }}.</p>
                </div>
                <button v-if="activeView === 'commands' && commandCategory === 'custom'" type="button" class="inline-flex shrink-0 items-center gap-2 rounded-xl bg-amber-300 px-4 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-amber-200" @click="openCreateCommand">
                 <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4"><path d="M12 5v14M5 12h14" /></svg>
                 Add command
               </button>
              </div>

             <div v-if="groupDataLoading || (activeView === 'activity' && refreshingActivity)" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6">
               <div class="flex items-center gap-3 text-sm text-zinc-400">
                 <span class="h-4 w-4 animate-spin rounded-full border-2 border-amber-300/30 border-t-amber-300"></span>
                 Loading group data...
               </div>
               <div class="mt-6 space-y-3">
                 <div class="h-4 w-1/3 animate-pulse rounded bg-white/[0.06]"></div>
                 <div class="h-24 animate-pulse rounded-xl bg-white/[0.04]"></div>
                 <div class="h-24 animate-pulse rounded-xl bg-white/[0.04]"></div>
               </div>
             </div>

             <div v-else-if="activeView === 'overview'" class="space-y-6">
              <div class="grid gap-4 sm:grid-cols-3">
                <div class="rounded-2xl border border-amber-300/20 bg-gradient-to-br from-amber-300/[0.12] to-transparent p-5"><p class="text-xs text-amber-200/70">Protected chats</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.protected_chats || 0 }}</p><p class="mt-2 text-xs text-zinc-500">All systems operational</p></div>
                <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Actions this week</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.actions_this_week || 0 }}</p><p class="mt-2 text-xs text-emerald-300">From the action log</p></div>
                <div class="rounded-2xl border border-white/[0.07] bg-white/[0.035] p-5"><p class="text-xs text-zinc-500">Messages cleaned</p><p class="mt-5 text-4xl font-semibold">{{ dashboard?.messages_cleaned || 0 }}</p><p class="mt-2 text-xs text-zinc-600">All time</p></div>
              </div>

              <div class="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.65fr)]">
                 <section id="chats" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div><h2 class="font-semibold">Your chats</h2><p class="mt-1 text-sm text-zinc-600">Connected Telegram groups</p></div><div class="mt-6 divide-y divide-white/[0.06]"><div v-for="chat in chats" :key="chat.name" class="flex items-center justify-between gap-4 py-4 first:pt-0"><div class="flex min-w-0 items-center gap-3"><div :class="chat.color" class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-xs font-bold text-zinc-950">{{ chat.initials }}</div><div class="min-w-0"><p class="truncate text-sm font-medium">{{ chat.name }}</p><p class="truncate text-xs text-zinc-600">{{ chat.handle }} · {{ chat.members }} participants</p></div></div><div class="hidden text-right sm:block"><p class="font-mono text-sm text-zinc-300">{{ chat.commands }}</p><p class="text-[10px] uppercase tracking-wider text-zinc-600">actions / 7d</p></div><span class="h-2 w-2 shrink-0 rounded-full" :class="chat.status === 'Healthy' ? 'bg-emerald-400' : 'bg-amber-300'"></span></div></div></section>
                 <section id="activity" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6"><div class="flex items-center justify-between"><div><h2 class="font-semibold">Recent activity</h2><p class="mt-1 text-sm text-zinc-600">Moderation across your workspace</p></div><span class="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_12px_theme(colors.emerald.400)]"></span></div><div class="mt-6 space-y-5"><div v-for="item in overviewActivity" :key="item.command + item.time" class="flex gap-3"><div class="mt-1 h-7 w-7 shrink-0 rounded-lg bg-white/[0.06] text-center text-[10px] leading-7 text-zinc-500">↗</div><div class="min-w-0"><p class="text-sm"><code :class="item.tone" class="font-mono text-xs">{{ item.command }}</code> <span class="text-zinc-500">by {{ item.user }}</span></p><p class="mt-1 truncate text-xs text-zinc-600">{{ item.chat }} · {{ item.time }}</p></div></div><p v-if="!overviewActivity.length" class="py-6 text-sm text-zinc-600">No recent activity yet.</p></div><button type="button" class="mt-6 w-full border-t border-white/[0.06] pt-4 text-left text-xs text-zinc-500 hover:text-zinc-200" @click="navigateToView('activity')">View all activity →</button></section>
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
                  <button v-for="type in ['all', 'custom', 'moderation', 'info'] as const" :key="type" type="button" class="rounded-lg px-3 py-1.5 text-xs font-medium capitalize transition" :class="activityType === type ? 'bg-amber-300/15 text-amber-200' : 'bg-white/[0.04] text-zinc-500 hover:text-zinc-200'" @click="setActivityType(type)">{{ type }}</button>
               </div>
               <div class="divide-y divide-white/[0.06]">
                <div v-for="item in activity" :key="item.command + item.time" class="flex items-center gap-4 py-5 first:pt-0">
                  <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white/[0.06] text-xs text-zinc-500">↗</div>
                  <div class="min-w-0 flex-1"><p class="text-sm"><code :class="item.tone" class="font-mono text-xs">{{ item.command }}</code> <span class="ml-2 rounded bg-white/[0.05] px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-zinc-500">{{ item.type }}</span> <span class="text-zinc-500">by {{ item.user }}</span></p><p class="mt-1 truncate text-xs text-zinc-600">{{ item.chat }}</p></div>
                  <time class="shrink-0 text-xs text-zinc-600">{{ item.time }}</time>
                </div>
                <div v-if="!activity.length" class="py-8 text-sm text-zinc-600">No events for this type.</div>
                </div>
               <div v-if="activityTotal > 0" class="mt-5 flex items-center justify-between border-t border-white/[0.06] pt-4 text-xs text-zinc-600">
                 <span>Page {{ activityPage }} of {{ activityPageCount }} · {{ activityTotal }} events</span>
                 <div class="flex gap-2"><button type="button" class="rounded-lg px-3 py-2 transition hover:bg-white/[0.05] hover:text-zinc-200 disabled:cursor-not-allowed disabled:opacity-40" :disabled="activityPage === 1 || refreshingActivity" @click="setActivityPage(activityPage - 1)">Previous</button><button type="button" class="rounded-lg px-3 py-2 transition hover:bg-white/[0.05] hover:text-zinc-200 disabled:cursor-not-allowed disabled:opacity-40" :disabled="activityPage === activityPageCount || refreshingActivity" @click="setActivityPage(activityPage + 1)">Next</button></div>
               </div>
              </div>

               <div v-else-if="commandCategory === 'custom'" class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6">
                <div class="mb-2 flex items-center justify-between gap-4 text-xs text-zinc-600">
                  <span>{{ commands.length }} command{{ commands.length === 1 ? '' : 's' }}</span>
                  <span>Available to chat members</span>
                </div>
                <p v-if="commandListError" class="mb-3 text-xs text-rose-300">{{ commandListError }}</p>
                <div class="divide-y divide-white/[0.06]">
                  <article v-for="command in commands" :key="command.id" class="flex min-w-0 items-center gap-3 py-4 first:pt-3">
                    <div class="min-w-0 flex-1"><div class="flex min-w-0 items-center gap-3"><code class="shrink-0 truncate font-mono text-sm" :class="command.enabled ? 'text-amber-200' : 'text-zinc-600 line-through'">{{ command.name }}</code><p class="min-w-0 truncate text-sm text-zinc-400">{{ command.actions.length === 1 ? `Send message: ${command.actions[0]?.payload || ''}` : `${command.actions.length} actions` }}</p></div><span class="mt-1 block truncate text-xs text-zinc-600">{{ command.aliases.length ? `aliases: ${command.aliases.map((alias) => '!' + alias).join(', ')}` : command.permission }}<span class="hidden sm:inline"> · {{ command.permission }} · {{ chats.find((chat) => chat.chat_id === command.chat_id)?.name || 'Unknown chat' }}</span></span></div>
                    <span class="hidden w-40 shrink-0 truncate text-xs text-zinc-600 sm:block">{{ chats.find((chat) => chat.chat_id === command.chat_id)?.name || 'Unknown chat' }}</span>
                    <div class="flex shrink-0 items-center gap-3"><button type="button" role="switch" :aria-checked="command.enabled" class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full p-1 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-300/40" :class="command.enabled ? 'bg-emerald-400/80' : 'bg-zinc-700'" @click="toggleCommand(command)"><span class="block h-4 w-4 rounded-full bg-white shadow-sm transition-transform" :class="command.enabled ? 'translate-x-5' : 'translate-x-0'"></span></button><button type="button" class="text-xs text-zinc-600 transition hover:text-amber-200" @click="openEditCommand(command)">Edit</button><button type="button" class="text-xs text-zinc-600 transition hover:text-rose-300" @click="deleteCommand(command.id)">Delete</button></div>
                  </article>
                  <div v-if="!commands.length" class="py-12 text-center text-sm text-zinc-600">No custom commands yet.</div>
                 </div>
               </div>
               <div v-else class="rounded-2xl border border-white/[0.07] bg-[#111418] p-6">
                 <div class="flex items-start justify-between gap-4 border-b border-white/[0.06] pb-5">
                   <div><h2 class="font-semibold text-zinc-200">Built-In command settings</h2><p class="mt-1 max-w-xl text-sm leading-6 text-zinc-500">Enable only the commands your group needs. Disabled commands are ignored by the bot.</p></div>
                   <div class="flex shrink-0 items-center gap-2"><button type="button" class="rounded-lg px-2.5 py-2 text-xs text-zinc-600 transition hover:bg-rose-500/10 hover:text-rose-300 disabled:cursor-wait disabled:opacity-50" :disabled="builtInCommandsLoading || resettingBuiltInCommands" @click="resetBuiltInCommands">Reset defaults</button><button type="button" class="inline-flex items-center gap-2 rounded-lg px-2.5 py-2 text-xs text-zinc-500 transition hover:bg-white/[0.05] hover:text-zinc-200 disabled:cursor-wait disabled:opacity-50" :disabled="builtInCommandsLoading || resettingBuiltInCommands" @click="loadBuiltInCommands">Refresh</button></div>
                 </div>
                 <p v-if="builtInCommandsError" class="mt-4 text-xs text-rose-300">{{ builtInCommandsError }}</p>
                 <div v-else-if="builtInCommandsLoading" class="py-10 text-center text-sm text-zinc-500">Loading built-in commands...</div>
                 <div v-else class="divide-y divide-white/[0.06]">
                   <article v-for="command in builtInCommands" :key="command.name" class="flex items-center gap-4 py-5">
                     <div class="min-w-0 flex-1"><div class="flex items-center gap-3"><code class="font-mono text-sm" :class="command.enabled ? 'text-amber-200' : 'text-zinc-600 line-through'">{{ command.name }}</code><span class="rounded bg-white/[0.05] px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-zinc-500">{{ command.permission }}</span></div><p class="mt-2 text-sm text-zinc-500">{{ command.description }}</p></div>
                     <div class="flex shrink-0 items-center gap-3"><button type="button" role="switch" :aria-checked="command.enabled" :aria-label="`Toggle ${command.name}`" :disabled="updatingBuiltInCommand === command.name" class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full p-1 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-300/40 disabled:cursor-wait disabled:opacity-50" :class="command.enabled ? 'bg-emerald-400/80' : 'bg-zinc-700'" @click="toggleBuiltInCommand(command)"><span class="block h-4 w-4 rounded-full bg-white shadow-sm transition-transform" :class="command.enabled ? 'translate-x-5' : 'translate-x-0'"></span></button><button type="button" class="text-xs text-zinc-600 transition hover:text-amber-200" @click="openEditBuiltInCommand(command)">Edit</button></div>
                   </article>
                   <div v-if="!builtInCommands.length" class="py-10 text-center text-sm text-zinc-600">No built-in commands found.</div>
                 </div>
               </div>
            </div>
         </div>
      </div>
    </template>

    <div v-if="builtInCommandModalOpen && editingBuiltInCommand" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-5 py-8 backdrop-blur-sm" @click.self="builtInCommandModalOpen = false">
      <form class="w-full max-w-lg rounded-2xl border border-white/[0.1] bg-[#15181d] p-6 shadow-2xl" @submit.prevent="saveBuiltInCommand">
        <div class="flex items-start justify-between gap-4"><div><h2 class="text-lg font-semibold">Edit {{ editingBuiltInCommand.name }}</h2><p class="mt-1 text-sm text-zinc-500">Changes apply only to {{ selectedChat?.name || 'this chat' }}.</p></div><button type="button" class="text-zinc-600 hover:text-zinc-200" aria-label="Close" @click="builtInCommandModalOpen = false"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-5 w-5"><path d="m6 6 12 12M18 6 6 18" /></svg></button></div>
        <label class="mt-6 flex items-center justify-between gap-4 rounded-xl border border-white/[0.08] bg-[#111418] px-4 py-3"><span><span class="block text-sm font-medium text-zinc-200">Command enabled</span><span class="mt-1 block text-xs text-zinc-600">Disabled commands are ignored by the bot.</span></span><button type="button" role="switch" :aria-checked="builtInCommandEnabled" class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full p-1 transition-colors focus:outline-none focus:ring-2 focus:ring-amber-300/40" :class="builtInCommandEnabled ? 'bg-emerald-400/80' : 'bg-zinc-700'" @click="builtInCommandEnabled = !builtInCommandEnabled"><span class="block h-4 w-4 rounded-full bg-white shadow-sm transition-transform" :class="builtInCommandEnabled ? 'translate-x-5' : 'translate-x-0'"></span></button></label>
        <label class="mt-4 block text-xs text-zinc-500">Who can use it<select v-model="builtInCommandPermission" class="mt-2 w-full rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 text-sm text-zinc-200 outline-none focus:border-amber-300/50"><option value="user">User — everyone</option><option value="moderator">Moderator — owner and granted moderators</option><option value="owner">Owner — chat owner only</option></select></label>
        <label v-if="editingBuiltInCommand.name === '!mute'" class="mt-4 block text-xs text-zinc-500">Default mute duration<input v-model="builtInCommandMuteDuration" placeholder="30m" class="mt-2 w-full rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 font-mono text-sm text-zinc-200 outline-none placeholder:text-zinc-700 focus:border-amber-300/50"><span class="mt-1 block text-[11px] text-zinc-600">Accepts minutes, Go durations such as `1h30m`, or days such as `2d`.</span></label>
         <div class="mt-4 flex items-center justify-between gap-4"><span class="text-xs text-zinc-500">{{ editingBuiltInCommand.name === '!help' ? 'Help output' : 'Reply message (optional)' }}</span><button type="button" class="inline-flex items-center gap-2 rounded-lg border border-amber-300/20 bg-amber-300/10 px-2.5 py-1.5 text-xs font-medium text-amber-200 transition hover:bg-amber-300/20" @click="variablesModalOpen = true">Variables</button></div>
         <label class="mt-2 block text-xs text-zinc-500"><textarea v-model="builtInCommandReplyMessage" maxlength="4096" rows="7" :placeholder="editingBuiltInCommand.name === '!help' ? 'Available commands...' : 'This action has been completed.'" class="w-full resize-none rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 text-sm text-zinc-200 outline-none placeholder:text-zinc-700 focus:border-amber-300/50"></textarea><span class="mt-1 block text-[11px] text-zinc-600">{{ editingBuiltInCommand.name === '!help' ? 'This text replaces the default !help output. Clear it to restore the default.' : 'Sent as a reply when this command is used.' }}</span></label>
        <div class="mt-6 flex justify-end gap-3"><button type="button" class="rounded-xl px-4 py-2.5 text-sm text-zinc-500 hover:text-zinc-200" @click="builtInCommandModalOpen = false">Cancel</button><button :disabled="savingBuiltInCommand" type="submit" class="rounded-xl bg-amber-300 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-amber-200 disabled:cursor-not-allowed disabled:opacity-50">{{ savingBuiltInCommand ? 'Saving...' : 'Save changes' }}</button></div>
      </form>
    </div>

    <div v-if="commandModalOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 px-5 py-8 backdrop-blur-sm" @click.self="commandModalOpen = false">
      <form class="max-h-[calc(100vh-4rem)] w-full max-w-lg overflow-y-auto rounded-2xl border border-white/[0.1] bg-[#15181d] p-6 shadow-2xl" @submit.prevent="createCommand">
         <div class="flex items-start justify-between gap-4">
           <div><h2 class="text-lg font-semibold">{{ editingCommandID ? 'Edit command' : 'Add command' }}</h2><p class="mt-1 text-sm text-zinc-500">Ответ будет доступен всем участникам чата.</p></div>
          <button type="button" class="text-zinc-600 hover:text-zinc-200" aria-label="Close" @click="commandModalOpen = false"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-5 w-5"><path d="m6 6 12 12M18 6 6 18" /></svg></button>
        </div>
        <label class="mt-6 block text-xs text-zinc-500">Chat<select v-model="commandChatID" class="mt-2 w-full rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 text-sm text-zinc-200 outline-none focus:border-amber-300/50"><option v-for="chat in chats" :key="chat.chat_id" :value="chat.chat_id">{{ chat.name }}</option></select></label>
         <label class="mt-4 block text-xs text-zinc-500">Command<div class="mt-2 flex overflow-hidden rounded-xl border border-white/[0.1] bg-[#111418] focus-within:border-amber-300/50"><span class="border-r border-white/[0.08] px-3 py-3 font-mono text-sm text-amber-200">!</span><input v-model="commandName" maxlength="32" placeholder="welcome" class="min-w-0 flex-1 bg-transparent px-3 py-3 font-mono text-sm text-zinc-200 outline-none placeholder:text-zinc-700"></div></label>
         <label class="mt-4 block text-xs text-zinc-500">Who can use it<select v-model="commandPermission" class="mt-2 w-full rounded-xl border border-white/[0.1] bg-[#111418] px-3 py-3 text-sm text-zinc-200 outline-none focus:border-amber-300/50"><option value="user">User — everyone</option><option value="moderator">Moderator — owner and granted moderators</option><option value="owner">Owner — chat owner only</option></select></label>
         <label class="mt-4 block text-xs text-zinc-500">Aliases<div class="mt-2 flex overflow-hidden rounded-xl border border-white/[0.1] bg-[#111418] focus-within:border-amber-300/50"><span class="border-r border-white/[0.08] px-3 py-3 font-mono text-sm text-amber-200">!</span><input v-model="commandAliases" placeholder="test, hi" class="min-w-0 flex-1 bg-transparent px-3 py-3 font-mono text-sm text-zinc-200 outline-none placeholder:text-zinc-700"></div><span class="mt-1 block text-[11px] text-zinc-600">Несколько aliases через запятую.</span></label>
         <div class="mt-6 flex items-center justify-between gap-4"><span class="text-xs font-semibold uppercase tracking-[0.16em] text-zinc-600">Actions</span><button type="button" class="inline-flex items-center gap-2 rounded-lg border border-amber-300/20 bg-amber-300/10 px-2.5 py-1.5 text-xs font-medium text-amber-200 transition hover:bg-amber-300/20" @click="variablesModalOpen = true">Variables</button></div>
        <div class="mt-3 space-y-3"><div v-for="(action, index) in commandActions" :key="index" draggable="true" class="rounded-xl border border-white/[0.08] bg-[#111418] transition" :class="draggedActionIndex === index ? 'opacity-40' : ''" @dragstart="draggedActionIndex = index" @dragend="draggedActionIndex = null" @dragover.prevent="dragOverCommandAction(index)" @drop.prevent="dropCommandAction(index)"><div class="p-3"><div class="mb-2 flex items-center justify-between gap-3"><div class="flex min-w-0 items-center gap-2"><span class="cursor-grab text-zinc-600 active:cursor-grabbing" title="Drag to reorder">⠿</span><select v-model="action.type" class="min-w-0 rounded-lg border border-white/[0.08] bg-[#15181d] px-2 py-1.5 text-xs text-zinc-300 outline-none" @change="normalizeAction(action)"><option value="send_message">Send message</option><option value="reply_message">Reply to message</option><option value="mute">Mute reply author</option><option value="delete_message">Delete reply</option></select></div><button v-if="commandActions.length > 1" type="button" class="text-xs text-zinc-600 hover:text-rose-300" @click="removeCommandAction(index)">Remove</button></div><textarea v-if="action.type === 'send_message' || action.type === 'reply_message'" v-model="action.payload" maxlength="4096" rows="4" :placeholder="action.type === 'reply_message' ? 'Reply text' : 'Message text'" class="w-full resize-none bg-transparent text-sm text-zinc-200 outline-none placeholder:text-zinc-700"></textarea><label v-else-if="action.type === 'mute'" class="block text-xs text-zinc-600">Duration<input v-model="action.payload" placeholder="30m" class="mt-2 w-full rounded-lg border border-white/[0.08] bg-transparent px-3 py-2 text-sm text-zinc-200 outline-none placeholder:text-zinc-700"></label><p v-else class="text-sm text-zinc-500">Deletes the message this command replies to.</p></div></div><button type="button" class="inline-flex items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-amber-200 transition hover:bg-amber-300/10" @click="addCommandAction"><span class="text-base leading-none">+</span>Add action</button></div>
        <p v-if="commandError" class="mt-3 text-xs text-rose-300">{{ commandError }}</p>
        <div class="mt-6 flex justify-end gap-3"><button type="button" class="rounded-xl px-4 py-2.5 text-sm text-zinc-500 hover:text-zinc-200" @click="commandModalOpen = false">Cancel</button><button :disabled="savingCommand || !commandChatID" type="submit" class="rounded-xl bg-amber-300 px-5 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-amber-200 disabled:cursor-not-allowed disabled:opacity-50">{{ savingCommand ? 'Saving...' : editingCommandID ? 'Save changes' : 'Save command' }}</button></div>
      </form>
    </div>

    <div v-if="toastMessage" class="fixed bottom-5 left-5 z-[60] flex max-w-sm items-center gap-3 rounded-xl border border-amber-300/20 bg-[#201d16] px-4 py-3 text-sm text-amber-100 shadow-2xl"><span class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-amber-300 text-xs font-bold text-zinc-950">!</span>{{ toastMessage }}</div>

    <div v-if="variablesModalOpen" class="fixed inset-0 z-[70] bg-black/50" @click.self="variablesModalOpen = false"><aside class="mr-auto h-full w-full max-w-sm overflow-y-auto border-r border-white/[0.1] bg-[#15181d] p-6 shadow-2xl"><div class="flex items-start justify-between gap-4"><div><h2 class="text-lg font-semibold">Variables</h2><p class="mt-1 text-sm text-zinc-500">Данные пользователя, вызвавшего команду, и автора reply.</p></div><button type="button" class="text-zinc-600 hover:text-zinc-200" aria-label="Close" @click="variablesModalOpen = false"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-5 w-5"><path d="m6 6 12 12M18 6 6 18" /></svg></button></div><div class="mt-8 space-y-4"><div class="rounded-xl border border-white/[0.08] bg-[#111418] p-4"><code class="font-mono text-sm text-amber-200">&#123;&#123;username&#125;&#125;</code><p class="mt-2 text-sm text-zinc-500">Username пользователя, вызвавшего команду, без `@`.</p></div><div class="rounded-xl border border-white/[0.08] bg-[#111418] p-4"><code class="font-mono text-sm text-amber-200">&#123;&#123;firstname&#125;&#125;</code><p class="mt-2 text-sm text-zinc-500">Имя пользователя, вызвавшего команду.</p></div><div class="rounded-xl border border-white/[0.08] bg-[#111418] p-4"><code class="font-mono text-sm text-amber-200">&#123;&#123;user_id&#125;&#125;</code><p class="mt-2 text-sm text-zinc-500">Telegram ID пользователя, вызвавшего команду.</p></div><div class="rounded-xl border border-white/[0.08] bg-[#111418] p-4"><code class="font-mono text-sm text-amber-200">&#123;&#123;reply_username&#125;&#125;</code><p class="mt-2 text-sm text-zinc-500">Username автора сообщения, на которое ответили.</p></div><div class="rounded-xl border border-white/[0.08] bg-[#111418] p-4"><code class="font-mono text-sm text-amber-200">&#123;&#123;reply_firstname&#125;&#125;</code><p class="mt-2 text-sm text-zinc-500">Имя автора сообщения, на которое ответили.</p></div></div></aside></div>
  </main>
</template>
