import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  base: '/vue-shell/',
  plugins: [vue()],
  build: { rollupOptions: { input: { main: fileURLToPath(new URL('./index.html', import.meta.url)), pages: fileURLToPath(new URL('./pages.html', import.meta.url)) } } },
})
