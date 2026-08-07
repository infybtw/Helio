<script setup lang="ts">
useHead({
  htmlAttrs: { class: 'scroll-smooth' },
  link: [{ rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }]
})
</script>

<template>
  <div class="min-h-screen bg-zinc-950 text-zinc-100 antialiased">
    <NuxtRouteAnnouncer />
    <NuxtPage />
  </div>
</template>
