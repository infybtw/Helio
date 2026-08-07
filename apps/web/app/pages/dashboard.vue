<script setup lang="ts">
useHead({
  title: 'HelioBot — Dashboard Sign In',
  meta: [
    { name: 'description', content: 'Sign in to the HelioBot dashboard with your Telegram account.' }
  ]
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
      <p class="mx-auto mt-6 max-w-2xl text-lg leading-relaxed text-zinc-400">
        Sign in with your Telegram account to manage your groups and moderation settings.
      </p>
      <div class="mt-10 flex items-center justify-center">
        <button
          type="button"
          class="flex items-center gap-3 rounded-lg bg-sky-400 px-6 py-3 text-sm font-semibold text-zinc-950 transition-colors hover:bg-sky-300"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="h-5 w-5">
            <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
          </svg>
          Sign in with Telegram
        </button>
      </div>
      <div class="mt-10">
        <NuxtLink to="/" class="text-sm text-zinc-500 transition-colors hover:text-zinc-300">
          &larr; Back to home
        </NuxtLink>
      </div>
    </div>
  </main>
</template>
