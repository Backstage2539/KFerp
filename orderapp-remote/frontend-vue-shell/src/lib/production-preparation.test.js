import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import { renderToString } from '@vue/server-renderer'

function component() {
 const { descriptor } = parse(fs.readFileSync(new URL('../components/ProductionPreparation.vue', import.meta.url), 'utf8'))
 const code = compileScript(descriptor, { id: 'preparation-test', inlineTemplate: true }).content
  .replace(/import \{([^}]+)\} from ["']vue["'];?/g, (_, names) => `const {${names.replace(/\bas\b/g, ':')}} = Vue;`)
  .replace('export default', 'return')
 return new Function('Vue', code)(Vue)
}
const source = { id: 39, picking_version: 1, component_name: '孟连水洗A', unit: 'kg', demand_g: 21000, wip_covered_g: 6000, transfer_g: 15000, shortage_g: 0, preparation_status: 'awaiting_transfer', allocations: [
 {warehouse:'wip',warehouse_name:'在制仓',owner_name:'工厂',qty_g:6000},
 {warehouse:'raw_materials',warehouse_name:'A仓',owner_name:'工厂',qty_g:10000},
 {warehouse:'backup',warehouse_name:'B仓',owner_name:'工厂',qty_g:5000},
] }
test('renders preparation without mandatory per-material source selection', async () => {
 const html=await renderToString(Vue.createSSRApp(component(),{sources:[source],editable:true}))
 for(const label of ['孟连水洗A','21 kg','6 kg','15 kg','供给已落实','A仓','B仓']) assert.ok(html.includes(label),label)
 assert.doesNotMatch(html, /请选择来源|<select/)
 assert.match(html, /<details/)
})
test('shows an explicit real shortage and distinguishes upstream supply', async () => {
 const html=await renderToString(Vue.createSSRApp(component(),{sources:[{...source,shortage_g:3000,preparation_status:'shortage'},{...source,id:40,component_name:'熟豆',upstream_g:4000,preparation_status:'awaiting_production'}]}))
 assert.match(html,/缺料/);assert.match(html,/3 kg/);assert.match(html,/待生产入库/)
})
test('empty preparation state remains readable', async () => {
 const html=await renderToString(Vue.createSSRApp(component(),{sources:[]}))
 assert.match(html,/暂无备料项/)
})
