import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../views/OrderEntryView.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'order-save-runtime' }).content
code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const response = (data, status = 200) => ({ ok: status < 400, status, json: async () => data })

async function fixture(t, { rejectSave = false, failRefresh = false } = {}) {
  const oldFetch = globalThis.fetch, oldWindow = globalThis.window
  const stored = new Map([['auth_token','employee-test-token']])
  globalThis.window = { location: { origin: 'http://localhost', href: 'http://localhost/vue-shell?view=orders' }, navigator: {locks: {request: async (_key,_options,fn) => fn({})}}, localStorage: {getItem: k => stored.get(k) || null, setItem: (k,v) => stored.set(k,v), removeItem: k => stored.delete(k)} }
  t.after(() => { globalThis.fetch = oldFetch; globalThis.window = oldWindow })
  let editor
  const events = [], writes = []
  await renderToString(createSSRApp({ setup() {
    editor = View.setup({ embedded: true, editId: 12, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} }, { expose() {}, emit: (...args) => events.push(args) })
    return () => h('div')
  } }))
  const initial = { edit_id: 12, edit_revision: 'revision-1', customer_id: 8, pay_status_id: 2, payment_method: '现金',
    prepayment_amount: '0', shipping_amount: '0', freight: { token: 'freight-1', current_receivable_cents: 0, charge_mode: 'manual_current' },
    items: [{ item_id: 21, product_id: 7, product_name: '测试商品', spec: '227g', qty: 1, unit: '袋', unit_price: '120', manual_price: true, tier_id: 'manual' }] }
  editor.payStatuses.value = [{ id: 1, name: '未付款' }, { id: 2, name: '已付款' }, { id: 3, name: '预付款（付款未完成）' }]
  editor.applyEditData(initial)
  editor.freightEstimator.value = { preview: async () => ({ token: editor.orderFreight.value.token }), payload: () => ({ expected_freight_token: editor.orderFreight.value.token, shipping_amount: editor.form.shipping_amount }) }
  editor.form.shipping_amount = '20'
  const saved = { ...initial, edit_revision: 'revision-2', pay_status_id: 3, prepayment_amount: '120', shipping_amount: '20', freight: { ...initial.freight, token: 'freight-2', current_receivable_cents: 2000 } }
  let reads = 0
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/auth/me') return response({employee_id: 11})
    if (path === '/api/order/stock-batch-preview') return response({ sufficient: false })
    if (path === '/api/order' && options.method === 'POST') {
      const payload = JSON.parse(options.body)
      writes.push(payload)
      if (rejectSave) return response(typeof rejectSave === 'object' ? rejectSave : { error: '分两次保存' }, 409)
      if (writes.length > 1 && (payload.edit_revision !== 'revision-2' || payload.expected_freight_token !== 'freight-2')) return response({ error: '旧版本' }, 409)
      return response({ order_id: 12, order_no: 'SO-TEST-12', edited: true })
    }
    if (path === '/api/order/form') { reads++; return failRefresh ? response({ error: '读取失败' }, 503) : response({ edit_mode: true, edit_id: 12, edit_data: saved }) }
    return response({ rows: [], groups: [], assignments: [] })
  }
  return { editor, events, writes, saved, reads: () => reads }
}

test('successful ordinary edit refreshes payment, receipt and both revision tokens before a second save', async t => {
  const { editor, writes, reads } = await fixture(t)
  await editor.save()
  assert.equal(editor.error.value, '')
  assert.equal(editor.form.edit_revision, 'revision-2')
  assert.equal(editor.orderFreight.value.token, 'freight-2')
  assert.equal(editor.form.pay_status_id, 3)
  assert.equal(editor.form.prepayment_amount, '120')
  assert.equal(editor.form.payment_voucher_asset_id, 0)
  await editor.save()
  assert.equal(editor.error.value, '')
  assert.equal(writes.length, 2)
  assert.equal(reads(), 2)
  assert.equal(writes[1].prepayment_amount, '120')
})

test('rejected save keeps the editable draft and does not reload or emit success', async t => {
  const { editor, events, reads } = await fixture(t, { rejectSave: true })
  const before = JSON.stringify({ form: editor.form, rows: editor.rows.value })
  await editor.save()
  assert.equal(editor.error.value, '分两次保存')
  assert.equal(JSON.stringify({ form: editor.form, rows: editor.rows.value }), before)
  assert.equal(reads(), 0)
  assert.equal(events.filter(([name]) => name === 'saved').length, 0)
})

test('a committed save with a failed refresh is reported as saved and blocks stale resubmission', async t => {
  const { editor, events, writes } = await fixture(t, { failRefresh: true })
  await editor.save()
  assert.match(editor.error.value, /已保存.*重新打开/)
  assert.ok(editor.editBlockReason.value)
  assert.equal(events.filter(([name]) => name === 'saved').length, 1)
  await editor.save()
  assert.equal(writes.length, 1)
})

test('locked ordinary order blocks the whole-form save and explains the independent payment entry', async t => {
  const { editor, writes } = await fixture(t)
  editor.applyEditData({ ...editor.fulfillmentEditData.value, freight: { ...editor.orderFreight.value, edit_locked: true } })
  assert.match(editor.editBlockReason.value, /已确认实际费用.*编辑付款与发货状态/)
  await editor.save()
  assert.equal(writes.length, 0)
})

test('closing a rejected draft and reopening hydrates stored fees and receipt without carrying unsaved changes', async t => {
  const { editor, saved } = await fixture(t)
  editor.applyEditData(saved)
  editor.form.prepayment_amount = '55'
  editor.form.discount_amount = '1'
  editor.applyEditData(saved)
  assert.equal(editor.form.prepayment_amount, '120')
  assert.equal(editor.form.discount_amount, '')
  assert.equal(editor.form.edit_revision, 'revision-2')
})

test('zero price is confined to a server-linked sample line; ordinary zero remains blocked', async t => {
  const { editor } = await fixture(t)
  const sample = { ...editor.fulfillmentEditData.value, is_sample_order: true, settlement_reason: 'free_sample',
    items: [{ item_id: 21, product_id: 7, parent_product_id: 7, bom_spec_id: 9, bom_variant_id: 10, spec: '227g', qty: 1, unit_price: '0', tier_id: 'manual' }] }
  editor.applyEditData(sample)
  assert.equal(editor.rowHasBlockingPrice(editor.rows.value[0]), false)
  assert.equal(editor.freeSampleSettlement.value, true)
  editor.applyEditData({ ...sample, is_sample_order: false, settlement_reason: '' })
  assert.equal(editor.rowHasBlockingPrice(editor.rows.value[0]), true)
  assert.equal(editor.hasValidManualPrice(editor.rows.value[0]), false)
})

test('read-back replaces cumulative receipt and voucher display with the persisted values', async t => {
  const { editor, saved } = await fixture(t)
  Object.assign(saved, { payment_goods_amount: '120.00', payment_shipping_amount: '0.00', payment_voucher_asset_id: 71, payment_voucher: { id: 71, filename: 'synthetic.png' } })
  await editor.save()
  assert.equal(editor.error.value, '')
  assert.equal(editor.form.payment_goods_amount, '120.00')
  assert.equal(editor.form.payment_shipping_amount, '0.00')
  assert.equal(editor.form.payment_voucher_asset_id, 71)
  assert.equal(editor.paymentVoucher.value.filename, 'synthetic.png')
})


test('retained payment save requires its customer warehouse but no recipient or freight, and preserves hidden history', async t => {
  for (const historical of [false, true]) {
    const { editor, writes, saved } = await fixture(t)
    const retained = { ...saved, delivery_mode: 'customer_stock', delivery_warehouse_code: 'CUSTOMER-8',
      shipping_amount: '0', pay_status_id: 3, payment_method: '现金', prepayment_amount: '30',
      receiver_name: historical ? '历史收件人' : '', receiver_phone: historical ? '13800000000' : '',
      receiver_address: historical ? '历史地址' : '', ship_tracking_no: historical ? 'OLD-TRACKING' : '', ship_status_id: 1 }
    Object.assign(saved, retained)
    editor.applyEditData(retained)
    editor.freightEstimator.value = { preview: () => { throw new Error('retained order must not quote freight') } }
    await editor.save()
    assert.match(editor.error.value, /当前客户的启用成品仓库/)
    assert.equal(writes.length, 0)
    editor.deliveryWarehouses.value = [{ code: 'CUSTOMER-8', customer_id: 8, kind: 'finished', active: true }]
    await editor.save()
    assert.equal(editor.error.value, '')
    assert.equal(writes.length, 1)
    for (const key of ['receiver_name', 'receiver_phone', 'receiver_address', 'ship_tracking_no', 'ship_status_id']) assert.equal(writes[0][key], retained[key], key)
    assert.equal(Number(writes[0].shipping_amount), 0)
    assert.equal(writes[0].prepayment_amount, '30')
    assert.equal(writes[0].payment_method, '现金')
  }
})

test('new retained purchase hides and clears draft receipt values while existing orders retain them', async t => {
  const { editor, writes } = await fixture(t)
  window.navigator = {}
  editor.form.edit_id = 0
  editor.form.delivery_mode = 'customer_stock'
  editor.form.delivery_warehouse_code = 'CUSTOMER-8'
  editor.deliveryWarehouses.value = [{ code: 'CUSTOMER-8', customer_id: 8, kind: 'finished', active: true }]
  Object.assign(editor.form, { pay_status_id: 2, payment_method: '现金', prepayment_amount: '30', payment_goods_amount: '120', payment_voucher_asset_id: 71 })
  assert.equal(editor.newRetainedPurchase.value, true)
  assert.equal(editor.paymentReceiptVisible.value, false)
  assert.equal(editor.paymentMethodRequired.value, false)
  await editor.save()
  assert.equal(editor.error.value, '')
  assert.equal(writes.length, 1)
  assert.equal(writes[0].pay_status_id, 0)
  assert.equal(writes[0].payment_method, '')
  assert.equal(writes[0].prepayment_amount, '0')
  assert.equal(writes[0].payment_goods_amount, '')
  assert.equal(writes[0].payment_voucher_asset_id, 0)
})

test('嵌入录单重开后按员工恢复原始请求并复用请求标识', async t => {
 const oldFetch=globalThis.fetch,oldWindow=globalThis.window
 const entries=new Map([['auth_token','employee-test-token']]), localStorage={getItem:k=>entries.get(k)||null,setItem:(k,v)=>entries.set(k,v),removeItem:k=>entries.delete(k)}
 const alerts=[]
 globalThis.window={location:{origin:'http://localhost',href:'http://localhost/vue-shell?view=orders'},navigator:{locks:{request:async(_key,_options,fn)=>fn({})}},localStorage,alert:(message)=>alerts.push(message)}
 t.after(()=>{globalThis.fetch=oldFetch;globalThis.window=oldWindow})
 const pending={route:'/api/order',payload:{request_id:'initial-order-request-123',customer_id:3,prepayment_amount:'20',receipt_date:'2026-11-10'}}
 localStorage.setItem('kferp.order-create.pending.employee-11',JSON.stringify(pending))
 let received=null
 globalThis.fetch=async(url,options={})=>{if(String(url).includes('/api/auth/me'))return response({employee_id:11,permissions:['orders.write']});received=JSON.parse(options.body);return response({order_id:71,order_no:'SO-71',receipt_warning:'请财务核对'})}
 let editor;const events=[]
 await renderToString(createSSRApp({setup(){editor=View.setup({embedded:true,editId:0,copyId:12,customerPortal:false,fulfillmentMode:false,viewParams:{}},{expose(){},emit:(...args)=>events.push(args)});return ()=>h('div')}}))
 await editor.restorePendingOrderCreate();assert.deepEqual(editor.pendingOrderCreate.value,pending)
 await editor.retryPendingOrderCreate();assert.deepEqual(received,pending.payload);assert.equal(localStorage.getItem('kferp.order-create.pending.employee-11'),null);assert.deepEqual(alerts,['请财务核对']);assert.equal(events[0][0],'saved')
})

test('首次提交明确未提交可修改；结果不明后重试的拒绝必须保留原请求', async t => {
 const {editor,writes}=await fixture(t,{rejectSave:{error:'价格规则需要确认',not_committed:true}})
 Object.assign(editor.form,{edit_id:0,delivery_mode:'ship',prepayment_amount:'30',payment_goods_amount:'30',payment_shipping_amount:'0',payment_method:'现金',pay_status_id:3,receipt_date:'2026-11-10'})
 await editor.save()
 assert.equal(writes.length,1)
 assert.equal(editor.pendingOrderCreate.value,null)
 assert.equal(window.localStorage.getItem('kferp.order-create.pending.employee-11'),null)
 assert.equal(editor.form.customer_request_id,'')
 const pending={route:'/api/order',payload:{request_id:'initial-order-request-456',customer_id:8}}
 window.localStorage.setItem('kferp.order-create.pending.employee-11',JSON.stringify(pending))
 await editor.restorePendingOrderCreate()
 await editor.retryPendingOrderCreate()
 assert.deepEqual(editor.pendingOrderCreate.value,pending)
 assert.deepEqual(JSON.parse(window.localStorage.getItem('kferp.order-create.pending.employee-11')),pending)
})

test('同一员工另一页面保存中时阻止并发新建', async t => {
 const {editor,writes}=await fixture(t)
 let held=false, release
 window.navigator.locks.request=async(_key,_opts,fn)=>{
   if(held)return fn(null)
   held=true
   try{return await fn({})}finally{held=false}
 }
 const prior=editor.withNewOrderLock(()=>new Promise(resolve=>{release=resolve}))
 while(!release) await new Promise(resolve=>setTimeout(resolve,0))
 Object.assign(editor.form,{edit_id:0,delivery_mode:'ship',prepayment_amount:'30',payment_method:'现金',pay_status_id:3,receipt_date:'2026-11-10'})
 await editor.save()
 assert.match(editor.error.value,/另一个页面正在保存订单/)
 assert.equal(writes.length,0)
 release();await prior
})

test('获取录单锁后切换登录账号不会读取新账号待确认订单或提交', async t => {
 const {editor,writes}=await fixture(t)
 Object.assign(editor.form,{edit_id:0,delivery_mode:'ship',prepayment_amount:'30',payment_method:'现金',pay_status_id:3,receipt_date:'2026-11-10'})
 window.localStorage.setItem('kferp.order-create.pending.employee-11',JSON.stringify({route:'/api/order',payload:{request_id:'pending-order-identity-test',customer_id:8}}))
 window.navigator.locks.request=async(_key,_options,fn)=>{window.localStorage.setItem('auth_token','other-user-token');return fn({})}
 await editor.save()
 assert.match(editor.error.value,/账号已变化/)
 assert.equal(editor.pendingOrderCreate.value,null)
 assert.equal(writes.length,0)
 assert.ok(window.localStorage.getItem('kferp.order-create.pending.employee-11'))
})

test('初始预付款先校验分项，纠正后提交；全款从明确的分项计算已收总额', async t => {
 for (const full of [false,true]) {
  const {editor,writes}=await fixture(t)
  Object.assign(editor.form,{edit_id:0,delivery_mode:'ship',prepayment_amount:full?'0':'30',payment_goods_amount:full?'70':'',payment_shipping_amount:full?'20':'',outsource_material_fee:full?'10':'',payment_method:'现金',payment_voucher_asset_id:full?71:0,pay_status_id:full?2:3,receipt_date:'2026-11-10'})
  if(full) editor.effectiveCopyID.value=12
  assert.equal(editor.paymentReceiptVisible.value,true)
  if(!full){await editor.save();assert.equal(writes.length,0);assert.match(editor.error.value,/货款和运费分项/);editor.form.payment_goods_amount='30';editor.form.payment_shipping_amount='0'}
  await editor.save()
  assert.equal(writes.length,1,`${editor.error.value} total=${JSON.stringify(editor.orderTotalPreviewValue.value)} item=${editor.itemsTotal.value} shipping=${editor.form.shipping_amount}`)
  assert.equal(writes[0].prepayment_amount,full?'90.00':'30')
  if(full) assert.equal(writes[0].outsource_material_fee,'10')
 }
})

test('管理员只能在服务端核对解除成功后清除原请求', async t => {
 const oldFetch=globalThis.fetch,oldWindow=globalThis.window
 const entries=new Map([['auth_token','employee-test-token']]), storage={getItem:k=>entries.get(k)||null,setItem:(k,v)=>entries.set(k,v),removeItem:k=>entries.delete(k)}
 const pending={route:'/api/order',payload:{request_id:'initial-order-admin-test-123',customer_id:3,prepayment_amount:'20'}}
 storage.setItem('kferp.order-create.pending.employee-11',JSON.stringify(pending))
 globalThis.window={location:{origin:'http://localhost',href:'http://localhost/'},localStorage:storage,navigator:{locks:{request:async(_key,_options,fn)=>fn({})}},prompt:()=> '已核对'}
 t.after(()=>{globalThis.fetch=oldFetch;globalThis.window=oldWindow})
 let calls=0
 globalThis.fetch=async(url,options={})=>{if(String(url).includes('/api/auth/me'))return response({employee_id:11,roles:[{code:'admin'}]});assert.equal(options.headers.Authorization,'Bearer employee-test-token');assert.deepEqual(JSON.parse(options.body).original,pending.payload);calls++;return response(calls===1?{status:'committed',order_id:71}:{status:'cancelled'})}
 let editor
 await renderToString(createSSRApp({setup(){editor=View.setup({embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}},{expose(){},emit(){}});return ()=>h('div')}}))
 await editor.restorePendingOrderCreate();assert.equal(editor.pendingOrderCanResolve.value,true)
 await editor.resolvePendingOrderCreate();assert.match(editor.error.value,/已有订单 71/);assert.ok(storage.getItem('kferp.order-create.pending.employee-11'))
 await editor.resolvePendingOrderCreate();assert.equal(storage.getItem('kferp.order-create.pending.employee-11'),null);assert.match(editor.ok.value,/已核对无订单/)
})

test('管理员按员工请求号解除后员工核对成功才能清除本机待确认', async t => {
 const oldFetch=globalThis.fetch,oldWindow=globalThis.window
 const entries=new Map([['auth_token','employee-test-token']]),storage={getItem:k=>entries.get(k)||null,setItem:(k,v)=>entries.set(k,v),removeItem:k=>entries.delete(k)}
 const requestID='employee-order-request-12345',pending={route:'/api/order',payload:{request_id:requestID,customer_id:3}}
 storage.setItem('kferp.order-create.pending.employee-11',JSON.stringify(pending))
 globalThis.window={location:{origin:'http://localhost',href:'http://localhost/'},localStorage:storage,navigator:{locks:{request:async(_key,_options,fn)=>fn({})}},prompt:()=> '员工提交失败后核对'}
 t.after(()=>{globalThis.fetch=oldFetch;globalThis.window=oldWindow})
 let authorized=false,cancelled=false
 globalThis.fetch=async(url,options={})=>{
  const path=new URL(String(url),window.location.origin).pathname
  if(path==='/api/auth/me')return response({employee_id:11,roles:authorized?[{code:'admin'}]:[]})
  if(path.endsWith('/status'))return response({cancelled})
  if(path==='/api/order/pending-request/resolve'){assert.equal(authorized,true);assert.equal(JSON.parse(options.body).request_id,requestID);cancelled=true;return response({status:'cancelled'})}
  throw Error('unexpected request '+path)
 }
 let editor;await renderToString(createSSRApp({setup(){editor=View.setup({embedded:true,editId:0,copyId:0,customerPortal:false,fulfillmentMode:false,viewParams:{}},{expose(){},emit(){}});return ()=>h('div')}}))
 await editor.restorePendingOrderCreate();await editor.checkPendingOrderCancelled();assert.ok(storage.getItem('kferp.order-create.pending.employee-11'));assert.match(editor.error.value,/尚未解除/)
 authorized=true;await editor.restorePendingOrderCreate();editor.adminResolveRequestID.value=requestID;await editor.resolveOtherPendingOrderCreate();assert.equal(cancelled,true);assert.ok(storage.getItem('kferp.order-create.pending.employee-11'))
 authorized=false;await editor.restorePendingOrderCreate();await editor.checkPendingOrderCancelled();assert.equal(storage.getItem('kferp.order-create.pending.employee-11'),null)
})
