// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  devtools: { enabled: true },
  ssr: false,
  $production: {
    nitro: {
      output: {
        publicDir: "../frontend/dist",
      },
    },
  },
  router: {
    options: {
      hashMode: true,
    },
  },
});