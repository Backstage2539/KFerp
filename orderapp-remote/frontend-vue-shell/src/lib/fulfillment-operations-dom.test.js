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
    if (['DataPanel', 'FulfillmentInventory', 'FulfillmentOperationsOverview'].includes(name)) {
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
 const retained={id:115,order_no:'SO-RETAINED-2',delivery_mode:'customer_stock',delivery_status:'pending_receipt',process_status:'待生产',ship_status:'待发货',portal_service_code:'product_order',fulfillment_progress:{label:'生产完成 · 已入库',can_ship:false}}
 const dispatch={id:117,order_no:'SO-DISPATCH',delivery_mode:'ship',fulfillment_mode:'stock',ship_status:'待发货',receiver_name:'测试收件人',receiver_address:'测试地址'}
 globalThis.fetch=async url=>{const path=String(url);if(path.includes('/overview'))return response({customer_id:8,capabilities:[],operations:{},assets:[]});if(path.includes('/customers'))return response({customers:[{id:8,name:'测试客户'}]});if(path.includes('/detail'))return response({edit_data:{...retained,items:[]}});return response({rows:[retained,dispatch],summary:{orders:2}})}
 const app=createApp(component,{customerContextId:8});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();button('订单').click();await flush()
  const rows=document.querySelectorAll('.fulfillment-orders-table tbody tr')
  assert.match(rows[0].textContent,/生产完成 · 已入库/);assert.match(rows[0].textContent,/留厂库存.*无需发货/)
  assert.doesNotMatch(rows[0].textContent,/待发货|待补收件|出库单/)
  assert.match(rows[1].textContent,/待发货/);assert.match(rows[1].textContent,/测试收件人/)
  rows[0].querySelector('.order-link').click();await flush()
  const drawer=document.querySelector('.order-detail-drawer')
  assert.doesNotMatch(drawer.textContent,/收件与快递|收件人：|发货：/)
  assert.match(drawer.textContent,/生产完成 · 已入库/)
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

test('生成月结直接选择日期方式，费用面板不渲染代发表单', async () => {
 const {component}=await loadComponent('../views/CustomerFulfillmentView.vue')
 const original=globalThis.fetch
 globalThis.fetch=async url=>{const path=String(url);if(path.includes('/overview'))return response({customer_id:8,customer_name:'测试客户',capabilities:['settlement','direct_ship'],operations:{},assets:[]});if(path.includes('/customers'))return response({customers:[{id:8,name:'测试客户'}]});if(path.includes('/settings'))return response({modes:[]});if(path.includes('/options'))return response({});return response({imports:[],rows:[],summary:{orders:0}})}
 const app=createApp(component,{viewParams:{},customerContextId:8});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();button('费用').click();await flush();button('生成月结').click();await flush()
  assert.ok(document.querySelector('.settlement-row'))
  assert.equal(document.querySelector('.direct-ship-panel'),null)
  for(const label of ['上月','当月','自定义日期']) assert.ok(button(label),`missing date option ${label}`)
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('费用页区分确认与结算状态，月结展示订单应收已收未收并保留成功提示', async () => {
 const {component}=await loadComponent('../views/CustomerFulfillmentView.vue')
 const original=globalThis.fetch;let settlementRequest=null
 const today=new Intl.DateTimeFormat('en-CA',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date())
 const orderCharge={charge_type:'order_goods',source_type:'order',source_id:61,source_no:'SO-FEE-61',occurred_at:today,receivable_cents:10000,amount_cents:2500,received_cents:7500,due_cents:2500,confirmation_status:'confirmed',settlement_status:'unsettled'}
 const overview={customer_id:8,customer_name:'测试客户',capabilities:['settlement','direct_ship'],operations:{},assets:[],order_charges:[orderCharge],fees:[
  {id:1,fee_type:'shipping',fee_name:'预估运费',source:'order',source_id:61,occurred_at:`${today} 10:00`,amount_cents:1200,status:'unsettled',confirmation_status:'pending_confirmation',settlement_status:'unsettled',received_cents:0,due_cents:1200},
  {id:2,fee_type:'direct_ship_service',fee_name:'代发服务费',source:'order',source_id:62,occurred_at:`${today} 11:00`,amount_cents:500,status:'unsettled',confirmation_status:'confirmed',settlement_status:'unsettled',received_cents:0,due_cents:500},
 ]}
 globalThis.fetch=async (url,options={})=>{const path=String(url);if(path.includes('/overview'))return response(overview);if(path.includes('/customers'))return response({customers:[{id:8,name:'测试客户'}]});if(path.includes('/settings'))return response({modes:[]});if(path.endsWith('/settlements')){settlementRequest=JSON.parse(options.body);return response({batch_id:9,settlement_no:'CS-9',fee_items:2,total_amount_cents:3000,received_cents:7500,due_amount_cents:3000})}return response({imports:[],rows:[],summary:{orders:0}})}
 const app=createApp(component,{viewParams:{},customerContextId:8});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();button('费用').click();await flush()
  assert.match(document.querySelector('.fee-confirmed').textContent,/已确认/)
  assert.match(document.querySelector('.fee-unconfirmed').textContent,/待确认/)
  button('生成月结').click();await flush()
  assert.match(document.querySelector('.settlement-order-preview').textContent,/SO-FEE-61/)
  assert.match(document.querySelector('.settlement-order-preview').textContent,/100\.00/)
  assert.match(document.querySelector('.settlement-order-preview').textContent,/75\.00/)
  assert.match(document.querySelector('.settlement-order-preview').textContent,/25\.00/)
  const submit=document.querySelector('.settlement-row .primary');submit.click();await flush()
  assert.deepEqual(settlementRequest.charge_types,['order_goods','shipping','direct_ship_service'])
  assert.match(document.body.textContent,/已生成月结 CS-9/)
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})
