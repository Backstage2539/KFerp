import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

const filename = new URL('../components/OrderBatchStatusDialog.vue', import.meta.url)
const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
let code = compileScript(descriptor, { id: 'batch-status-test' }).content
code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
const { default: Dialog } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)

test('batch dialog saves each order once and retries only failed orders', async () => {
  let view
  const events = []
  const props = { selectedRows: [{ id: 1 }, { id: 2 }], payStatuses: [], shipStatuses: [] }
  await renderToString(createSSRApp({ setup() { view = Dialog.setup(props, { expose() {}, emit: (...args) => events.push(args) }); return () => h('div') } }))
  const original = globalThis.fetch
  const requests = []
  globalThis.fetch = async (url, options) => {
    const body = JSON.parse(options.body)
    requests.push({ url: String(url), body })
    if (String(url).endsWith('/preview')) return { ok: true, json: async () => ({ orders: [1, 2].map(id => ({ order_id: id, edit_revision: `v${id}`, payment_goods_amount: `${id}0`, payment_voucher_asset_id: id + 10 })) }) }
    return { ok: true, json: async () => ({ results: body.orders.map(item => ({ order_id: item.order_id, status: item.order_id === 1 ? 'updated' : requests.filter(r => r.url.endsWith('status-batch')).length === 1 ? 'failed' : 'updated', error: item.order_id === 2 ? '请检查资料' : '' })) }) }
  }
  try {
    await view.readOrders([1, 2])
    view.payStatusID.value = 3
    view.drafts.value[1].payment_goods_amount = '29.50'
    await view.submit()
    assert.equal(view.drafts.value[0].result, 'updated')
    assert.equal(view.drafts.value[1].result, 'failed')
    assert.equal(requests[1].body.orders[0].payment_voucher_asset_id, 11)
    assert.equal(requests[1].body.orders[1].payment_goods_amount, 29.5)
    await view.submit()
    assert.deepEqual(requests[2].body.orders.map(row => row.order_id), [2])
    assert.equal(events.filter(e => e[0] === 'updated').length, 2)
  } finally { globalThis.fetch = original }
})

async function setupDialog() {
  let view
  const props = { selectedRows: [{ id: 1 }], payStatuses: [], shipStatuses: [] }
  await renderToString(createSSRApp({ setup() { view = Dialog.setup(props, { expose() {}, emit() {} }); return () => h('div') } }))
  return view
}

const previewOrder = (revision = 'v1', extra = {}) => ({ order_id: 1, order_no: 'SO-1', edit_revision: revision, prepayment_amount: '0', payment_goods_amount: '10', payment_shipping_amount: '0', payment_voucher_asset_id: 11, ...extra })

test('preview failure is visible and can be retried', async () => {
  const view = await setupDialog()
  const original = globalThis.fetch
  let attempts = 0
  globalThis.fetch = async () => ++attempts === 1
    ? { ok: false, status: 500, json: async () => ({ error: '读取失败' }) }
    : { ok: true, json: async () => ({ orders: [previewOrder()] }) }
  try {
    await view.readOrders([1])
    assert.equal(view.error.value, '读取失败')
    assert.equal(view.drafts.value.length, 0)
    await view.readOrders([1])
    assert.equal(view.error.value, '')
    assert.equal(view.drafts.value[0].edit_revision, 'v1')
  } finally { globalThis.fetch = original }
})

test('unknown submission result requires refresh before retry', async () => {
  const view = await setupDialog()
  const original = globalThis.fetch
  let submissions = 0
  globalThis.fetch = async url => {
    if (String(url).endsWith('/preview')) return { ok: true, json: async () => ({ orders: [previewOrder(submissions ? 'v2' : 'v1')] }) }
    submissions++
    throw new Error('网络中断')
  }
  try {
    await view.readOrders([1])
    view.shipStatusID.value = 3
    await view.submit()
    assert.equal(view.drafts.value[0].requiresReload, true)
    assert.equal(view.hasPending.value, false)
    await view.submit()
    assert.equal(submissions, 1)
    await view.reloadFailures()
    assert.equal(view.drafts.value[0].edit_revision, 'v2')
    assert.equal(view.hasPending.value, true)
  } finally { globalThis.fetch = original }
})

test('refresh after version conflict preserves edited amount and newly uploaded voucher', async () => {
  const view = await setupDialog()
  const original = globalThis.fetch
  let previewCount = 0
  globalThis.fetch = async (url, options) => {
    if (String(url).endsWith('/preview')) {
      previewCount++
      return { ok: true, json: async () => ({ orders: [previewCount === 1 ? previewOrder() : previewOrder('v2', { payment_goods_amount: '20', prepayment_amount: '5' })] }) }
    }
    if (String(url).endsWith('/payment-vouchers')) return { ok: true, json: async () => ({ asset: { id: 77, filename: '新凭证.png' } }) }
    return { ok: true, json: async () => ({ results: [{ order_id: 1, status: 'failed', error: '订单已被其他操作修改' }] }) }
  }
  try {
    await view.readOrders([1])
    view.payStatusID.value = 3
    view.drafts.value[0].payment_goods_amount = '30'
    await view.uploadVoucher(view.drafts.value[0], { target: { files: [new Blob(['voucher'])], value: 'x' } })
    await view.submit()
    assert.equal(view.drafts.value[0].result, 'failed')
    await view.reloadFailures()
    const row = view.drafts.value[0]
    assert.equal(row.edit_revision, 'v2')
    assert.equal(row.saved.payment_goods_amount, '20')
    assert.equal(row.payment_goods_amount, '30')
    assert.equal(row.prepayment_amount, '5')
    assert.equal(row.payment_voucher_asset_id, 77)
    assert.equal(row.payment_voucher_filename, '新凭证.png')
  } finally { globalThis.fetch = original }
})

test('failed voucher upload cannot silently submit old voucher', async () => {
  const view = await setupDialog()
  const original = globalThis.fetch
  let submissions = 0
  globalThis.fetch = async url => {
    if (String(url).endsWith('/preview')) return { ok: true, json: async () => ({ orders: [previewOrder()] }) }
    if (String(url).endsWith('/payment-vouchers')) return { ok: false, status: 400, json: async () => ({ error: '上传失败' }) }
    submissions++
    return { ok: true, json: async () => ({ results: [] }) }
  }
  try {
    await view.readOrders([1])
    view.payStatusID.value = 3
    await view.uploadVoucher(view.drafts.value[0], { target: { files: [new Blob(['voucher'])], value: 'x' } })
    assert.equal(view.drafts.value[0].uploadError, true)
    assert.equal(view.hasPending.value, false)
    await view.submit()
    assert.equal(submissions, 0)
    assert.equal(view.drafts.value[0].payment_voucher_asset_id, 11)
  } finally { globalThis.fetch = original }
})

test('refresh after a failed replacement upload still requires explicit resolution', async () => {
  const view = await setupDialog()
  const original = globalThis.fetch
  let submissions = 0
  let uploadSucceeds = false
  globalThis.fetch = async url => {
    if (String(url).endsWith('/preview')) return { ok: true, json: async () => ({ orders: [previewOrder(submissions ? 'v2' : 'v1')] }) }
    if (String(url).endsWith('/payment-vouchers')) return uploadSucceeds
      ? { ok: true, json: async () => ({ asset: { id: 77, filename: '新凭证.png' } }) }
      : { ok: false, status: 400, json: async () => ({ error: '上传失败' }) }
    submissions++
    return { ok: true, json: async () => ({ results: [{ order_id: 1, status: 'failed', error: '订单已被其他操作修改' }] }) }
  }
  try {
    await view.readOrders([1])
    view.payStatusID.value = 3
    await view.submit()
    const upload = () => view.uploadVoucher(view.drafts.value[0], { target: { files: [new Blob(['voucher'])], value: 'x' } })
    await upload()
    await view.reloadFailures()
    assert.equal(view.drafts.value[0].uploadError, true)
    assert.match(view.drafts.value[0].resultError, /上传失败/)
    assert.equal(view.hasPending.value, false)
    await view.submit()
    assert.equal(submissions, 1)
    uploadSucceeds = true
    await upload()
    assert.equal(view.drafts.value[0].uploadError, false)
    assert.equal(view.hasPending.value, true)
    assert.equal(view.drafts.value[0].payment_voucher_asset_id, 77)
    await view.submit()
    uploadSucceeds = false
    await upload()
    await view.reloadFailures()
    assert.equal(view.hasPending.value, false)
    view.keepVoucher(view.drafts.value[0])
    assert.equal(view.hasPending.value, true)
    assert.equal(view.drafts.value[0].payment_voucher_asset_id, 77)
  } finally { globalThis.fetch = original }
})
