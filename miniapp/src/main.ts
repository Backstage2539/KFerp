import { createSSRApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import {
  defaultMiniappShare,
  defaultMiniappTimelineShare,
  refreshMiniappShareMenu,
} from './utils/miniappShare'

export function createApp() {
  const app = createSSRApp(App)
  app.mixin({
    onShow: refreshMiniappShareMenu,
    onShareAppMessage: defaultMiniappShare,
    onShareTimeline: defaultMiniappTimelineShare,
  })
  app.use(createPinia())
  return { app }
}
