// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: ['@nuxtjs/tailwindcss'],
  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL || 'http://localhost:8080'
    }
  },
  vite: {
    server: {
      // Allow access via FRP/ngrok hosts in development.
      // In production the app is served by Nitro, not Vite.
      allowedHosts: process.env.NUXT_ALLOWED_HOSTS
        ? process.env.NUXT_ALLOWED_HOSTS.split(',')
        : true
    }
  }
})