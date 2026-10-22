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
    if (['OrderEntryView', 'FulfillmentRecipientEditor', 'FulfillmentOrderFees', 'OrderItemWeights'].includes(name)) {
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

const order = {
  id: 117, order_no: 'SO-TEST-117', customer_id: 8, customer: '测试客户',
  portal_service_code: 'direct_ship', fulfillment_mode: 'stock', confirmation_required: false,
  total_amount: '0.00', shipping_amount: '0.00', grand_total: '0.00',
}
const items = [{ item_id: 21, product_id: 7, parent_product_id: 7, bom_spec_id: 19, bom_variant_id: 1185,
  product_name: '已购商品', bom_spec_name: '227g袋装', spec: '227g袋装', qty: '4', unit: '袋', unit_price: '0.00',
  bean_list_publication_id: 0, price_source_json: '{"source":"prepaid_customer_stock"}' }]
const editData = { ...order, edit_id: 117, edit_revision: 'revision-1', can_edit: true,
  order_date: '2026-10-22', receiver_name: '张测试', receiver_phone: '13800000000', receiver_address: '原地址', notes: '原备注', items }
const formData = { edit_mode: true, edit_id: 117, edit_data: editData, customers: [{ id: 8, name: '测试客户' }],
  products: [], product_families: [], product_bom_spec_options: [], bean_list_version_options: [] }
const fees = { service_fee_cents: 1400, calculated_service_fee_cents: 1400, shipping_fee_cents: 0,
  calculated_shipping_fee_cents: 810, packaging_weight_g: 500, chargeable_weight_g: 1580, shipping_template_revision: 2,
  freight: { estimate_known: true, estimate: { base_cents: 810 }, estimated_total_cents: 810, current_receivable_cents: 0, status: 'pending_actual' } }
const context = { ...editData, order_id: 117, catalog: { current_customer_id: 8, fulfillment_mode: 'stock', source_warehouse: 'customer-warehouse', product_families: [{
  parent_product_id: 7, name: '已购商品', specs: [{ product_id: 7, bom_spec_id: 19, bom_variant_id: 1185,
    spec_label: '227g袋装', inventory_unit: '袋', physical_available_qty: 4, pending_inbound_qty: 2, available_qty: 6, own_reserved_qty: 4 },
    { product_id: 7, bom_spec_id: 20, bom_variant_id: 1186, spec_label: '454g袋装', inventory_unit: '袋', physical_available_qty: 0, pending_inbound_qty: 3, available_qty: 3 }],
  }], price_tables: [] }, fees }
const response = (data, status = 200) => ({ ok: status < 400, status, json: async () => data })
async function flush() { for (let i = 0; i < 5; i++) { await nextTick(); await new Promise(resolve => setImmediate(resolve)) } }
function button(text, root = document) { return [...root.querySelectorAll('button')].find(node => node.textContent.trim() === text) }

test('drawer sender always displays the batch default and preserves an explicit choice across refresh and reopen', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  let resolveProfiles
  globalThis.fetch = async (url) => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/settings/sender') return new Promise(resolve => { resolveProfiles = () => resolve(response({ profiles: [{ sender_id: 5, sender_label: '营业部', is_default: true }, { sender_id: 9, sender_label: '发货人' }] })) })
    if (path === '/api/orders') return response({ rows: [order], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail' || path === '/api/order/form') return response(formData)
    if (path === '/api/orders/117/fulfillment') return response(context)
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders)
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    const sender = () => document.querySelector('[aria-label="选择寄件人"]')
    assert.equal(sender().value, '0', 'unset per-order choice displays follow-default even while profiles load')
    assert.match(sender().selectedOptions[0].textContent, /默认寄件人/)
    resolveProfiles()
    await flush()
    button('刷新').click()
    await flush()
    assert.equal(sender().value, '0', 'list refresh cannot clear the visible sender selection')
    assert.match(sender().selectedOptions[0].textContent, /营业部/)
    sender().value = '9'
    sender().dispatchEvent(new Event('change', { bubbles: true }))
    await flush()
    button('刷新').click()
    await flush()
    assert.equal(sender().value, '9', 'list refresh preserves the open order sender choice')
    button('关闭', document.querySelector('.drawer-head')).click()
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    assert.equal(sender().value, '9', 'reopening preserves an explicit sender')
    sender().value = '0'
    sender().dispatchEvent(new Event('change', { bubbles: true }))
    await flush()
    assert.match(sender().selectedOptions[0].textContent, /营业部/)
  } finally {
    app.unmount()
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})

test('area A alone shows recalculated draft fees and restores saved fees when the editor is reopened', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  const previewFees = { ...fees, service_fee_cents: 1600, calculated_service_fee_cents: 1600 }
  globalThis.fetch = async (url) => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/orders') return response({ rows: [order], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail' || path === '/api/order/form') return response(formData)
    if (path === '/api/orders/117/fulfillment') return response(context)
    if (path === '/api/orders/117/fulfillment/preview') return response({ items, goods_total_cents: 0, fees: previewFees, preview_token: 'fees-preview', edit_revision: 'revision-1' })
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders)
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    assert.match(document.querySelector('.order-summary .fulfillment-fees').textContent, /代发服务费¥14.00/)
    button('重新核算代发费用').click()
    await flush()
    assert.match(document.querySelector('.order-summary .fulfillment-fees').textContent, /代发服务费¥16.00/, 'recalculation updates the retained summary, not a hidden duplicate')
    assert.equal(document.querySelectorAll('.order-detail-drawer .fulfillment-fees').length, 1)
    button('关闭', document.querySelector('.drawer-head')).click()
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    assert.match(document.querySelector('.order-summary .fulfillment-fees').textContent, /代发服务费¥14.00/, 'unsaved preview cannot leak into a reopened order')
  } finally {
    app.unmount()
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})

test('real OrdersView and OrderEntry templates load a fulfillment edit without null Teleport targets and expose full editing', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  const errors = []
  const requests = []
  let resolveForm
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(String(url), window.location.origin).pathname
    requests.push({ path, options })
    if (path === '/api/orders') return response({ rows: [order], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail') return response(formData)
    if (path === '/api/order/form') return new Promise(resolve => { resolveForm = () => resolve(response(formData)) })
    if (path === '/api/orders/117/fulfillment') return response(context)
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders, { viewParams: {}, workspaceMode: '', customerContextId: 0, customerContextLabel: '' })
  app.config.errorHandler = error => errors.push(String(error.message || error))
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    assert.ok(resolveForm, 'the actual child starts loading the selected order')
    const pendingShowsBlankEntry = [...document.querySelectorAll('h2')].some(node => node.textContent.trim() === '录单')
    resolveForm()
    await flush()
    assert.deepEqual(errors, [], 'real template updates must not patch v-model or events onto null Teleport nodes')
    assert.equal(pendingShowsBlankEntry, false, 'an existing order never renders an empty new-order form while loading')
    const delivery = [...document.querySelectorAll('.delivery-choice label')].find(label => label.textContent.includes('交付方式')).querySelector('select')
    assert.equal(delivery.disabled, true, 'a fulfillment edit cannot promise a delivery-mode change that is not submitted')
    const recipient = [...document.querySelectorAll('.fulfillment-recipient-section label')].find(label => label.querySelector('span')?.textContent === '收件人').querySelector('input')
    assert.equal(recipient.value, '张测试', 'the exact stored recipient is hydrated into the shared input')
    assert.match(document.querySelector('.quote-source-summary').textContent, /原购货库存\/权益/)
    assert.ok(button('新增明细'), 'the same product editor supports adding items')
    assert.ok(button('保存订单'), 'full-order save is available')
    assert.equal([...document.querySelectorAll('.order-detail-drawer button')].filter(node => node.textContent.trim() === '保存订单').length, 1, 'the drawer has one save action at the bottom')
    assert.equal(document.querySelector('#order-workflow-117 button'), null, 'payment and delivery do not contain an extra save action')
    assert.equal(document.querySelectorAll('.order-detail-drawer .fulfillment-fees').length, 1, 'fees appear only in summary area A')
    assert.match(document.body.textContent, /已购商品/)
    assert.match(document.querySelector('.fulfillment-supply').textContent, /本单原占用 4/)
    assert.match(document.querySelector('.fulfillment-supply').textContent, /可用量已计入本单原占用/)
    assert.match(document.body.textContent, /代发服务费/)
    assert.match(document.body.textContent, /预估运费/)
    assert.equal(requests.some(request => request.path === '/api/order/catalog'), false, 'stock editing never consults ordinary sales price tables')
    const spec = document.querySelector('.line-item .spec-control select')
    spec.value = ''
    spec.dispatchEvent(new Event('change', { bubbles: true }))
    await flush()
    assert.equal(button('保存订单', document.querySelector('.footer-panel')).disabled, true, 'clearing the selected spec must block rather than silently submit the old specification')
    assert.match(document.body.textContent, /请重新选择商品规格/)
    assert.equal(document.querySelector('.line-item input[type="number"][step="1"]').value, '4', 'invalid selection keeps the entered quantity')
    spec.value = '19'
    spec.dispatchEvent(new Event('change', { bubbles: true }))
    await flush()
    assert.equal(button('保存订单', document.querySelector('.footer-panel')).disabled, false, 'selecting an authorized spec again restores a valid edit')

    button('关闭', document.querySelector('.drawer-head')).click()
    await flush()
    assert.equal(document.querySelector('.order-detail-drawer'), null)
  } finally {
    try { app.unmount() } catch {}
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})

test('on-demand full editor shows its authorized fixed price and submits quantity through the fulfillment save path', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  const errors = [], writes = []
  const demand = structuredClone(context)
  demand.fulfillment_mode = 'on_demand'
  demand.catalog.fulfillment_mode = 'on_demand'
  demand.catalog.product_families[0].specs[0].unit_price = 35
  demand.catalog.product_families[0].specs[0].price_tiers = [{ publication_id: 91, unit_price: 35 }]
  demand.items[0].unit_price = 32
  demand.items[0].publication_id = 80
  demand.freight = { actual_cost_cents: 900, billed_cents: 900, service_fee_cents: 1400, service_due_cents: 0 }
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/orders') return response({ rows: [{ ...order, fulfillment_mode: 'on_demand' }], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail' || path === '/api/order/form') return response({ ...formData, edit_data: { ...editData, ...demand } })
    if (path === '/api/orders/117/fulfillment/preview') return response({ items: demand.items, goods_total_cents: 9600, fees, preview_token: 'demand-preview', edit_revision: demand.edit_revision })
    if (path === '/api/orders/117/fulfillment' && options.method === 'PATCH') { const command = JSON.parse(options.body); writes.push(command); demand.items[0].qty = command.items[0].qty; return response({ ok: true, order_id: 117 }) }
    if (path === '/api/orders/117/fulfillment') return response(demand)
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders, { viewParams: {}, workspaceMode: '', customerContextId: 0, customerContextLabel: '' })
  app.config.errorHandler = error => errors.push(String(error.message || error))
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    const price = document.querySelector('.line-item .price-control input')
    assert.equal(price.value, '32')
    const spec = document.querySelector('.line-item .spec-control select')
    spec.dispatchEvent(new Event('change', { bubbles: true }))
    await flush()
    assert.equal(price.value, '32', 'reselecting the original spec does not reprice an existing order')
    assert.equal(price.readOnly, true)
    assert.match(document.body.textContent, /授权代发价格表固定价/)
    assert.match(document.body.textContent, /服务费已结清/)
    assert.match(document.body.textContent, /已出账 ¥9.00/)
    const quantity = document.querySelector('.line-item input[type="number"][step="1"]')
    quantity.value = '3'
    quantity.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    button('保存订单', document.querySelector('.footer-panel')).click()
    await flush()
    assert.equal(writes.length, 1)
    assert.equal(writes[0].items[0].publication_id, 80)
    assert.equal(writes[0].items[0].qty, 3)
    assert.equal('unit_price' in writes[0].items[0], false)
    assert.deepEqual(errors, [])
  } finally {
    try { app.unmount() } catch {}
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})

test('real full-edit controls add a customer entitlement spec, preserve failed edits, save once, reload and close', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  const errors = [], writes = [], previews = []
  let stored = structuredClone(context), failSave = true
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/orders') return response({ rows: [order], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail' || path === '/api/order/form') return response({ ...formData, edit_data: { ...editData, ...stored } })
    if (path === '/api/order/shipping-destination/resolve') return response({ status: 'resolved', destination: { province: '云南省', city: '昆明市', district: '盘龙区' } })
    if (path === '/api/orders/117/fulfillment/preview') {
      const command = JSON.parse(options.body)
      previews.push(command)
      return response({ items: command.items, goods_total_cents: 0, fees, preview_token: 'preview-1', edit_revision: stored.edit_revision })
    }
    if (path === '/api/orders/117/fulfillment' && options.method === 'PATCH') {
      const command = JSON.parse(options.body)
      writes.push(command)
      if (failSave) return response({ error: '订单已变化，请重新核对后保存' }, 409)
      stored = { ...stored, ...command, edit_revision: 'revision-2', items: command.items.map((item, index) => ({ ...item, item_id: item.item_id || 22, product_name: '已购商品', spec_name: item.bom_spec_id === 19 ? '227g袋装' : '454g袋装', inventory_unit: '袋', unit_price: 0 })) }
      return response({ ok: true, order_id: 117, order_no: order.order_no, items: stored.items, fees, edit_revision: stored.edit_revision })
    }
    if (path === '/api/orders/117/fulfillment') return response(stored)
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders, { viewParams: {}, workspaceMode: '', customerContextId: 0, customerContextLabel: '' })
  app.config.errorHandler = error => errors.push(String(error.message || error))
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    const address = [...document.querySelectorAll('.fulfillment-recipient-section label')].find(label => label.textContent.trim() === '地址').querySelector('input')
    address.value = '云南省昆明市盘龙区新地址'
    address.dispatchEvent(new Event('input', { bubbles: true }))
    let quantity = document.querySelector('.line-item input[type="number"][step="1"]')
    quantity.value = '3'
    quantity.dispatchEvent(new Event('input', { bubbles: true }))
    button('新增明细').click()
    await flush()
    const newLine = document.querySelectorAll('.line-item')[1]
    newLine.querySelector('input[type="search"]').dispatchEvent(new Event('focus', { bubbles: true }))
    await flush()
    newLine.querySelector('.combo-option').dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }))
    await flush()
    const spec = document.querySelectorAll('.line-item')[1].querySelector('.spec-control select')
    spec.value = '20'
    spec.dispatchEvent(new Event('change', { bubbles: true }))
    await flush()
    button('保存订单', document.querySelector('.footer-panel')).click()
    button('保存订单', document.querySelector('.footer-panel')).click()
    await flush()
    assert.equal(writes.length, 1, 'double clicking never issues two updates')
    assert.equal(writes[0].items[0].qty, 3)
    assert.deepEqual(writes[0].items.map(item => [item.item_id, item.bom_spec_id]), [[21, 19], [0, 20]])
    assert.equal(writes[0].receiver_address, address.value)
    assert.equal(writes[0].province, '云南省')
    assert.equal(writes[0].preview_token, 'preview-1')
    assert.equal('service_fee_cents' in writes[0], false, 'a visible original fee is not a manual override')
    assert.equal('unit_price' in writes[0].items[0], false, 'stock merchandise must never be repriced by the client')
    assert.match(document.body.textContent, /订单已变化/)
    assert.equal(document.querySelectorAll('.line-item')[1].querySelector('.spec-control select').value, '20')
    assert.equal(button('保存订单', document.querySelector('.footer-panel')).disabled, false)
    failSave = false
    button('保存订单', document.querySelector('.footer-panel')).click()
    await flush()
    assert.equal(writes.length, 2)
    assert.equal(previews.length, 2)
    assert.match(document.body.textContent, /代发订单已保存/)
    assert.equal(document.querySelectorAll('.line-item').length, 2)
    assert.equal(document.querySelector('.line-item input[type="number"][step="1"]').value, '3')
    assert.deepEqual(errors, [])
    button('关闭', document.querySelector('.hero-actions')).click()
    await flush()
    assert.equal(document.querySelector('.order-detail-drawer'), null, 'the embedded editor close event reaches its parent')
  } finally {
    try { app.unmount() } catch {}
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})

test('failed asynchronous edit loading remains closable and retries without rendering a blank order', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  const errors = []
  let failed = true
  globalThis.fetch = async url => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/orders') return response({ rows: [order], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail' || path === '/api/order/form') return response(formData)
    if (path === '/api/orders/117/fulfillment') return failed ? response({ error: '暂时无法读取代发订单' }, 503) : response(context)
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders, { viewParams: {}, workspaceMode: '', customerContextId: 0, customerContextLabel: '' })
  app.config.errorHandler = error => errors.push(String(error.message || error))
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    assert.match(document.body.textContent, /暂时无法读取代发订单/)
    assert.equal(document.querySelector('.line-item'), null)
    assert.ok(button('关闭', document.querySelector('[aria-label="加载订单编辑"]')))
    failed = false
    button('重新加载').click()
    await flush()
    assert.equal(document.querySelectorAll('.line-item').length, 1)
    assert.ok(button('保存订单'))
    assert.deepEqual(errors, [])
  } finally {
    try { app.unmount() } catch {}
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})


test('restoring the original address after another destination preview restores its original billing region before save', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  const errors = [], previews = [], writes = []
  const originalDestination = { province: '云南省', city: '昆明市', district: '五华区' }
  const changedDestination = { province: '北京市', city: '北京市', district: '朝阳区' }
  const stored = { ...structuredClone(context), ...originalDestination, receiver_address: '云南省昆明市五华区原地址1号' }
  stored.fees.freight.destination = originalDestination
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/orders') return response({ rows: [order], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail' || path === '/api/order/form') return response({ ...formData, edit_data: { ...editData, ...stored } })
    if (path === '/api/order/shipping-destination/resolve') return response({ status: 'resolved', destination: changedDestination })
    if (path === '/api/orders/117/fulfillment/preview') {
      previews.push(JSON.parse(options.body))
      return response({ items: stored.items, goods_total_cents: 0, fees: stored.fees, preview_token: 'restored-preview', edit_revision: stored.edit_revision })
    }
    if (path === '/api/orders/117/fulfillment' && options.method === 'PATCH') { writes.push(JSON.parse(options.body)); return response({ ok: true, order_id: 117 }) }
    if (path === '/api/orders/117/fulfillment') return response(stored)
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders, { viewParams: {}, workspaceMode: '', customerContextId: 0, customerContextLabel: '' })
  app.config.errorHandler = error => errors.push(String(error.message || error))
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    document.querySelector('.order-link').click()
    await flush()
    const address = [...document.querySelectorAll('.fulfillment-recipient-section label')].find(label => label.querySelector('span')?.textContent === '地址').querySelector('input')
    address.value = '北京市朝阳区新地址2号'
    address.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    button('重新核算代发费用').click()
    await flush()
    assert.equal(previews[0].province, '北京市')
    address.value = stored.receiver_address
    address.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    button('保存订单', document.querySelector('.footer-panel')).click()
    await flush()
    assert.equal(writes.length, 1)
    assert.equal(writes[0].receiver_address, stored.receiver_address)
    assert.deepEqual({ province: writes[0].province, city: writes[0].city, district: writes[0].district }, originalDestination, 'an address restored to Yunnan cannot be saved with the Beijing preview region')
    assert.equal(writes[0].preview_token, 'restored-preview')
    assert.deepEqual(errors, [])
  } finally {
    try { app.unmount() } catch {}
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})


test('historical discounted amounts, net order totals and rich item snapshots survive the real fulfillment detail integration', async () => {
  const { component: Orders } = await loadComponent('../views/OrdersView.vue')
  const originalFetch = globalThis.fetch
  const errors = [], writes = []
  const discountedOrder = { ...order, fulfillment_mode: 'on_demand', total_amount: 120, discount_amount: 15, rounding_amount: 0.2, grand_total: 104.8 }
  const richItem = { ...items[0], unit_price: 30, line_total: 115, line_total_before_discount: 120, discount_amount: 5, discount_type: 'amount', discount_value: 5,
    logistics_unit_weight_g: 270, logistics_item_weight_g: 1080, weight_snapshot_json: '{"qty":4}', price_source_json: '{"source":"published_bean_list","publication_id":80}' }
  const stored = { ...structuredClone(context), fulfillment_mode: 'on_demand', goods_total_cents: 10480,
    items: [{ item_id: 21, product_id: 7, bom_spec_id: 19, bom_variant_id: 1185, product_name: '已购商品 227g袋装', spec_name: '227g袋装', spec_label: '227g袋装', inventory_unit: '袋', sales_unit: '袋', qty: 4,
      unit_price: 30, line_amount: 115, base_line_amount: 120, discount_amount: 5, discount_type: 'amount', discount_value: 5, publication_id: 80 }] }
  stored.catalog.fulfillment_mode = 'on_demand'
  stored.catalog.product_families[0].specs[0].unit_price = 35
  stored.catalog.product_families[0].specs[0].price_tiers = [{ publication_id: 91, unit_price: 35 }]
  globalThis.fetch = async (url, options = {}) => {
    const path = new URL(String(url), window.location.origin).pathname
    if (path === '/api/orders') return response({ rows: [discountedOrder], summary: {}, page: 1, limit: 20 })
    if (path === '/api/orders/117/detail' || path === '/api/order/form') return response({ ...formData, edit_data: { ...editData, ...discountedOrder, items: [richItem] } })
    if (path === '/api/orders/117/fulfillment/preview') {
      const command = JSON.parse(options.body)
      return command.items[0].qty === 4
        ? response({ items: stored.items, goods_total_cents: 10480, fees: stored.fees, preview_token: 'discount-preview', edit_revision: stored.edit_revision })
        : response({ error: '该商品包含历史行折扣，修改数量前请先通过财务流程核对折扣' }, 400)
    }
    if (path === '/api/orders/117/fulfillment' && options.method === 'PATCH') { writes.push(JSON.parse(options.body)); return response({ ok: true, order_id: 117 }) }
    if (path === '/api/orders/117/fulfillment') return response(stored)
    return response({ rows: [], profiles: [], groups: [], assignments: [], regions: [] })
  }
  const app = createApp(Orders, { viewParams: {}, workspaceMode: '', customerContextId: 0, customerContextLabel: '' })
  app.config.errorHandler = error => errors.push(String(error.message || error))
  app.config.warnHandler = () => {}
  try {
    app.mount(document.querySelector('#app'))
    await flush()
    assert.match(document.body.textContent, /商品货款.*104.80/)
    document.querySelector('.order-link').click()
    await flush()
    const detail = document.querySelector('.detail-item')
    assert.match(detail.textContent, /227g袋装 × 4袋/)
    assert.match(detail.textContent, /物流重量：单件 270g · 本项 1080g/)
    assert.match(document.querySelector('.order-summary-row .fulfillment-fees').textContent, /商品货款¥104.80/)
    assert.equal(document.querySelector('.line-total strong').textContent, '115.00')
    assert.equal(document.querySelector('.footer-panel .fulfillment-fees'), null, 'the editor does not repeat area A')
    assert.equal(document.querySelector('.total-pill strong').textContent, '118.80', 'historical global discount and rounding remain in the net goods total')
    assert.equal(document.querySelector('.line-item .discount-control'), null, 'historical discount is read-only')
    const spec = document.querySelector('.line-item .spec-control select')
    spec.dispatchEvent(new Event('change', { bubbles: true }))
    await flush()
    assert.equal(document.querySelector('.line-total strong').textContent, '115.00', 'reselecting the original spec preserves its discounted net subtotal')
    const quantity = document.querySelector('.line-item input[type="number"][step="1"]')
    quantity.value = '5'
    quantity.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    assert.match(document.querySelector('.line-item').textContent, /历史行折扣.*财务流程核对折扣/)
    button('保存订单', document.querySelector('.footer-panel')).click()
    await flush()
    assert.equal(writes.length, 0)
    assert.equal(quantity.value, '5', 'a finance review rejection preserves the changed quantity')
    assert.match(document.body.textContent, /该商品包含历史行折扣/)
    quantity.value = '4'
    quantity.dispatchEvent(new Event('input', { bubbles: true }))
    await flush()
    button('保存订单', document.querySelector('.footer-panel')).click()
    await flush()
    assert.equal(writes.length, 1)
    for (const field of ['unit_price', 'discount_type', 'discount_value', 'discount_amount']) assert.equal(field in writes[0].items[0], false)
    assert.equal(document.querySelector('.line-total strong').textContent, '115.00')
    assert.deepEqual(errors, [])
  } finally {
    try { app.unmount() } catch {}
    globalThis.fetch = originalFetch
    document.body.innerHTML = '<div id="app"></div>'
  }
})
