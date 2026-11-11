import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import * as Vue from 'vue'

const url = new URL('../components/ProductionPlanSemiFinishedStock.vue', import.meta.url)
const fixture = { draft_token: 'v1', items: [{ key: 'material:1', name: '初晓', unit: 'kg', required_qty: 60.112, stock_covered_qty: 0, batches: [{ batch_id: 3, warehouse: 'raw_materials', available_qty: 15, selected_qty: 0 }] }] }
async function setup(send, get = async () => structuredClone(fixture)) {
  const calls = [], events = []
  const client = { apiGet: async (...args) => { calls.push(['get', ...args]); return get(...args) }, apiSend: async (...args) => { calls.push(['send', ...args]); return send(...args) } }
  const { descriptor } = parse(fs.readFileSync(url, 'utf8'))
  const compiled = compileScript(descriptor, { id: 'stock-component' })
  const bindings = {}, modules = new Map()
  for (const [name, binding] of Object.entries(compiled.imports)) {
    if (!modules.has(binding.source)) modules.set(binding.source, binding.source === 'vue' ? Vue : binding.source === '../api/client' ? client : await import(new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, url)))
    bindings[name] = modules.get(binding.source)[binding.imported]
  }
  const code = compiled.content.replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\n/gm, '').replace('export default', 'return')
  const component = new Function(...Object.keys(bindings), code)(...Object.values(bindings))
  const scope = Vue.effectScope()
  const state = scope.run(() => component.setup({ detail: { id: 26, draft_token: 'v1' }, disabled: false }, { expose() {}, emit: (...args) => events.push(args) }))
  await state.load()
  return { state, calls, events, close: () => scope.stop() }
}

test('preview is read only and confirmation sends same explicit batch and updates plan', async () => {
  const app = await setup(async (_, options) => options.body.check_only ? { preview_token: 'preview-v1', changes: [{ before_qty: 60.112, after_qty: 45.112, operations: [] }] } : { plan: { id: 26, draft_token: 'v2' } })
  try {
    await app.state.open()
    app.state.values.value['material:1|raw_materials|3'] = 15
    await app.state.requestPreview()
    assert.equal(app.calls.filter(c => c[0] === 'send')[0][2].body.check_only, true)
    assert.equal(app.state.preview.value.changes[0].after_qty, 45.112)
    await app.state.commit()
    const sends = app.calls.filter(c => c[0] === 'send')
    assert.equal(sends[1][2].body.check_only, false)
    assert.equal(sends[1][2].body.preview_token, 'preview-v1')
    assert.deepEqual(sends[1][2].body.selections, [{ key: 'material:1', batch_id: 3, warehouse: 'raw_materials', qty: 15 }])
    assert.deepEqual(app.events.find(e => e[0] === 'updated'), ['updated', { id: 26, draft_token: 'v2' }])
  } finally { app.close() }
})

test('incomplete impact preview cannot be confirmed', async () => {
  const app = await setup(async () => ({ changes: [{ before_qty: 60.112, after_qty: 45.112 }] }))
  try {
    await app.state.open()
    app.state.values.value['material:1|raw_materials|3'] = 15
    await app.state.requestPreview()
    assert.equal(app.state.previewComplete.value, false)
    await app.state.commit()
    assert.equal(app.calls.filter(c => c[0] === 'send').length, 1)
  } finally { app.close() }
})

test('conflict keeps chosen quantity and requests a new preview', async () => {
  const app = await setup(async (_, options) => { if (options.body.check_only) return { preview_token: 'preview-v1', changes: [] }; throw new Error('库存已变化') })
  try {
    await app.state.open()
    app.state.values.value['material:1|raw_materials|3'] = 10
    await app.state.requestPreview()
    await app.state.commit()
    assert.equal(app.state.values.value['material:1|raw_materials|3'], 10)
    assert.equal(app.state.preview.value, null)
    assert.match(app.state.actionError.value, /库存已变化/)
  } finally { app.close() }
})

test('failed stock reload cannot preview stale choices and retry restores availability', async () => {
  let fail = false
  const app = await setup(async () => ({ changes: [] }), async () => { if (fail) throw new Error('网络中断'); return structuredClone(fixture) })
  try {
    fail = true
    await app.state.open()
    assert.match(app.state.loadError.value, /网络中断/)
    await app.state.requestPreview()
    assert.equal(app.calls.filter(c => c[0] === 'send').length, 0)
    fail = false
    await app.state.load()
    await app.state.requestPreview()
    assert.equal(app.calls.filter(c => c[0] === 'send').length, 1)
  } finally { app.close() }
})

test('malformed or tokenless response blocks preview instead of displaying no stock', async () => {
  let invalid = false
  const app = await setup(async () => ({ changes: [] }), async () => invalid ? { items: [] } : structuredClone(fixture))
  try {
    invalid = true
    await app.state.open()
    assert.match(app.state.loadError.value, /数据|版本/)
    await app.state.requestPreview()
    assert.equal(app.calls.filter(c => c[0] === 'send').length, 0)
  } finally { app.close() }
})

test('out-of-order refresh cannot replace newer stock data', async () => {
  let queue = false
  const pending = []
  const app = await setup(async () => ({ changes: [] }), async () => queue ? new Promise(resolve => pending.push(resolve)) : structuredClone(fixture))
  try {
    queue = true
    const first = app.state.load()
    const second = app.state.load()
    pending[1]({ ...structuredClone(fixture), draft_token: 'v2' })
    await second
    pending[0]({ ...structuredClone(fixture), draft_token: 'v1' })
    await first
    assert.equal(app.state.draftToken.value, 'v2')
  } finally { app.close() }
})

test('a successful response without a plan does not pretend the save was applied', async () => {
  const app = await setup(async (_, options) => options.body.check_only ? { preview_token: 'preview-v1', changes: [] } : {})
  try {
    await app.state.open()
    await app.state.requestPreview()
    await app.state.commit()
    assert.equal(app.state.opened.value, true)
    assert.match(app.state.actionError.value, /刷新计划核对/)
    assert.equal(app.events.some(event => event[0] === 'updated'), false)
  } finally { app.close() }
})

test('preview without verification token cannot be confirmed', async () => {
  const app = await setup(async () => ({ changes: [] }))
  try {
    await app.state.open()
    await app.state.requestPreview()
    assert.equal(app.state.previewComplete.value, false)
    await app.state.commit()
    assert.equal(app.calls.filter(c => c[0] === 'send').length, 1)
  } finally { app.close() }
})
