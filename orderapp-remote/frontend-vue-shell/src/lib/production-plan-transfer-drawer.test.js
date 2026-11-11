import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'

const file = new URL('../components/ProductionPlanTransferDrawer.vue', import.meta.url)
async function renderDrawer(Component, props) {
  const context = {}
  await renderToString(createSSRApp(Component, props), context)
  return context.teleports?.body || ''
}
async function component() {
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
  const compiled = compileScript(descriptor, { id: 'plan-transfer-drawer', inlineTemplate: true })
  const bindings = {}
  for (const [, imports] of compiled.content.matchAll(/import\s+\{([^}]+)\}\s+from\s+"vue"/g)) for (const part of imports.split(',')) { const [imported, local] = part.trim().split(/\s+as\s+/); bindings[local || imported] = Vue[imported] }
  for (const binding of Object.values(compiled.imports)) {
    const mod = binding.source === 'vue' ? Vue : await import(new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, file))
    bindings[binding.local] = mod[binding.imported]
  }
  return new Function(...Object.keys(bindings), compiled.content.replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\s*$/gm, '').replace('export default', 'return'))(...Object.values(bindings))
}

test('single transfer shows accurate WIP shortage and does not post inventory', async () => {
  const Component = await component()
  const source = { id: 5, component_name: '生豆', unit: 'kg', demand_g: 6767, wip_covered_g: 1000 }
  const html = await renderDrawer(Component, { detail: { id: 26, plan_no: 'PP-26' }, source, mode: 'single' })
  assert.match(html, /生豆/)
  assert.match(html, /5\.767 kg/)
  assert.match(html, /仓库确认/)
  assert.match(html, /生成调拨单/)
})

test('bulk drawer does not claim all shortage has available inventory', async () => {
  const Component = await component()
  const html = await renderDrawer(Component, { detail: { id: 26 }, mode: 'bulk' })
  assert.match(html, /生成所有缺料调拨单/)
  assert.match(html, /暂不生成/)
  assert.match(html, /仓库确认/)
})

test('stock view exposes location and batch identity without a submit action', async () => {
  const Component = await component()
  const html = await renderDrawer(Component, { detail: { id: 26 }, source: { id: 5, component_name: '生豆', unit: 'kg' }, mode: 'stock' })
  assert.match(html, /物料库存/)
  assert.match(html, /仓库/)
  assert.match(html, /批次/)
  assert.doesNotMatch(html, /生成调拨单/)
})
