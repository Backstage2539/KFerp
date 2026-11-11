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
 assert.match(html, /缺料 1 kg/)
 assert.match(html, /等待前道生产 8 kg/)
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

function renderedRow(html, id) {
 const match = html.match(new RegExp(`<article id="component-source-${id}"[\\s\\S]*?</article>`))
 assert.ok(match, `missing source ${id}`)
 return match[0]
}

test('only unmet non-upstream requirement offers a transfer, with upstream separately visible', async () => {
 const Component = await component()
 const sources = [
  {id: 21, component_name: '前道熟豆', unit: 'kg', demand_g: 43584, required_g: 0, upstream_g: 43584, wip_covered_g: 0},
  {id: 22, component_name: '混合熟豆', unit: 'kg', demand_g: 10000, required_g: 2000, upstream_g: 8000, wip_covered_g: 1000},
  {id: 23, component_name: '纯现货', unit: 'kg', demand_g: 3000, required_g: 3000, upstream_g: 0, wip_covered_g: 0},
  {id: 24, component_name: '齐套包材', unit: '件', demand_units: 5, required_units: 5, wip_covered_units: 5},
 ]
 const html = await renderToString(Vue.createSSRApp(Component, {sources, editable: true, planningDraft: true}))
 const upstream = renderedRow(html, 21)
 assert.match(upstream, /等待前道生产 43\.584 kg/)
 assert.doesNotMatch(upstream, /缺料|生成调拨单/)
 const mixed = renderedRow(html, 22)
 assert.match(mixed, /等待前道生产 8 kg/)
 assert.match(mixed, /缺料 1 kg/)
 assert.match(mixed, /生成调拨单/)
 assert.match(renderedRow(html, 23), /缺料 3 kg/)
 assert.match(renderedRow(html, 23), /生成调拨单/)
 assert.match(renderedRow(html, 24), /充足/)
 assert.doesNotMatch(renderedRow(html, 24), /生成调拨单|等待前道/)
})

test('pending transfers reduce the quantity available to create without hiding upstream', async () => {
 const Component = await component()
 const sources = [
  {id: 31, component_name: '待确认熟豆', unit: 'kg', demand_g: 10000, required_g: 2000, upstream_g: 8000, wip_covered_g: 1000},
  {id: 32, component_name: '部分待确认', unit: 'kg', demand_g: 10000, required_g: 4000, upstream_g: 6000, wip_covered_g: 1000},
 ]
 const documents = [{id: 1, source_id:31,status:'draft',qty_g:1000}, {id: 2, source_id:32,status:'draft',qty_g:1000}]
 const html = await renderToString(Vue.createSSRApp(Component, {sources, documents, editable: true, planningDraft: true}))
 assert.match(renderedRow(html, 31), /等待前道生产 8 kg/)
 assert.doesNotMatch(renderedRow(html, 31), /生成调拨单/)
 assert.match(renderedRow(html, 32), /等待前道生产 6 kg/)
 assert.match(renderedRow(html, 32), /缺料 3 kg/)
 assert.match(renderedRow(html, 32), /生成调拨单/)
})

test('finished-product units are not treated as grams when both representations exist', async () => {
 const Component = await component()
 const sources = [{id: 41, component_type:'product', component_name:'盒装半成品', unit:'kg', demand_g:2270, demand_units:5, required_g:908, required_units:2, upstream_g:1362, upstream_units:3, wip_covered_g:454, wip_covered_units:1}]
 const html = await renderToString(Vue.createSSRApp(Component, {sources, editable: true, planningDraft: true}))
 const row = renderedRow(html, 41)
 assert.match(row, /等待前道生产 3 件/)
 assert.match(row, /缺料 1 件/)
 assert.match(row, /生成调拨单/)
})

test('submitted historical task preserves its original weight display when product has grams and units', async () => {
 const Component = await component()
 const sources = [{id: 51, component_type:'product', component_name:'历史半成品', unit:'kg', demand_g:2270, demand_units:5, required_g:908, required_units:2, upstream_g:1362, upstream_units:3, wip_covered_g:454, wip_covered_units:1, preparation_status:'awaiting_production'}]
 const html = await renderToString(Vue.createSSRApp(Component, {sources, editable:false, planningDraft:false, showNote:false}))
 assert.match(html, /2\.27 kg/)
 assert.match(html, /0\.454 kg/)
 assert.match(html, /待生产入库 1\.362 kg/)
 assert.doesNotMatch(html, /5 件|等待前道生产/)
})
