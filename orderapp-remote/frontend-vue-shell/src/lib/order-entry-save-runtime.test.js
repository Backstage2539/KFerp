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
  globalThis.window = { location: { origin: 'http://localhost', href: 'http://localhost/vue-shell?view=orders' } }
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
    if (path === '/api/order/stock-batch-preview') return response({ sufficient: false })
    if (path === '/api/order' && options.method === 'POST') {
      const payload = JSON.parse(options.body)
      writes.push(payload)
      if (rejectSave) return response({ error: '分两次保存' }, 409)
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
