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
 const app=createApp(component,{customerContextId:8,actor:{basic_auth_admin:true}});app.config.warnHandler=()=>{}
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
 const app=createApp(component,{viewParams:{},customerContextId:8,actor:{basic_auth_admin:true}});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();
  const tabs=[...document.querySelectorAll('[role="tab"]')];assert.deepEqual(tabs.map(t=>t.textContent.trim()),['概览','库存','订单','代发','费用'])
  assert.equal(tabs[0].getAttribute('aria-selected'),'true');assert.equal(document.querySelector('.fulfillment-orders-table'),null)
  assert.match(document.body.textContent,/131/);assert.equal(document.querySelector('input[type="file"]'),null)
  button('库存').click();await flush();assert.match(document.body.textContent,/可立即发货/);assert.equal(button('调整库存').disabled,true)
  button('订单').click();await flush();assert.ok(document.querySelector('.fulfillment-orders-table'))
  button('概览').click();await flush();document.querySelector('[data-todo="pending_shipment"]').click();await flush();assert.ok(calls.some(u=>u.includes('fulfillment_queue=pending_shipment')))
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('费用页默认打开未出账流水，提供日期快捷方式且不渲染代发表单', async () => {
 const {component}=await loadComponent('../views/CustomerFulfillmentView.vue')
 const original=globalThis.fetch
 globalThis.fetch=async url=>{const path=String(url);if(path.includes('/overview'))return response({customer_id:8,customer_name:'测试客户',capabilities:['settlement','direct_ship'],operations:{},assets:[]});if(path.includes('/customers'))return response({customers:[{id:8,name:'测试客户'}]});if(path.includes('/settings'))return response({modes:[]});if(path.includes('/options'))return response({});return response({imports:[],rows:[],summary:{orders:0}})}
 const app=createApp(component,{viewParams:{},customerContextId:8,actor:{basic_auth_admin:true}});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();button('费用').click();await flush()
  assert.ok(document.querySelector('.billing-workspace'))
  assert.ok(button('未出账'))
  assert.ok(button('已出账'))
  assert.equal(document.querySelector('.direct-ship-panel'),null)
  for(const label of ['全部日期','上月','当月','自定义']) assert.ok(button(label),`missing date option ${label}`)
  assert.equal(document.querySelector('.settlement-row'),null)
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('费用页按选择来源预览出账并生成已出账账单', async () => {
 const {component}=await loadComponent('../views/CustomerFulfillmentView.vue')
 const original=globalThis.fetch;let previewRequest=null;let issueRequest=null
 const today=new Intl.DateTimeFormat('en-CA',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date())
 const orderCharge={id:'order_goods:order:61',charge_type:'order_goods',source_type:'order',source_id:61,source_no:'SO-FEE-61',occurred_at:today,description:'订单款项',receivable_cents:10000,received_cents:7500,due_cents:2500,confirmation_status:'confirmed',settlement_status:'unsettled'}
 const issuedBill={batch_id:9,customer_id:8,settlement_no:'CB-9',customer_name:'测试客户',period_from:today,period_to:today,fee_items:1,total_amount_cents:2500,current_received_cents:0,current_due_amount_cents:2500,lines:[{...orderCharge,amount_cents:2500,issued_received_cents:7500}]}
 globalThis.fetch=async (url,options={})=>{const path=String(url);if(path.includes('/overview'))return response({customer_id:8,customer_name:'测试客户',capabilities:['settlement','direct_ship'],operations:{},assets:[],fees:[]});if(path.includes('/customers'))return response({customers:[{id:8,name:'测试客户'}]});if(path.includes('/settings/sales-order/seals'))return response({rows:[]});if(path.includes('/settings'))return response({modes:[]});if(path.includes('/billing/summary'))return response({total_unpaid_cents:2500,unbilled_cents:2500,issued_cents:0});if(path.includes('/billing/unbilled'))return response({rows:[orderCharge],total:1,page:1,limit:25});if(path.endsWith('/settlements/preview')){previewRequest=JSON.parse(options.body);return response({preview_version:'version-1',customer_id:8,customer_name:'测试客户',period_from:today,period_to:today,fee_items:1,total_amount_cents:2500,lines:[{...orderCharge,amount_cents:2500,issued_received_cents:7500}]})}if(path.endsWith('/settlements/issue')){issueRequest=JSON.parse(options.body);return response(issuedBill)}if(path.endsWith('/billing/bills/9/pdf-settings'))return response({customer_id:8,settlement_id:9,seal_asset_id:0,placements:[]});if(path.endsWith('/billing/bills/9'))return response(issuedBill);if(path.includes('/billing/bills'))return response({rows:[issuedBill],total:1,page:1,limit:25});return response({imports:[],rows:[],summary:{orders:0}})}
 const app=createApp(component,{viewParams:{},customerContextId:8,actor:{basic_auth_admin:true}});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush();button('费用').click();await flush()
  const sourceCheckbox=document.querySelector('input[aria-label="选择SO-FEE-61"]');assert.ok(sourceCheckbox);sourceCheckbox.checked=true;sourceCheckbox.dispatchEvent(new Event('change',{bubbles:true}));await flush()
  button('出账（1）').click();await flush()
  assert.match(document.querySelector('.billing-preview').textContent,/SO-FEE-61/)
  assert.match(document.querySelector('.billing-preview').textContent,/100\.00/)
  assert.match(document.querySelector('.billing-preview').textContent,/75\.00/)
  assert.match(document.querySelector('.billing-preview').textContent,/25\.00/)
  assert.deepEqual(previewRequest.sources,[{charge_type:'order_goods',source_type:'order',source_id:61}])
  button('确认出账').click();await flush()
  assert.equal(issueRequest.preview_version,'version-1')
  assert.ok(issueRequest.idempotency_key)
  assert.match(document.body.textContent,/已出账 CB-9，1 笔，合计 ¥25\.00/)
  assert.ok(document.querySelector('.billing-bill-drawer'))
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})

test('customer uses the same five modules, bound identity and no internal management controls', async () => {
 const {component}=await loadComponent('../views/CustomerFulfillmentView.vue')
 const original=globalThis.fetch, calls=[]
 globalThis.fetch=async (url, options={})=>{ const path=String(url);calls.push(path)
  if(path.includes('/fulfillment/overview'))return response({customer_id:149,customer_name:'客户甲',capabilities:['direct_ship','product_order','settlement'],operations:{},assets:[],fees:[]})
  if(path.includes('/direct-ship/settings'))return response({modes:[{mode:'stock',enabled:true,warehouse_code:'C149'}]})
  if(path.includes('/billing/summary'))return response({total_unpaid_cents:100,unbilled_cents:100,issued_cents:0})
  if(path.includes('/billing/unbilled'))return response({rows:[{id:'order-1',source_type:'order',source_id:1,source_no:'SO-1',due_cents:100}],total:1,page:1})
  return response({rows:[],summary:{orders:0}})
 }
 const app=createApp(component,{viewParams:{customer_id:999},customerContextId:999,customerAccountActor:true,actor:{account_type:'channel_customer',permissions:['customer_processing.read','customer_processing.submit']}});app.config.warnHandler=()=>{}
 try{app.mount(document.querySelector('#app'));await flush()
  assert.match(document.body.textContent,/客户甲/)
  for(const label of ['概览','库存','订单','代发','费用']) assert.ok(button(label),label)
  assert.equal(button('客户设置'),undefined)
  button('库存').click();await flush();assert.equal(button('调整库存'),undefined)
  button('费用').click();await flush();assert.equal(button('出账'),undefined);assert.equal(document.querySelector('input[aria-label="选择SO-1"]'),null)
  button('代发').click();await flush();assert.ok(button('库存代发'));assert.ok(calls.some(x=>x.includes('customer_id=149')));assert.ok(!calls.some(x=>x.includes('/api/customer-fulfillment/')))
 }finally{app.unmount();globalThis.fetch=original;document.body.innerHTML='<div id="app"></div>'}
})
