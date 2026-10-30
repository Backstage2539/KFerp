import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../components/OrderBatchStatusDialog.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'combined-payment-test' }).content
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: Dialog } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const rows = [
  { order_id: 1, order_no: 'SO-1', customer_id: 7, edit_revision: 'v1', total_cents: 10000, paid_cents: 3000, due_cents: 7000, fully_paid: false },
  { order_id: 2, order_no: 'SO-2', customer_id: 7, edit_revision: 'v2', total_cents: 5000, paid_cents: 5000, due_cents: 0, fully_paid: true },
]
function storage() { const data = new Map(); return { getItem: k => data.get(k) ?? null, setItem: (k,v) => data.set(k,String(v)), removeItem: k => data.delete(k) } }
async function setupDialog() {
  let view
  const events = []
  const props = { selectedRows: [{ id: 1 }, { id: 2 }], payStatuses: [], shipStatuses: [] }
  await renderToString(createSSRApp({ setup() { view = Dialog.setup(props, { expose() {}, emit: (...args) => events.push(args) }); return () => h('div') } }))
  return { view, events }
}
function response(value, status = 200) { return { ok: status >= 200 && status < 300, status, json: async () => value } }

test('合并收款使用整数分、统一凭证和发货状态并反馈待分配金额', async () => {
  const originalFetch = globalThis.fetch, originalWindow = globalThis.window
  globalThis.window = { sessionStorage: storage(), location: { origin: 'http://localhost', pathname: '/' } }
  const requests = []
  globalThis.fetch = async (url, options) => {
    const body = JSON.parse(options.body)
    requests.push({ url: String(url), body })
    if (String(url).endsWith('/preview')) return response({ orders: rows })
    return response({ batch_id: 10, actual_cents: 8000, due_cents: 7000, unallocated_cents: 1000, allocations: [{ order_id: 1, amount_cents: 7000 }] })
  }
  try {
    const { view, events } = await setupDialog()
    await view.readOrders([1, 2])
    view.paymentMode.value = 'combined'
    view.combinedActual.value = '80.00'
    view.combinedMethod.value = '微信支付'
    view.combinedVoucherID.value = 11
    view.shipStatusID.value = 4
    await view.submitCombined()
    assert.equal(requests[1].body.actual_cents, 8000)
    assert.equal(requests[1].body.expected_due_cents, 7000)
    assert.equal(requests[1].body.payment_voucher_asset_id, 11)
    assert.deepEqual(requests[1].body.orders, [{ order_id: 1, edit_revision: 'v1' }, { order_id: 2, edit_revision: 'v2' }])
    assert.ok(requests[1].body.idempotency_key)
    assert.equal(view.combinedResult.value.unallocated_cents, 1000)
    assert.ok(events.some(e => e[0] === 'updated'))
    assert.equal(globalThis.window.sessionStorage.getItem('kferp.order-combined-payment.pending'), null)
  } finally { globalThis.fetch = originalFetch; globalThis.window = originalWindow }
})

test('网络结果未知后重开窗口继续同一键和完全相同请求', async () => {
  const originalFetch = globalThis.fetch, originalWindow = globalThis.window
  globalThis.window = { sessionStorage: storage(), location: { origin: 'http://localhost', pathname: '/' } }
  const requests = []
  let attempts = 0
  globalThis.fetch = async (url, options) => {
    if (String(url).endsWith('/preview')) return response({ orders: rows })
    requests.push(JSON.parse(options.body))
    if (++attempts === 1) throw new Error('网络中断')
    return response({ batch_id: 11, actual_cents: 7000, due_cents: 7000, unallocated_cents: 0, allocations: [] })
  }
  try {
    const { view } = await setupDialog()
    await view.readOrders([1, 2]); view.paymentMode.value = 'combined'; view.combinedActual.value = '70'; view.combinedMethod.value = '微信支付'
    await view.submitCombined()
    assert.equal(view.combinedUncertain.value, true)
    const reopened = (await setupDialog()).view
    await reopened.readOrders([1, 2])
    assert.equal(reopened.combinedUncertain.value, true)
    await reopened.submitCombined()
    assert.deepEqual(requests[1], requests[0])
    assert.equal(reopened.combinedResult.value.batch_id, 11)
  } finally { globalThis.fetch = originalFetch; globalThis.window = originalWindow }
})

test('预览不同客户或实际收款不足不发送合并请求', async () => {
  const originalFetch = globalThis.fetch, originalWindow = globalThis.window
  globalThis.window = { sessionStorage: storage(), location: { origin: 'http://localhost', pathname: '/' } }
  const calls = []
  globalThis.fetch = async (url) => { calls.push(String(url)); return response({ orders: rows }) }
  try {
    const { view } = await setupDialog()
    await view.readOrders([1,2]); view.paymentMode.value = 'combined'; view.combinedActual.value = '69.99'; view.combinedMethod.value = '微信支付'
    await view.submitCombined()
    assert.equal(calls.length, 1)
    assert.match(view.error.value, /不足/)
    view.drafts.value[1].customer_id = 8
    view.combinedActual.value = '70.00'
    await view.submitCombined()
    assert.equal(calls.length, 1)
    assert.match(view.error.value, /同一客户/)
  } finally { globalThis.fetch = originalFetch; globalThis.window = originalWindow }
})

test('可确认的冲突不重试旧版本，必须重新读取', async () => {
  const originalFetch = globalThis.fetch, originalWindow = globalThis.window
  globalThis.window = { sessionStorage: storage(), location: { origin: 'http://localhost', pathname: '/' } }
  const requests = []
  globalThis.fetch = async (url, options) => {
    requests.push(String(url))
    if (String(url).endsWith('/preview')) return response({ orders: rows })
    return response({ error: '订单已被其他操作修改' }, 409)
  }
  try {
    const { view } = await setupDialog()
    await view.readOrders([1,2]); view.paymentMode.value = 'combined'; view.combinedActual.value = '70'; view.combinedMethod.value = '微信支付'
    await view.submitCombined()
    assert.equal(view.combinedUncertain.value, false)
    assert.equal(view.combinedNeedsRefresh.value, true)
    assert.match(view.error.value, /订单已被其他操作修改/)
    assert.equal(globalThis.window.sessionStorage.getItem('kferp.order-combined-payment.pending'), null)
    await view.submitCombined()
    assert.equal(requests.length, 2)
    await view.readOrders([1,2])
    assert.equal(view.combinedNeedsRefresh.value, false)
  } finally { globalThis.fetch = originalFetch; globalThis.window = originalWindow }
})

test('存储不可用时不发送付款请求', async () => {
  const originalFetch = globalThis.fetch, originalWindow = globalThis.window
  globalThis.window = { location: { origin: 'http://localhost', pathname: '/' } }
  const calls = []
  globalThis.fetch = async url => { calls.push(String(url)); return response({ orders: rows }) }
  try {
    const { view } = await setupDialog()
    await view.readOrders([1,2]); view.paymentMode.value = 'combined'; view.combinedActual.value = '70'; view.combinedMethod.value = '微信支付'
    assert.match(view.combinedStorageError.value, /存储不可用/)
    await view.submitCombined()
    assert.equal(calls.length, 1)
  } finally { globalThis.fetch = originalFetch; globalThis.window = originalWindow }
})

test('上传新凭证失败时不得提交或沿用未确认的旧凭证', async () => {
  const originalFetch = globalThis.fetch, originalWindow = globalThis.window
  globalThis.window = { sessionStorage: storage(), location: { origin: 'http://localhost', pathname: '/' } }
  const calls = []
  globalThis.fetch = async url => {
    calls.push(String(url))
    if (String(url).endsWith('/preview')) return response({ orders: rows })
    return response({ error: '上传失败' }, 400)
  }
  try {
    const { view } = await setupDialog()
    await view.readOrders([1,2]); view.paymentMode.value = 'combined'; view.combinedActual.value = '70'; view.combinedMethod.value = '微信支付'
    view.combinedVoucherID.value = 77
    await view.uploadCombinedVoucher({ target: { files: [new Blob(['voucher'])], value: 'x' } })
    assert.match(view.combinedVoucherError.value, /上传失败/)
    await view.submitCombined()
    assert.equal(calls.length, 2)
    view.clearCombinedVoucher()
    assert.equal(view.combinedVoucherID.value, 0)
    assert.equal(view.combinedVoucherError.value, '')
  } finally { globalThis.fetch = originalFetch; globalThis.window = originalWindow }
})
