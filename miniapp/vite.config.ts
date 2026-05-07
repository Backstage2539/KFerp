import { defineConfig } from 'vite'
import uniPluginModule from '@dcloudio/vite-plugin-uni'

const uni = (
  typeof uniPluginModule === 'function'
    ? uniPluginModule
    : (uniPluginModule as { default: typeof uniPluginModule }).default
)

export default defineConfig({
  plugins: [uni()],
})
