import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript, compileTemplate } from '@vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'

async function view(name) {
  const file = new URL(`../components/${name}.vue`, import.meta.url)
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
  const script = compileScript(descriptor, { id: name })
  let code = script.content.replace(/import (\w+) from ['"][^'"]+\.vue['"]/g, (_, name) => `const ${name}={render:()=>null}`)
  code = code.replace('export default', 'const component =')
  const template = compileTemplate({ id: name, source: descriptor.template.content, filename: file.pathname, compilerOptions: { bindingMetadata: script.bindings } })
  assert.deepEqual(template.errors, [])
  code += '\n' + template.code.replace('export function render', 'function render') + '\ncomponent.render=render;export default component;'
  code = code.replace(/from (['"])([^'"]+)\1/g, (_, quote, path) => `from ${JSON.stringify(path.startsWith('.') ? new URL(/\.[a-z]+$/.test(path) ? path : `${path}.js`, file).href : import.meta.resolve(path))}`)
  return { ...(await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default }
}
async function editor(name, props = {}, emit = () => {}) {
  const View = await view(name)
  let vm
  await renderToString(createSSRApp({ setup() { vm = View.setup(props, { expose() {}, emit }); return () => h('div') } }))
  return vm
}
const response = value => ({ ok: true, status: 200, json: async () => value })

test('either enabled dropship scene locks automatic product ordering on', async () => {
  for (const mode of ['stock', 'on_demand', null]) {
    const View = await view('CustomerFulfillmentSettings')
    const setup = View.setup
    View.setup = (props, ctx) => {
      const vm = setup(props, ctx)
      vm.loaded.value = true
      for (const row of vm.form.value.modes) row.enabled = row.mode === mode
      return vm
    }
    const html = await renderToString(createSSRApp(View, { customerId: 147 }))
    if (mode) {
      assert.match(html, /商品下单已自动开启/)
      assert.match(html, /<input[^>]*type="checkbox"[^>]*checked[^>]*disabled/)
      assert.match(html, /不能单独关闭/)
    } else assert.doesNotMatch(html, /商品下单已自动开启/)
  }
})

test('successful fulfillment save notifies capability summary without resetting outer portal draft', async () => {
  const original = globalThis.fetch
  const events = []
  globalThis.fetch = async () => response({ customer_id: 147, revision: 2, modes: [{ mode: 'on_demand', enabled: true, fee_configured: true, fixed_fee_cents: 0, per_package_fee_cents: 0, shipping_free: true }] })
  try {
    const vm = await editor('CustomerFulfillmentSettings', { customerId: 147 }, (...args) => events.push(args))
    Object.assign(vm.form.value.modes[1], { enabled: true, shipping_free: true, fixed_fee: '0', per_package_fee: '0', surcharge: '0' })
    await vm.save()
    assert.equal(events[0]?.[0], 'settings-changed')
    assert.equal(events[0][1].customerId, 147)
    assert.equal(events[0][1].modes.find(row => row.mode === 'on_demand').enabled, true)
  } finally { globalThis.fetch = original }
})

test('enabled dropship card alone renders freight, warehouse and fees without ordinary shipping editor', async () => {
  const View = await view('CustomerFulfillmentSettings')
  const setup = View.setup
  View.setup = (props, ctx) => {
    const vm = setup(props, ctx)
    vm.loaded.value = true
    vm.form.value.modes[0].enabled = true
    return vm
  }
  const html = await renderToString(createSSRApp(View, { customerId: 147 }))
  assert.match(html, /打开仓库/)
  assert.match(html, /默认运费模板/)
  assert.equal((html.match(/每票固定附加运费/g) || []).length, 1)
  assert.doesNotMatch(html, /普通发货录单/)
})

test('disabled scenes hide configuration while on-demand alone never asks for a warehouse', async () => {
  for (const onDemand of [false, true]) {
    const View = await view('CustomerFulfillmentSettings')
    const setup = View.setup
    View.setup = (props, ctx) => { const vm = setup(props, ctx); vm.loaded.value = true; vm.form.value.modes[1].enabled = onDemand; return vm }
    const html = await renderToString(createSSRApp(View, { customerId: 147 }))
    assert.doesNotMatch(html, /打开仓库|客户成品仓/)
    assert.equal((html.match(/默认运费模板/g) || []).length, onDemand ? 1 : 0)
    assert.equal((html.match(/每单固定服务费/g) || []).length, onDemand ? 1 : 0)
  }
})

test('warehouse refresh selects new own finished warehouse and preserves the entire unsaved settings draft', async () => {
  const original = globalThis.fetch
  const events = []
  globalThis.fetch = async () => response({ rows: [{ code: 'new', kind: 'finished', active: true, is_virtual: true, customer_id: 147 }] })
  try {
    const vm = await editor('CustomerFulfillmentSettings', { customerId: 147 }, (...args) => events.push(args))
    vm.form.value.modes[0].enabled = true
    vm.form.value.modes[0].fixed_fee = '3.25'
    vm.form.value.modes[0].surcharge = '1.25'
    const draft = vm.form.value
    await vm.handleWarehouseChanged({ action: 'create', warehouse: { code: 'new', customer_id: 147, kind: 'finished' } })
    assert.equal(vm.form.value, draft)
    assert.equal(vm.form.value.modes[0].warehouse_code, 'new')
    assert.equal(vm.form.value.modes[0].fixed_fee, '3.25')
    assert.equal(vm.form.value.modes[0].surcharge, '1.25')
    assert.equal(events[0][0], 'warehouses-changed')
  } finally { globalThis.fetch = original }
})

test('one save submits all enabled mode policies once and preserves draft on conflict', async () => {
  const original = globalThis.fetch
  const calls = []
  globalThis.fetch = async (url, options) => {
    calls.push({ url, body: JSON.parse(options.body) })
    return { ok: false, status: 409, json: async () => ({ error: '履约设置已变化，请重新核对' }) }
  }
  try {
    const vm = await editor('CustomerFulfillmentSettings', { customerId: 147 })
    const mode = vm.form.value.modes[1]
    Object.assign(mode, { enabled: true, shipping_free: true, fixed_fee: '0', per_package_fee: '0', surcharge: '0' })
    await vm.save()
    assert.equal(calls.length, 1)
    assert.match(calls[0].url, /147\/settings$/)
    assert.equal(calls[0].body.modes[1].shipping_policy.free_shipping, true)
    assert.equal(vm.form.value.modes[1].enabled, true)
    assert.match(vm.error.value, /重新核对/)
  } finally { globalThis.fetch = original }
})

test('ordinary shipping editor only saves sale policy for selected ordinary customer', async () => {
  const original = globalThis.fetch
  const calls = []
  globalThis.fetch = async (url, options) => {
    calls.push({ url, method: options.method || 'GET', body: options.body && JSON.parse(options.body) })
    if (options.method === 'PUT') return response({ scene: 'sale', template_id: 8, surcharge_cents: 125, free_shipping: false, revision: 2 })
    if (url.includes('shipping-policies')) return response({ policies: [{ scene: 'sale', template_id: 8, surcharge_cents: 125, free_shipping: false, revision: 1 }, { scene: 'stock', template_id: 9 }] })
    return response({ templates: [] })
  }
  try {
    const vm = await editor('CustomerShippingPolicies', { customerId: 501 })
    await vm.load()
    assert.equal(vm.policy.value.scene, 'sale')
    await vm.save()
    const writes = calls.filter(call => call.method === 'PUT')
    assert.equal(writes.length, 1)
    assert.match(writes[0].url, /501\/shipping-policies\/sale$/)
    assert.equal(writes[0].body.surcharge_cents, 125)
  } finally { globalThis.fetch = original }
})

test('ordinary shipping customer switch rejects a late response from the previous customer', async () => {
  const original = globalThis.fetch
  const pending = new Map()
  globalThis.fetch = async url => {
    if (!url.includes('shipping-policies')) return response({ templates: [] })
    return new Promise(resolve => pending.set(Number(url.match(/fulfillment\/(\d+)/)[1]), resolve))
  }
  try {
    const props = { customerId: 501 }
    const vm = await editor('CustomerShippingPolicies', props)
    const first = vm.load()
    await Promise.resolve()
    props.customerId = 502
    const second = vm.load()
    await Promise.resolve()
    pending.get(502)(response({ policies: [{ scene: 'sale', template_id: 22, surcharge_cents: 100, revision: 2 }] }))
    await second
    pending.get(501)(response({ policies: [{ scene: 'sale', template_id: 11, surcharge_cents: 0, revision: 1 }] }))
    await first
    assert.equal(props.customerId, 502)
    assert.equal(vm.policy.value.template_id, 22)
  } finally { globalThis.fetch = original }
})

test('virtual warehouse drawer uses immutable code, revision and paginated current-customer list', async () => {
  const original = globalThis.fetch
  const calls = [], events = []
  globalThis.fetch = async (url, options) => {
    calls.push({ url, method: options.method || 'GET', body: options.body && JSON.parse(options.body) })
    if (options.method === 'PUT') return response({ code: 'v1', name: '客户成品仓', kind: 'finished', customer_id: 147, is_virtual: true, revision: 4 })
    return response({ rows: [], total: 0, total_pages: 1, page: 2, limit: 10 })
  }
  try {
    const vm = await editor('VirtualWarehouseDrawer', { customerId: 147 }, (...args) => events.push(args))
    await vm.loadPage(2)
    assert.match(calls[0].url, /customer_id=147/)
    assert.match(calls[0].url, /page=2/)
    vm.edit({ code: 'v1', name: '客户成品仓', kind: 'finished', customer_id: 147, revision: 3 })
    await vm.save()
    const write = calls.find(call => call.method === 'PUT')
    assert.match(write.url, /virtual-warehouses\/v1$/)
    assert.equal(write.body.expected_revision, 3)
    assert.equal(write.body.code, undefined)
    assert.equal(events[0][0], 'changed')
    assert.equal(events[0][1].warehouse.code, 'v1')
  } finally { globalThis.fetch = original }
})

test('protected warehouse delete keeps the row and reports the reason; success uses its revision', async () => {
  const original = globalThis.fetch
  const calls = [], events = []
  let blocked = true
  globalThis.fetch = async (url, options) => {
    calls.push({ url, method: options.method || 'GET' })
    if (options.method === 'DELETE' && blocked) return { ok: false, status: 400, json: async () => ({ error: '仓库仍有库存，无法删除' }) }
    return response({ rows: [], total: 0, page: 1 })
  }
  try {
    const vm = await editor('VirtualWarehouseDrawer', { customerId: 147 }, (...args) => events.push(args))
    const warehouse = { code: 'v1', name: '成品仓', revision: 4 }
    vm.rows.value = [warehouse]; vm.pendingDeletion.value = warehouse
    await vm.remove(warehouse)
    assert.match(vm.error.value, /仍有库存/)
    assert.equal(vm.rows.value.length, 1)
    assert.equal(events.length, 0)
    assert.equal(vm.pendingDeletion.value.code, 'v1')
    blocked = false
    await vm.remove(warehouse)
    assert.match(calls[0].url, /v1\?expected_revision=4$/)
    assert.equal(events[0][1].action, 'delete')
    assert.equal(vm.rows.value.length, 0)
  } finally { globalThis.fetch = original }
})


test('system default editor loads and saves without choosing any customer', async () => {
 const original = globalThis.fetch, calls = []
 globalThis.fetch = async (url, options) => {
  calls.push({url, method:options.method || 'GET', body:options.body && JSON.parse(options.body)})
  return response(url.includes('shipping-templates') ? {templates:[]} : {scene:'sale',template_id:8,revision:2,surcharge_cents:0,free_shipping:false})
 }
 try {
  const vm = await editor('CustomerShippingPolicies')
  await vm.load(); await vm.save()
  const writes = calls.filter(c=>c.method==='PUT')
  assert.equal(writes.length,1)
  assert.match(writes[0].url,/settings\/logistics\/default-shipping-policy$/)
  const View = await view('CustomerShippingPolicies')
  const html = await renderToString(createSSRApp(View))
  assert.match(html,/所有录单的默认/)
  assert.doesNotMatch(html,/搜索客户|请先选择/)
 } finally { globalThis.fetch = original }
})
