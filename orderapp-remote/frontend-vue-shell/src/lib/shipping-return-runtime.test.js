import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {parse,compileScript,compileTemplate} from '@vue/compiler-sfc'
import {createSSRApp,h} from 'vue'
import {renderToString} from '@vue/server-renderer'
import {freightLabel} from './freight.js'
import {shippingReturnUploadMessage} from './shipping-return-upload.js'
test('re-upload states historical weight corrections and conflicts without implying a shipment',()=>{
 assert.match(shippingReturnUploadMessage({rematched:true,pending:1,weight_backfilled:2,weight_skipped:3}),/补录寄件重量 2 条/)
 assert.match(shippingReturnUploadMessage({rematched:true,pending:1,weight_backfilled:2,weight_skipped:3}),/重量冲突或异常跳过 3 条/)
 assert.match(shippingReturnUploadMessage({rematched:true,pending:1}),/补录寄件重量 0 条/)
 assert.match(shippingReturnUploadMessage({rematched:true,pending:1}),/未执行发货/)
 assert.match(shippingReturnUploadMessage({updated:2,pending:1,skipped:0}),/已回填 2 条/)
})
async function loadView(name,props,emit){const file=new URL(`../components/${name}.vue`,import.meta.url);const {descriptor}=parse(fs.readFileSync(file,'utf8'),{filename:file.pathname});const script=compileScript(descriptor,{id:name});const template=compileTemplate({id:name,source:descriptor.template.content,filename:file.pathname,compilerOptions:{bindingMetadata:script.bindings}});assert.deepEqual(template.errors,[]);let code=script.content.replace(/from (['"])([^'"]+)\1/g,(_,quote,path)=>`from ${JSON.stringify(path.startsWith('.')?new URL(/\.[a-z]+$/.test(path)?path:`${path}.js`,file).href:import.meta.resolve(path))}`);const {default:View}=await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`);let vm;await renderToString(createSSRApp({setup(){vm=View.setup(props,{expose(){},emit});return()=>h('div')}}));return vm}
const ticket = (id, batch = 8, changes = {}) => ({
  id, batch_id: batch, order_id: 12, version: 'v1', can_select: true, actual_cents: 2010, settlement_weight_kg: '10.500', ...changes,
})

test('picker selects only explicitly checked tickets in the chosen batch and submits them together', async () => {
  const original = globalThis.fetch
  const requests = [], events = []
  globalThis.fetch = async (url, opts) => {
    requests.push({ url, body: JSON.parse(opts.body) })
    return { ok: true, status: 200, json: async () => ({ updated: 1 }) }
  }
  try {
    const rows = [ticket(50), ticket(51), ticket(49, 7)]
    const vm = await loadView('ShippingReturnPicker', { orderId: 12, orderNo: 'SO-TEST', rows }, (...args) => events.push(args))
    assert.deepEqual(vm.selectedIDs.value, [])
    await vm.submit()
    assert.equal(requests.length, 0)
    vm.selectedIDs.value = [50, 51]
    assert.equal(vm.selectedCents.value, 4020)
    assert.equal(vm.weightLabel.value, '结算重量 21.000 kg')
    vm.changeBatch(7)
    assert.deepEqual(vm.selectedIDs.value, [])
    vm.changeBatch(8)
    vm.selectedIDs.value = [50, 51]
    await vm.submit()
    assert.match(requests[0].url, /orders\/12\/shipping-return-selection$/)
    assert.deepEqual(requests[0].body, { candidate_ids: [50, 51], version: 'v1' })
    assert.deepEqual(events, [['saved']])
  } finally { globalThis.fetch = original }
})

test('picker reports missing parcel weights without treating them as zero', async () => {
  const rows = [ticket(50), ticket(51, 8, {settlement_weight_kg:''})]
  const vm = await loadView('ShippingReturnPicker', {orderId:12, orderNo:'SO-TEST', rows}, ()=>{})
  vm.selectedIDs.value = [50,51]
  assert.equal(vm.weightLabel.value, '已知结算重量 10.500 kg（1 票未记录）')
  vm.selectedIDs.value = [51]
  assert.equal(vm.weightLabel.value, '结算重量未记录（1 票）')
})

test('single ticket preserves its original request and a conflict stays in the dialog', async () => {
  const original = globalThis.fetch
  const requests = [], events = []
  globalThis.fetch = async (url, opts) => {
    requests.push(JSON.parse(opts.body))
    return { ok: false, status: 409, json: async () => ({ error: '订单已变化' }) }
  }
  try {
    const vm = await loadView('ShippingReturnPicker', { orderId: 12, orderNo: 'SO-TEST', rows: [ticket(44)] }, (...args) => events.push(args))
    vm.selectedIDs.value = [44]
    await vm.submit()
    assert.deepEqual(requests, [{ candidate_id: 44, version: 'v1' }])
    assert.match(vm.error.value, /订单已变化/)
    assert.deepEqual(events, [])
  } finally { globalThis.fetch = original }
})

test('report ties each ticket to its own row and counts matched orders only once', async () => {
  const { shippingReturnBatchMatches } = await import('./shipping-return-upload.js')
  const rows = [
    { id: 50, tracking_no: 'SF-A', settlement_weight_kg: '1.500', status: 'pending', candidates: [ticket(50), ticket(50, 8, {order_id: 13, can_select: false})] },
    { id: 51, tracking_no: 'SF-B', status: 'pending', candidates: [ticket(51)] },
    { id: 52, status: 'applied', order_id: 14 },
    { id: 53, status: 'unmatched' },
  ]
  const matches = shippingReturnBatchMatches(rows, 8)
  assert.equal(matches.orderCount, 3)
  assert.equal(matches.pendingCount, 2)
  assert.deepEqual(matches.candidatesByOrder.get(12).map(r => [r.id,r.tracking_no]), [[50, 'SF-A'], [51, 'SF-B']])
  assert.equal(matches.candidatesByOrder.get(13)[0].can_select, false)
})

test('template copying keeps source mapping independent',async()=>{const vm=await loadView('ShippingReturnSettings',{companies:[]},()=>{});vm.templates.value=[{id:'sf',name:'顺丰',revision:1,columns:{amount:{column:'S',header:'应收金额'}}}];vm.copy();vm.templates.value[1].columns.amount.column='Q';assert.equal(vm.templates.value[0].columns.amount.column,'S');assert.equal(vm.templates.value[1].revision,0)})
test('blocked, mixed-batch or stale tickets cannot be submitted', async () => {
  const original = globalThis.fetch
  let calls = 0
  globalThis.fetch = async () => { calls++; return {ok:true, status:200, json:async()=>({})} }
  try {
    const rows = [ticket(50), ticket(51, 8, { can_select:false }), ticket(48, 7)]
    const vm = await loadView('ShippingReturnPicker', { orderId:12, orderNo:'SO-TEST', rows }, ()=>{})
    for (const selection of [[50,51], [50,48], [50,999]]) {
      vm.selectedIDs.value = selection
      await vm.submit()
    }
    assert.equal(calls,0)
  } finally { globalThis.fetch = original }
})
test('shipping return settings and upload report expose optional pickup time and settlement weight',()=>{const settings=fs.readFileSync(new URL('../components/ShippingReturnSettings.vue',import.meta.url),'utf8');const orders=fs.readFileSync(new URL('../views/OrdersView.vue',import.meta.url),'utf8');for(const key of ['settlement_weight','pickup_time','weight_unit'])assert.ok(settings.includes(key),`missing ${key}`);assert.match(orders,/揽件时间：\{\{r\.pickup_time\|\|'未记录'\}\}/);assert.match(orders,/结算重量：\{\{r\.settlement_weight_kg/);})
test('confirmed actual cost displays final customer freight and receipts for manual orders',()=>{assert.match(freightLabel({charge_mode:'manual_current',current_receivable_cents:2000,final_receivable_cents:1400,received_freight_cents:500,due_freight_cents:900,actual_cost_cents:1200}),/最终运费 ¥14.00 · 已收 ¥5.00 · 待收 ¥9.00 · 成本 ¥12.00/)})

test('order cards include the actual freight adjustment exactly once',async()=>{const {customerFulfillmentOrderFees}=await import('./customer-fulfillment.js');const {inlineOrderTotals}=await import('./order-list-inline.js');const {fulfillmentFeeSummary,fulfillmentListFeeLines}=await import('./fulfillment-order-edit.js');const order={total_amount:100,grand_total:120,shipping_amount:20,freight:{final_receivable_cents:1400,actual_cost_cents:1200,received_freight_cents:500,due_freight_cents:900}};assert.equal(customerFulfillmentOrderFees(order).at(-1).value,'114');assert.equal(inlineOrderTotals(order).at(-1).value,'¥114.00');assert.equal(fulfillmentFeeSummary({service_fee_cents:200},10000,order.freight).total,11600);assert.equal(fulfillmentListFeeLines(order).find(x=>x.label==='最终运费').value,'14.00')})
