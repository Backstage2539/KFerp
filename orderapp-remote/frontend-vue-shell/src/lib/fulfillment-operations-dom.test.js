import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'http://localhost/vue-shell?view=orders', pretendToBeVisual: true })
for (const key of ['window', 'document', 'Node', 'Element', 'HTMLElement', 'SVGElement', 'Event', 'MouseEvent', 'CustomEvent']) globalThis[key] = dom.window[key]
const { createApp, nextTick } = await import('vue')
const components = new Map()
async function loadComponent(relative) {
  const file = new URL(relative, import.meta.url)
  if (components.has(file.href)) return components.get(file.href)
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
  let code = compileScript(descriptor, { id: file.pathname, inlineTemplate: true }).content
  const imports = [...code.matchAll(/import (\w+) from ['"]([^'"]+\.vue)['"]/g)]
  for (const [statement, name, path] of imports) {
    if (['FulfillmentInventory', 'FulfillmentOperationsOverview'].includes(name)) {
      const dependency = await loadComponent(new URL(path, file).href)
      code = code.replace(statement, `import ${name} from ${JSON.stringify(dependency.url)}`)
    } else code = code.replace(statement, `const ${name}={name:${JSON.stringify(name)},render(){return null}}`)
  }
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : path.startsWith('data:') ? path : import.meta.resolve(path))}`)
  const url = `data:text/javascript;base64,${Buffer.from(code).toString('base64')}`
  const entry = { url, component: (await import(url)).default }
  components.set(file.href, entry)
  return entry
}


const response = data => ({ok:true,status:200,json:async()=>data})
async function flush(){for(let i=0;i<5;i++){await nextTick();await new Promise(r=>setImmediate(r))}}
const button = text => [...document.querySelectorAll('button')].find(b=>b.textContent.trim()===text)
test('留厂订单主状态为生产进度，详情无收件与发货区域，库存代发仍可见', async () => {
 const {component}=await loadComponent('../views/CustomerFulfillmentView.vue')
 const original=globalThis.fetch
 const retained={id:115,order_no:'SO-RETAINED-2',delivery_mode:'customer_stock',delivery_status:'pending_receipt',process_status:'待生产',ship_status:'待发货',portal_service_code:'product_order'}
 const dispatch={id:117,order_no:'SO-DISPATCH',delivery_mode:'ship',fulfillment_mode:'stock',ship_status:'待发货',receiver_name:'测试收件人',receiver_address:'测试地址'}
 globalThis.fetch=async url=>{const path=String(url);if(path.includes('/overview'))return response({customer_id:8,capabilities:[],operations:{},assets:[]});if(path.includes('/customers'))return response({customers:[{id:8,name:'测试客户'}]});if(path.includes('/detail'))return response({edit_data:{...retained,items:[]}});return response({rows:[retained,dispatch],summary:{orders:2}})}
 const app=createApp(component,{customerContextId:8});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();button('订单').click();await flush()
  const rows=document.querySelectorAll('.fulfillment-orders-table tbody tr')
  assert.match(rows[0].textContent,/待生产/);assert.match(rows[0].textContent,/留厂库存.*无需发货/)
  assert.doesNotMatch(rows[0].textContent,/待发货|待补收件|出库单/)
  assert.match(rows[1].textContent,/待发货/);assert.match(rows[1].textContent,/测试收件人/)
  rows[0].querySelector('.order-link').click();await flush()
  const drawer=document.querySelector('.order-detail-drawer')
  assert.doesNotMatch(drawer.textContent,/收件与快递|收件人：|发货：/)
  assert.match(drawer.textContent,/待生产/)
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})
test('operations opens on todos, keeps five historical modules and gates only new actions',async()=>{
 const {component}=await loadComponent('../views/CustomerFulfillmentView.vue')
 const original=globalThis.fetch;const calls=[]
 globalThis.fetch=async url=>{calls.push(String(url));if(String(url).includes('/overview'))return response({customer_id:8,customer_name:'测试客户',capabilities:[],operations:{pending_shipment:131},assets:[]});if(String(url).includes('/customers'))return response({customers:[{id:8,name:'测试客户'}]});return response({rows:[],summary:{orders:131},modes:[]})}
 const app=createApp(component,{viewParams:{},customerContextId:8});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();
  const tabs=[...document.querySelectorAll('[role="tab"]')];assert.deepEqual(tabs.map(t=>t.textContent.trim()),['概览','库存','订单','代发','费用'])
  assert.equal(tabs[0].getAttribute('aria-selected'),'true');assert.equal(document.querySelector('.fulfillment-orders-table'),null)
  assert.match(document.body.textContent,/131/);assert.equal(document.querySelector('input[type="file"]'),null)
  button('库存').click();await flush();assert.match(document.body.textContent,/可立即发货/);assert.equal(button('调整库存').disabled,true)
  button('订单').click();await flush();assert.ok(document.querySelector('.fulfillment-orders-table'))
  button('概览').click();await flush();document.querySelector('[data-todo="pending_shipment"]').click();await flush();assert.ok(calls.some(u=>u.includes('fulfillment_queue=pending_shipment')))
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})
