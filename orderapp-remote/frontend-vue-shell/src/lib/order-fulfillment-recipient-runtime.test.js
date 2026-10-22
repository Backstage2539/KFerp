import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { createSSRApp, h, nextTick } from 'vue'
import { renderToString } from '@vue/server-renderer'

async function setupView(relative, props, emitted = []) {
  const filename = new URL(relative, import.meta.url)
  const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
  let code = compileScript(descriptor, { id: 'fulfillment-recipient-regression' }).content
  code = code.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, q, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, filename).href : import.meta.resolve(path))}`)
  const { default: View } = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
  let editor
  await renderToString(createSSRApp({ setup() {
    editor = View.setup(props, { expose() {}, emit: (...args) => emitted.push(args) })
    return () => h('div')
  } }))
  return editor
}

const storedOrder = (mode = 'stock') => ({
  edit_id: 117, customer_id: 8, edit_revision: 'original-version', can_edit: true,
  portal_service_code: 'direct_ship', fulfillment_mode: mode,
  receiver_name: '收件人', receiver_phone: '13800000000', receiver_address: '原收件地址', receiver_company: '原公司',
  items: [{ item_id: 21, product_id: 7, parent_product_id: 7, bom_spec_id: 19, bom_variant_id: 1185,
    product_name: '库存商品', bom_spec_name: '227g袋装', spec: '227g袋装', qty: '4', unit: '袋',
    unit_price: '0.00', price_source_json: '{"source":"prepaid_customer_stock"}', bean_list_publication_id: 0 }],
})

test('stock and on-demand address edits preserve stored lines without consulting current price tables', async () => {
  const editor = await setupView('../views/OrderEntryView.vue', { embedded: true, editId: 117, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} })
  for (const mode of ['stock', 'on_demand']) {
    const order = storedOrder(mode)
    editor.applyEditData(order)
    editor.form.receiver_address = '只修改本单地址'
    editor.syncRowsForType({ priceListChanged: true })
    editor.repriceHydratedRows()
    const row = editor.rows.value[0]
    assert.equal(row.product_id, 7, 'address edit must not erase the prepaid product')
    assert.equal(row.bom_spec_id, 19)
    assert.equal(row.bom_variant_id, 1185)
    assert.equal(String(row.qty), '4')
    assert.equal(row.unit_price, '0.00')
    assert.equal(row.price_source_json, order.items[0].price_source_json)
    assert.equal(row.spec_invalid_message || '', '')
    assert.equal(editor.fulfillmentOrderEditing.value, true)
    assert.equal(await editor.refreshStaffOrderCatalog(), true)
  }
  editor.applyEditData({ ...storedOrder(), edit_id: 0, fulfillment_mode: '' })
  assert.equal(editor.fulfillmentOrderEditing.value, false, 'new ordinary orders still require published prices')
  await nextTick()
})

test('recipient save submits only recipient fields and revision, retains input on error, and ignores duplicate clicks', async () => {
  const emitted = []
  const editor = await setupView('../components/FulfillmentRecipientEditor.vue', { order: storedOrder(), recipientTarget: '', workflowTarget: '', disabledReason: '' }, emitted)
  const previousFetch = globalThis.fetch
  const previousWindow = globalThis.window
  const calls = []
  globalThis.window = { location: { origin: 'http://localhost', href: 'http://localhost/app/vue-shell', pathname: '/app/vue-shell' }, localStorage: { getItem() { return '' } } }
  try {
    let finish
    globalThis.fetch = (url, options) => { calls.push({ url, options }); return new Promise(resolve => { finish = resolve }) }
    editor.recipient.receiver_address = '新收件地址'
    const pending = editor.save()
    await editor.save()
    assert.equal(calls.length, 1)
    assert.equal(calls[0].url, 'http://localhost/app/api/orders/117/recipient')
    assert.equal(calls[0].options.method, 'PATCH')
    assert.deepEqual(JSON.parse(calls[0].options.body), {
      edit_revision: 'original-version', receiver_name: '收件人', receiver_phone: '13800000000',
      receiver_address: '新收件地址', receiver_company: '原公司',
    })
    finish({ ok: false, status: 409, json: async () => ({ error: '订单已被其他操作修改，请重新打开后再编辑' }) })
    await pending
    assert.match(editor.error.value, /其他操作修改/)
    assert.equal(editor.recipient.receiver_address, '新收件地址')
    assert.equal(emitted.length, 0)
    globalThis.fetch = async () => ({ ok: true, status: 200, json: async () => ({ ok: true, order_id: 117 }) })
    await editor.save()
    assert.equal(emitted[0][0], 'saved')
    assert.equal(emitted[0][1].order_id, 117)
    assert.equal(editor.error.value, '')
  } finally { globalThis.fetch = previousFetch; globalThis.window = previousWindow }
})

test('refresh keeps the fulfillment recipient editor visible until the new order snapshot arrives', async () => {
  const editor = await setupView('../views/OrderEntryView.vue', { embedded: true, editId: 117, copyId: 0, customerPortal: false, fulfillmentMode: false, viewParams: {} })
  editor.applyEditData(storedOrder())
  const previousFetch = globalThis.fetch
  const previousWindow = globalThis.window
  let finish
  globalThis.window = { location: { origin: 'http://localhost', href: 'http://localhost/vue-shell?edit_id=117', pathname: '/vue-shell' }, localStorage: { getItem() { return '' } } }
  globalThis.fetch = url => String(url).includes('/api/order/form')
    ? new Promise(resolve => { finish = resolve })
    : Promise.resolve({ ok: true, status: 200, json: async () => ({ rows: [], groups: [], assignments: [] }) })
  const loading = editor.load()
  try {
    assert.equal(editor.loading.value, true)
    assert.equal(editor.fulfillmentOrderEditing.value, true, 'refresh must not expose ordinary sales editing')
    assert.equal(editor.rows.value[0].bom_spec_id, 19)
  } finally {
    finish({ ok: true, status: 200, json: async () => ({ edit_mode: true, edit_id: 117, edit_data: storedOrder(), customers: [], products: [], product_families: [], product_bom_spec_options: [] }) })
    await loading
    globalThis.fetch = previousFetch
    globalThis.window = previousWindow
  }
})
