import { createSSRApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { notificationPageMixin } from './utils/notificationPrompt'

export function createApp() {
  const app = createSSRApp(App)
  app.use(createPinia())
  app.mixin(notificationPageMixin)
  return { app }
}
