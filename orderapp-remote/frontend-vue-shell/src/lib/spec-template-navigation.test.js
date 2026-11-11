import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
const here = dirname(fileURLToPath(import.meta.url))
const root = resolve(here, '../../..')
const read = (path) => readFileSync(resolve(root, path), 'utf8')
test('规格模板有商品菜单入口并使用 BOM 权限', () => {
  const menu = read('frontend-vue-shell/src/lib/menu-ia.js')
  assert.match(menu, /key: 'productSpecTemplates', label: '规格模板'/)
  const app = read('frontend-vue-shell/src/App.vue')
  assert.match(app, /productSpecTemplates: SpecTemplatesView/)
  const auth = read('internal/infrastructure/postgres/authz/schema.go')
  assert.match(auth, /"productSpecTemplates"\s*:\s*"bom.read"/)
})
test('BOM 跳转规格模板并携带返回上下文，支持指定模板版本', () => {
  const bom = read('frontend-vue-shell/src/views/BomView.vue')
  assert.match(bom, /key: 'productSpecTemplates'/)
  assert.match(bom, /targetKey: 'productSpecTemplates'/)
  assert.match(bom, /spec_template_version_id/)
  assert.match(bom, /specTemplatePage/)
})
