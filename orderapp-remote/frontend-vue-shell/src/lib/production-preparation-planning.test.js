import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import * as Vue from 'vue'
import { compileScript, parse } from '@vue/compiler-sfc'
import { renderToString } from '@vue/server-renderer'

async function component() {
 const file = new URL('../components/ProductionPreparation.vue', import.meta.url)
 const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
 const code = compileScript(descriptor, { id: 'planning-preparation', inlineTemplate: true }).content
 const bindings = {}
 for (const [, names] of code.matchAll(/import\s+\{([^}]+)\}\s+from\s+["']vue["']/g)) for (const part of names.split(',')) { const [name, alias] = part.trim().split(/\s+as\s+/); bindings[alias || name] = Vue[name] }
 return new Function(...Object.keys(bindings), code.replace(/^import\s+[\s\S]*?\s+from\s+(['"])vue\1;?\s*$/gm, '').replace('export default', 'return'))(...Object.values(bindings))
}

test('planning shows per-task WIP coverage and full demand including upstream', async () => {
 const Component = await component()
 const sources = [
  {id: 1, component_name: '熟豆', unit: 'kg', demand_g: 10000, required_g: 2000, upstream_g: 8000, wip_covered_g: 1000},
  {id: 2, component_name: '包装袋', unit: '件', demand_units: 5, wip_covered_units: 5},
 ]
 const html = await renderToString(Vue.createSSRApp(Component, {sources, editable: true, planningDraft: true}))
 assert.match(html, /WIP库存/)
 assert.match(html, /缺料 9 kg/)
 assert.match(html, /充足/)
 assert.equal((html.match(/生成调拨单/g) || []).length, 1)
 assert.doesNotMatch(html, /其他仓可调|真实缺口|待领|查看明细/)
})

test('planning displays only the current material linked transfer number, amount and status', async () => {
 const Component = await component()
 const sources = [{id: 1, component_name: '熟豆', unit: 'kg', demand_g: 10000, required_g: 10000, wip_covered_g: 1000}]
 const documents = [{id: 7, source_id: 1, entry_no:'SE-PLAN-7', status:'draft', qty_g:2000}, {id: 8, source_id: 2, entry_no:'SE-OTHER', status:'submitted', qty_g:3000}]
 const html = await renderToString(Vue.createSSRApp(Component, {sources, documents, editable: true, planningDraft: true}))
 assert.match(html, /SE-PLAN-7/)
 assert.match(html, /待仓库确认/)
 assert.match(html, /2 kg/)
 assert.doesNotMatch(html, /SE-OTHER/)
})
