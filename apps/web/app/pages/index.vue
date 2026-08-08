<script setup lang="ts">
useHead({
  title: 'Helio — Telegram Group Moderation',
  meta: [
    { name: 'description', content: 'Helio is a lightweight Telegram group moderation bot. Delete, mute and ban with simple reply commands.' }
  ]
})

const adminCommands = [
  {
    command: '!delete',
    description: 'Deletes the replied-to message.'
  },
  {
    command: '!mute [duration]',
    description: 'Mutes the author of the replied-to message and deletes it. Accepts bare minutes (30), Go syntax (30m, 1h30m), or days (1d, 2d12h). Default: 30 minutes.'
  },
  {
    command: '!ban',
    description: 'Permanently bans the author of the replied-to message and deletes it.'
  }
]

const ownerCommands = [
  {
    command: '!grant',
    description: 'Grants command rights in the chat to the author of the replied-to message.'
  },
  {
    command: '!revoke',
    description: 'Revokes command rights in the chat from the author of the replied-to message.'
  }
]
</script>

<template>
  <main>
    <header class="border-b border-zinc-800/80">
      <nav class="mx-auto flex max-w-5xl items-center justify-between px-6 py-5">
        <div class="flex items-center gap-2.5">
          <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-amber-400/10 ring-1 ring-amber-400/30">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5 text-amber-400">
              <circle cx="12" cy="12" r="4" />
              <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
            </svg>
          </div>
          <span class="text-lg font-semibold tracking-tight">Helio</span>
        </div>
        <NuxtLink
          to="/dashboard"
          class="rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-2 text-sm font-medium text-zinc-300 transition-colors hover:border-zinc-700 hover:text-zinc-100"
        >
          Dashboard
        </NuxtLink>
      </nav>
    </header>

    <section class="mx-auto max-w-5xl px-6 pb-20 pt-24 text-center">
      <div class="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border border-zinc-800 bg-zinc-900/60 px-4 py-1.5 text-sm text-zinc-400">
        <span class="h-2 w-2 rounded-full bg-emerald-400" />
        Open source · Written in Go
      </div>
      <h1 class="mx-auto max-w-3xl text-5xl font-bold leading-tight tracking-tight sm:text-6xl">
        Telegram group moderation,
        <span class="bg-gradient-to-r from-amber-300 to-orange-400 bg-clip-text text-transparent">made simple</span>
      </h1>
      <p class="mx-auto mt-6 max-w-2xl text-lg leading-relaxed text-zinc-400">
         Helio keeps your group clean with simple reply commands. Delete spam, mute troublemakers, and ban bad actors — no dashboards, no bloat, just chat commands.
      </p>
      <div class="mt-10 flex items-center justify-center gap-4">
        <a
          href="#commands"
          class="rounded-lg bg-amber-400 px-6 py-3 text-sm font-semibold text-zinc-950 transition-colors hover:bg-amber-300"
        >
          View commands
        </a>
      </div>
    </section>

    <section id="commands" class="border-t border-zinc-800/80 bg-zinc-900/30">
      <div class="mx-auto max-w-5xl px-6 py-20">
        <h2 class="text-3xl font-bold tracking-tight">Commands</h2>
        <p class="mt-3 text-zinc-400">
          Commands are used by replying to a message in a group. Admin commands work for the group owner and users granted rights with <code class="rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-sm text-amber-300">!grant</code>.
        </p>

        <div class="mt-10 grid gap-8 md:grid-cols-2">
          <div>
            <h3 class="mb-4 text-sm font-semibold uppercase tracking-wider text-zinc-500">Admin commands</h3>
            <ul class="space-y-3">
              <li
                v-for="cmd in adminCommands"
                :key="cmd.command"
                class="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5"
              >
                <code class="font-mono text-sm font-semibold text-amber-300">{{ cmd.command }}</code>
                <p class="mt-2 text-sm leading-relaxed text-zinc-400">{{ cmd.description }}</p>
              </li>
            </ul>
          </div>

          <div>
            <h3 class="mb-4 text-sm font-semibold uppercase tracking-wider text-zinc-500">Owner commands</h3>
            <ul class="space-y-3">
              <li
                v-for="cmd in ownerCommands"
                :key="cmd.command"
                class="rounded-xl border border-zinc-800 bg-zinc-900/60 p-5"
              >
                <code class="font-mono text-sm font-semibold text-emerald-300">{{ cmd.command }}</code>
                <p class="mt-2 text-sm leading-relaxed text-zinc-400">{{ cmd.description }}</p>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </section>

    <footer class="border-t border-zinc-800/80">
      <div class="mx-auto flex max-w-5xl items-center justify-between px-6 py-8 text-sm text-zinc-500">
         <span>Helio — Telegram group moderation</span>
        <a
          href="https://github.com/infybtw/Helio"
          target="_blank"
          rel="noopener"
          class="transition-colors hover:text-zinc-300"
        >
          Source on GitHub
        </a>
      </div>
    </footer>
  </main>
</template>
