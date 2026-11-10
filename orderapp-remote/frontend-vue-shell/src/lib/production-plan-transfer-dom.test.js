import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { JSDOM } from 'jsdom'
import { parse, compileScript } from '@vue/compiler-sfc'

const dom = new JSDOM('<!doctype html><html><body><div id="app"></div></body></html>', { url: 'http://localhost/app/vue-shell' })
for (const name of ['window', 'document', 'Node', 'Element', 'HTMLElement', 'SVGElement', 'MutationObserver', 'Event']) globalThis[name] = dom.window[name]
const Vue = await import('vue')
const file = new URL('../components/ProductionPlanTransferDrawer.vue', import.meta.url)
async function component() {
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'), { filename: file.pathname })
  const compiled = compileScript(descriptor, { id: 'plan-transfer-dom', inlineTemplate: true })
  const bindings = {}
  for (const [, imports] of compiled.content.matchAll(/import\s+\{([^}]+)\}\s+from\s+"vue"/g)) for (const part of imports.split(',')) { const [imported, local] = part.trim().split(/\s+as\s+/); bindings[local || imported] = Vue[imported] }
  for (const binding of Object.values(compiled.imports)) {
    const mod = binding.source === 'vue' ? Vue : await import(new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, file))
    bindings[binding.local] = mod[binding.imported]
  }
  return new Function(...Object.keys(bindings), compiled.content.replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\s*$/gm, '').replace('export default', 'return'))(...Object.values(bindings))
}
async function settle() { for (let i = 0; i < 5; i++) { await new Promise(resolve => setImmediate(resolve)); await Vue.nextTick() } }
const button = (label) => [...document.querySelectorAll('button')].find((el) => el.textContent.trim() === label)

test('bulk editing loads warehouse batches and preserves changed input after preview failure', async () => {
  const source = { id: 5, component_name: '生豆', unit: 'kg', demand_g: 6767, wip_covered_g: 0 }
  const item = { source_id: 5, warehouse: 'raw', owner_customer_id: 0, batches: [{ batch_code: 'B1', qty_g: 3000, qty_units: 0 }] }
  const calls = []
  globalThis.fetch = async (url, options = {}) => {
    const path = String(url); calls.push({ path, options })
    if (path.includes('material-transfer-options')) return new Response(JSON.stringify({ source, pending_documents: [], options: [{ batch_id: 1, batch_code: 'B1', owner_customer_id: 0, qty_g: 10000, available_g: 10000 }] }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.includes('/preview')) {
      const body = JSON.parse(options.body)
      if (body.items.length) return new Response(JSON.stringify({ error: '库存已变化，请重新核对' }), { status: 409, headers: { 'content-type': 'application/json' } })
      return new Response(JSON.stringify({ groups: [{ items: [item] }], unavailable: [], document_count: 1 }), { status: 200, headers: { 'content-type': 'application/json' } })
    }
    throw new Error(`unexpected request ${path}`)
  }
  const Component = await component()
  const app = Vue.createApp(Component, { mode: 'bulk', detail: { id: 26, revision: 3, component_sources: [source] }, warehouses: [{ code: 'raw', name: '原料仓' }] })
  app.mount('#app')
  try {
    await settle()
    assert.ok(button('调整来源'))
    const toggle = document.querySelector('input[aria-label="选择生豆"]')
    assert.ok(toggle)
    toggle.checked = false; toggle.dispatchEvent(new Event('change', { bubbles: true })); await Vue.nextTick()
    assert.equal(button('生成 0 张调拨单').disabled, true)
    toggle.checked = true; toggle.dispatchEvent(new Event('change', { bubbles: true })); await Vue.nextTick()
    button('调整来源').click()
    await settle()
    const input = document.querySelector('input[aria-label="B1调拨数量"]')
    assert.equal(input.value, '3')
    input.value = '6'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await Vue.nextTick()
    button('重新核对').click()
    await settle()
    assert.match(document.body.textContent, /库存已变化，请重新核对/)
    assert.equal(document.querySelector('input[aria-label="B1调拨数量"]').value, '6')
    assert.equal(button('生成 1 张调拨单').disabled, true)
    const check = calls.findLast((row) => row.path.includes('/preview'))
    assert.equal(JSON.parse(check.options.body).items[0].batches[0].qty_g, 6000)
    assert.equal(calls.filter((row) => row.path.endsWith('/material-transfers')).length, 0)
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('single retry retains input and idempotency key after an unknown creation outcome', async () => {
  const source = { id: 5, component_name: '生豆', unit: 'kg', demand_g: 6767, wip_covered_g: 1000 }
  const requests = []
  let attempts = 0
  globalThis.fetch = async (url, options = {}) => {
    const path = String(url)
    if (path.includes('material-transfer-options')) return new Response(JSON.stringify({ source, pending_documents: [], options: [{ batch_id: 1, batch_code: 'B1', owner_customer_id: 0, qty_g: 10000, available_g: 10000 }] }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.includes('/preview')) return new Response(JSON.stringify({ groups: [{ items: JSON.parse(options.body).items }], unavailable: [] }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.endsWith('/material-transfers')) {
      requests.push(JSON.parse(options.body)); attempts += 1
      if (attempts === 1) throw new Error('网络中断')
      return new Response(JSON.stringify({ documents: [{ id: 11 }] }), { status: 200, headers: { 'content-type': 'application/json' } })
    }
    throw new Error(`unexpected request ${path}`)
  }
  const Component = await component()
  let updated = 0
  const app = Vue.createApp(Component, { mode: 'single', source, detail: { id: 26, revision: 3, draft_token: 'current-draft-token' }, warehouses: [{ code: 'raw', name: '原料仓' }], onUpdated: () => { updated++ } })
  app.mount('#app')
  try {
    await settle()
    const input = document.querySelector('input[aria-label="B1调拨数量"]')
    input.value = '4'; input.dispatchEvent(new Event('input', { bubbles: true })); await Vue.nextTick()
    button('生成调拨单').click(); await settle()
    assert.match(document.body.textContent, /网络中断/)
    assert.equal(document.querySelector('input[aria-label="B1调拨数量"]').value, '4')
    button('生成调拨单').click(); await settle()
    assert.equal(requests.length, 2)
    assert.equal(requests[0].idempotency_key, requests[1].idempotency_key)
    assert.equal(requests[0].draft_token, 'current-draft-token')
    assert.equal(requests[0].items[0].batches[0].qty_g, 4000)
    assert.equal(updated, 1)
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('single transfer does not create a document when preview reports no eligible batch', async () => {
  const source = { id: 5, component_name: '生豆', unit: 'kg', demand_g: 5000, wip_covered_g: 0 }
  let creates = 0
  globalThis.fetch = async (url) => {
    const path = String(url)
    if (path.includes('material-transfer-options')) return new Response(JSON.stringify({ source, pending_documents: [], options: [{ batch_id: 1, batch_code: 'B1', owner_customer_id: 0, qty_g: 10000, available_g: 10000 }] }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.includes('/preview')) return new Response(JSON.stringify({ groups: [], unavailable: [{ source_id: 5, reason: '批次已冻结' }] }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.endsWith('/material-transfers')) { creates++; return new Response('{}', { status: 200, headers: { 'content-type': 'application/json' } }) }
    throw new Error(`unexpected request ${path}`)
  }
  const Component = await component()
  const app = Vue.createApp(Component, { mode: 'single', source, detail: { id: 26, draft_token: 'draft' }, warehouses: [{ code: 'raw', name: '原料仓' }] })
  app.mount('#app')
  try {
    await settle()
    const input = document.querySelector('input[aria-label="B1调拨数量"]')
    input.value = '4'; input.dispatchEvent(new Event('input', { bubbles: true })); await Vue.nextTick()
    button('生成调拨单').click(); await settle()
    assert.match(document.body.textContent, /批次已冻结/)
    assert.equal(input.value, '4')
    assert.equal(creates, 0)
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('stock lookup starts with authorized warehouses rather than arbitrary global warehouse', async () => {
  const source = { id: 5, component_name: '生豆', unit: 'kg', demand_g: 5000, wip_covered_g: 0 }
  const calls = []
  globalThis.fetch = async (url) => {
    calls.push(String(url))
    return new Response(JSON.stringify({ source, warehouses: [{ warehouse: 'wip', warehouse_name: 'WIP在制仓' }, { warehouse: 'raw', warehouse_name: '原料仓' }], pending_documents: [], options: [{ warehouse: 'wip', batch_id: 1, batch_code: 'B1', qty_g: 3000, reserved_g: 1000, available_g: 2000, quality_status: '合格' }] }), { status: 200, headers: { 'content-type': 'application/json' } })
  }
  const Component = await component()
  const app = Vue.createApp(Component, { mode: 'stock', source, detail: { id: 26, draft_token: 'draft' }, warehouses: [{ code: 'not_authorized', name: '无权仓库' }] })
  app.mount('#app')
  try {
    await settle()
    assert.match(calls[0], /source_id=5/)
    assert.doesNotMatch(calls[0], /warehouse=not_authorized/)
    assert.match(document.body.textContent, /B1/)
    assert.match(document.body.textContent, /WIP在制仓/)
    assert.doesNotMatch(document.body.textContent, /无权仓库/)
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('source adjustment targets the plan source, not the detached lookup response', async () => {
  const source = { id: 5, component_name: '生豆', unit: 'kg', demand_g: 5000, required_g: 5000, wip_covered_g: 0, options: [{ warehouse: 'raw', warehouse_name: '原料仓', owner_customer_id: 0, available_g: 5000 }] }
  globalThis.fetch = async () => new Response(JSON.stringify({ source: { ...source }, warehouses: source.options, pending_documents: [], options: [] }), { status: 200, headers: { 'content-type': 'application/json' } })
  const Component = await component()
  let adjusted
  const app = Vue.createApp(Component, { mode: 'stock', source, detail: { id: 26, draft_token: 'draft' }, onAdjust: (row, allocations) => { adjusted = { row, allocations } } })
  app.mount('#app')
  try {
    await settle()
    const input = document.querySelector('.adjust-row input')
    input.value = '2'; input.dispatchEvent(new Event('input', { bubbles: true })); await Vue.nextTick()
    button('保存来源建议').click(); await settle()
    assert.equal(adjusted.row, source)
    assert.equal(adjusted.allocations[0].qty_g, 2000)
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('source adjustment shows save failure inside drawer and preserves the input', async () => {
  const source = { id: 5, component_name: '生豆', unit: 'kg', demand_g: 5000, required_g: 5000, options: [{ warehouse: 'raw', warehouse_name: '原料仓', owner_customer_id: 0, available_g: 5000 }] }
  globalThis.fetch = async () => new Response(JSON.stringify({ source: { ...source }, warehouses: source.options, pending_documents: [], options: [] }), { status: 200, headers: { 'content-type': 'application/json' } })
  const Component = await component()
  const app = Vue.createApp(Component, { mode: 'stock', source, detail: { id: 26, draft_token: 'draft' }, onAdjust: (row, allocations, done) => done('保存失败，请重试') })
  app.mount('#app')
  try {
    await settle()
    const input = document.querySelector('.adjust-row input')
    input.value = '2'; input.dispatchEvent(new Event('input', { bubbles: true })); await Vue.nextTick()
    button('保存来源建议').click(); await settle()
    assert.match(document.body.textContent, /保存失败，请重试/)
    assert.equal(input.value, '2')
    assert.equal(button('保存来源建议').disabled, false)
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('single drawer distinguishes WIP shortage from actual transferable need when upstream is planned', async () => {
  const source = { id: 5, component_name: '熟豆', unit: 'kg', demand_g: 43584, required_g: 15000, upstream_g: 28584, wip_covered_g: 0 }
  globalThis.fetch = async () => new Response(JSON.stringify({ source, warehouses: [{ warehouse: 'raw', warehouse_name: '原料仓' }], pending_documents: [{ id: 10, source_id: 5, status: 'draft', qty_g: 1000 }], options: [] }), { status: 200, headers: { 'content-type': 'application/json' } })
  const Component = await component()
  const app = Vue.createApp(Component, { mode: 'single', source, detail: { id: 26, draft_token: 'draft' } })
  app.mount('#app')
  try {
    await settle()
    assert.match(document.body.textContent, /43\.584 kg/)
    assert.match(document.body.textContent, /本次最多生成：14 kg/)
    assert.match(document.body.textContent, /本计划上游供应 28\.584 kg/)
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('bulk shows real preview groups and pending rows; choosing a subset submits only selected groups', async () => {
  const sources = [{ id: 5, component_name: '生豆', unit: 'kg', demand_g: 6000 }, { id: 6, component_name: '袋子', unit: '件', demand_units: 10 }, { id: 7, component_name: '缺口物料', unit: 'kg', demand_g: 1000 }]
  const items = [
    { source_id: 5, warehouse: 'raw', owner_customer_id: 0, batches: [{ batch_code: 'B1', qty_g: 6000, qty_units: 0 }] },
    { source_id: 6, warehouse: 'packaging', owner_customer_id: 0, batches: [{ batch_code: 'B2', qty_g: 0, qty_units: 10 }] },
  ]
  let created
  globalThis.fetch = async (url, options = {}) => {
    const path = String(url)
    if (path.includes('/preview')) return new Response(JSON.stringify({ groups: [{ warehouse: 'raw', owner_customer_id: 0, items: [items[0]] }, { warehouse: 'packaging', owner_customer_id: 0, items: [items[1]] }], unavailable: [{ source_id: 7, reason: '没有可用库存' }], pending_documents: [{ id: 9, entry_no: 'SE-9', status: 'draft', source_id: 5, qty_g: 1000 }], document_count: 2 }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.endsWith('/material-transfers')) { created = JSON.parse(options.body); return new Response(JSON.stringify({ documents: [{ id: 11 }] }), { status: 200, headers: { 'content-type': 'application/json' } }) }
    throw new Error(`unexpected request ${path}`)
  }
  const Component = await component()
  const app = Vue.createApp(Component, { mode: 'bulk', detail: { id: 26, draft_token: 'plan-token', component_sources: sources } })
  app.mount('#app')
  try {
    await settle()
    assert.ok(button('生成 2 张调拨单'))
    assert.match(document.body.textContent, /缺口物料/)
    assert.match(document.body.textContent, /没有可用库存/)
    assert.match(document.body.textContent, /SE-9 · 待仓库确认/)
    const toggle = document.querySelector('input[aria-label="选择袋子"]')
    toggle.checked = false; toggle.dispatchEvent(new Event('change', { bubbles: true })); await Vue.nextTick()
    assert.ok(button('生成 1 张调拨单'))
    button('生成 1 张调拨单').click(); await settle()
    assert.equal(created.draft_token, 'plan-token')
    assert.deepEqual(created.items, [items[0]])
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})

test('product batches with mixed gram and piece snapshots are entered in pieces', async () => {
  const source = { id: 15, component_name: '盒装成品', component_type: 'product', component_spec_g: 454, unit: 'kg', demand_g: 4540, demand_units: 10, required_g: 4540, required_units: 10, wip_covered_g: 908, wip_covered_units: 2 }
  const options = [{ batch_id: 1, batch_code: 'PIECE', warehouse: 'raw', owner_customer_id: 0, qty_g: 50000, qty_units: 6, available_g: 50000, available_units: 6 }, { batch_id: 2, batch_code: 'LOOSE', warehouse: 'raw', owner_customer_id: 0, qty_g: 10000, qty_units: 0, available_g: 10000, available_units: 0 }]
  let created
  globalThis.fetch = async (url, config = {}) => {
    const path = String(url)
    if (path.includes('material-transfer-options')) return new Response(JSON.stringify({ source, warehouses: [{ warehouse: 'raw', warehouse_name: '原料仓' }], pending_documents: [{ id: 10, source_id: 15, status: 'draft', qty_g: 0, qty_units: 1 }], options }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.includes('/preview')) return new Response(JSON.stringify({ groups: [{ items: JSON.parse(config.body).items }], unavailable: [] }), { status: 200, headers: { 'content-type': 'application/json' } })
    if (path.endsWith('/material-transfers')) { created = JSON.parse(config.body); return new Response(JSON.stringify({ documents: [] }), { status: 200, headers: { 'content-type': 'application/json' } }) }
    throw new Error(`unexpected request ${path}`)
  }
  const Component = await component()
  const app = Vue.createApp(Component, { mode: 'single', source, detail: { id: 26, draft_token: 'draft' } })
  app.mount('#app')
  try {
    await settle()
    assert.match(document.body.textContent, /本次最多生成：7 件/)
    assert.match(document.body.textContent, /该批次只有散装库存，不能按件调拨/)
    assert.equal(document.querySelector('input[aria-label="LOOSE调拨数量"]').disabled, true)
    const input = document.querySelector('input[aria-label="PIECE调拨数量"]')
    assert.equal(input.step, '1')
    input.value = '4'; input.dispatchEvent(new Event('input', { bubbles: true })); await Vue.nextTick()
    button('生成调拨单').click(); await settle()
    assert.deepEqual(created.items[0].batches, [{ batch_code: 'PIECE', qty_g: 0, qty_units: 4 }])
  } finally { app.unmount(); document.body.innerHTML = '<div id="app"></div>' }
})
