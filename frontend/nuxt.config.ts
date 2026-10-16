// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  devtools: { enabled: true },
  ssr: false,
  app: {
    head: {
      htmlAttrs: { lang: "th" },
      title: "Photo Backup",
      meta: [{ name: "viewport", content: "width=device-width, initial-scale=1" }],
    },
  },
  css: [
    // ฟอนต์ฝังมากับแอป ใช้ได้โดยไม่ต้องต่ออินเทอร์เน็ต (ไทย + ละติน)
    "@fontsource/sarabun/thai-400.css",
    "@fontsource/sarabun/latin-400.css",
    "@fontsource/sarabun/thai-600.css",
    "@fontsource/sarabun/latin-600.css",
    "@fontsource/prompt/thai-600.css",
    "@fontsource/prompt/latin-600.css",
    "~/assets/css/main.css",
  ],
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
