import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../views/CustomerFulfillmentView.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'shared-workbench-runtime' }).content
code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)

test('客户工作台按本人身份读取数据，费用入口不使用内部写接口', async t => {
 const oldWindow=globalThis.window, oldFetch=globalThis.fetch
 globalThis.window={ location:{origin:'http://localhost',href:'http://localhost/?customer_id=999'}, localStorage:{getItem:()=>''}, dispatchEvent:()=>true }
 t.after(()=>{globalThis.window=oldWindow;globalThis.fetch=oldFetch})
 const calls=[]
 globalThis.fetch=async (url,init={})=>{ const path=new URL(String(url),'http://localhost').pathname;calls.push([path,init.method||'GET']);const data=path.endsWith('/overview')?{customer_id:149,customer_name:'客户甲',capabilities:['settlement','direct_ship'],assets:[]}:path.endsWith('/settings')?{modes:[]}:{rows:[],total:0};return new Response(JSON.stringify(data),{status:200,headers:{'Content-Type':'application/json'}})}
 let view
 await renderToString(createSSRApp({setup(){view=View.setup({customerAccountActor:true,viewParams:{customer_id:999},customerContextId:999,customerContextLabel:'',workspaceMode:'',actor:{permissions:['customer_processing.read']}},{expose(){},emit(){}});return()=>h('div')}}))
 view.customerId.value=149
 await view.loadAll();await view.loadBillingWorkspace(1)
 assert.equal(view.overview.value.customer_id,149)
 assert(calls.some(([path])=>path==='/api/customer-processing/portal/fulfillment/overview'))
 assert(calls.some(([path])=>path==='/api/customer-processing/portal/fulfillment/billing/unbilled'))
 assert(!calls.some(([path])=>path.startsWith('/api/customer-fulfillment/')))
 assert(calls.every(([,method])=>method==='GET'))
 assert.match(descriptor.template.content,/v-if="activeTab === 'inventory' && canInternal\('stock.write'\)"/)
 assert.match(descriptor.template.content,/v-if="activeTab === 'fees' && billingView === 'unbilled' && canInternal\('stock.write'\)"/)
})
