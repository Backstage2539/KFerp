import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'

const viewURL = new URL('../views/ProducePlanView.vue', import.meta.url)
const draft = { id: 113, plan_no: 'PP-0000000113', status: 'draft', draft_token: 'v1', items: [], component_sources: [], operation_splits: [], readiness: { can_submit: false, blocking_count: 1, issues: [] } }
const demand = { product_id: 1, parent_product_id: 1, product: '验收咖啡', bom_spec_id: 10, spec_label: '227g', sales_unit: '袋', sales_spec_count: 2, gap_sales_spec_count: 2, gap_units: 2, gap_g: 454, need_g: 454, inventory_unit: '袋', need_inventory_qty: 2, gap_inventory_qty: 2, demand_status: 'unplanned', selection_id: 'a', selection_key: 'a', order_nos: 'SO-1' }

async function setup(url = 'https://dev.example/app/vue-shell?view=productionFlow', api = {}) {
  const previousWindow = globalThis.window
  const previousDocument = globalThis.document
  const storage = new Map()
  const events = new Map()
  const histories = []
  const window = globalThis.window = {
    location: { href: url, origin: 'https://dev.example' },
    confirm: () => true, alert() {}, dispatchEvent() {},
    sessionStorage: { getItem: key => storage.get(key) ?? null, setItem: (key, value) => storage.set(key, value), removeItem: key => storage.delete(key) },
    addEventListener: (name, fn) => events.set(name, fn), removeEventListener: name => events.delete(name),
    history: { state: {}, replaceState(state, _, href) { this.state = state; window.location.href = new URL(href, window.location.href).href }, pushState(state, _, href) { histories.push(href); this.replaceState(state, _, href) } },
  }
  globalThis.document = { querySelector: () => ({ scrollIntoView() {} }), getElementById: () => null }
  const calls = []
  const cleanups = []
  const mounted = []
  const defaults = {
    async apiGet(endpoint) {
      if (String(endpoint).match(/production-plans\/113$/)) return structuredClone(draft)
      if (String(endpoint).includes('unproduced')) return { rows: [demand], plan_rows: [demand] }
      return { rows: [] }
    },
    async apiSend() { return structuredClone(draft) },
  }
  const client = Object.fromEntries(['apiGet', 'apiSend'].map(key => [key, async (...args) => { calls.push([key, ...args]); return (api[key] || defaults[key])(...args) }]))
  const { descriptor } = parse(fs.readFileSync(viewURL, 'utf8'))
  const compiled = compileScript(descriptor, { id: 'unified-draft-behavior' })
  const bindings = {}
  const modules = new Map()
  for (const [name, binding] of Object.entries(compiled.imports)) {
    if (!modules.has(binding.source)) {
      modules.set(binding.source, binding.source === 'vue'
        ? { ...Vue, onMounted: fn => mounted.push(fn), onBeforeUnmount: fn => cleanups.push(fn), onActivated() {}, onDeactivated() {} }
        : binding.source === '../api/client' ? client
          : binding.source.endsWith('.vue') ? { default: {} }
            : await import(binding.source.startsWith('.') ? new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, viewURL) : binding.source))
    }
    bindings[name] = modules.get(binding.source)[binding.imported]
  }
  const code = compiled.content.replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\n/gm, '').replace('export default', 'return')
  const component = new Function(...Object.keys(bindings), code)(...Object.values(bindings))
  const scope = Vue.effectScope()
  const state = scope.run(() => component.setup({ embedded: true, viewParams: {}, customerContextId: 0 }, { expose() {} }))
  return { state, window, calls, histories, mounted, async readySelection() {
    state.rows.value = [structuredClone(demand)]
    state.replaceSelected({ a: true })
    await Vue.nextTick()
    state.planRows.value = [structuredClone(demand)]
    state.activePlanningStep.value = 'reviewGap'
  }, close() { cleanups.forEach(fn => fn()); scope.stop(); globalThis.window = previousWindow; globalThis.document = previousDocument } }
}

test('mounted selection refresh revalidates demand and restores only the two-step preview', async () => {
  const app = await setup('https://dev.example/app/vue-shell?view=productionFlow&selected=a,obsolete&planning_step=reviewGap')
  try {
    for (const mount of app.mounted) await mount()
    assert.equal(app.state.activePlanningStep.value, 'reviewGap')
    assert.equal(app.state.productionPlanDetail.value, null)
    assert.match(app.state.notice.value, /部分需求/)
    assert.equal(app.state.selected.obsolete, undefined)
    assert.ok(app.calls.some(([, url]) => String(url).includes('plan=1')))
    assert.equal(app.calls.filter(([method]) => method === 'apiSend').length, 0)
  } finally { app.close() }
})

test('save conflict preserves user input and dirty state until explicit reload', async () => {
  const app = await setup(undefined, { apiSend: async () => { throw Object.assign(new Error('stale draft'), { status: 409 }) } })
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
    await app.state.saveProductionPlanDetailDraft()
    assert.equal(app.state.productionPlanDetail.value.items[0].target_warehouse, 'wip')
    assert.equal(app.state.productionPlanDetailDirty.value, true)
    assert.match(app.state.productionPlanDetailError.value, /当前输入已保留.*重新加载/)
  } finally { app.close() }
})

test('selection step refresh keeps URL choices when unplanned summary has an empty selected map', async () => {
  const app = await setup('https://dev.example/app/vue-shell?view=productionFlow&selected=a&planning_step=selectDemand', {
    apiGet: async endpoint => String(endpoint).includes('unproduced') ? { rows: [demand], selected: {} } : { rows: [] },
  })
  try {
    await app.state.restorePlanningLocation()
    assert.equal(app.state.selected.a, true)
    assert.equal(new URL(app.window.location.href).searchParams.get('selected'), 'a')
  } finally { app.close() }
})

test('failed supply refresh preserves input and never marks it as saved', async () => {
  const app = await setup(undefined, { apiSend: async () => { throw new Error('供应暂时不可用') } })
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
    await app.state.refreshProductionPlanDetailSupply()
    assert.equal(app.state.productionPlanDetailDirty.value, true)
    assert.equal(app.state.productionPlanDetail.value.items[0].target_warehouse, 'wip')
    assert.match(app.state.productionPlanDetailError.value, /供应暂时不可用/)
  } finally { app.close() }
})

test('cancelled draft returns to demand selection and removes the document URL', async () => {
  const app = await setup(undefined, { apiSend: async () => ({ ...draft, status: 'cancelled' }) })
  try {
    await app.state.openProductionPlanDetail(draft)
    await app.state.cancelProductionPlanDraft(app.state.productionPlanDetail.value, 'detail')
    assert.equal(app.state.productionPlanDetail.value, null)
    assert.equal(app.state.currentPlanStepKey.value, 'selectDemand')
    assert.equal(new URL(app.window.location.href).searchParams.has('production_plan_id'), false)
    assert.match(app.state.notice.value, /已撤销/)
    assert.ok(app.calls.some(([, endpoint]) => String(endpoint).includes('unproduced')))
  } finally { app.close() }
})

test('sidebar navigation refuses dirty edits, and history navigation restores server state', async () => {
  const app = await setup()
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
    app.window.confirm = () => false
    let prevented = false
    app.state.handlePlanningNavigation({ preventDefault() { prevented = true }, stopImmediatePropagation() {} })
    assert.equal(prevented, true)
    app.window.location.href = 'https://dev.example/app/vue-shell?view=productionFlow'
    await app.state.handlePlanningPopState()
    assert.equal(new URL(app.window.location.href).searchParams.get('production_plan_id'), '113')
    app.window.confirm = () => true
    app.window.location.href = 'https://dev.example/app/vue-shell?view=productionFlow'
    await app.state.handlePlanningPopState()
    assert.equal(app.state.productionPlanDetail.value, null)
    app.window.location.href = 'https://dev.example/app/vue-shell?view=productionFlow&production_plan_id=113'
    await app.state.handlePlanningPopState()
    assert.equal(app.state.productionPlanDetail.value.id, 113)
    assert.equal(app.state.productionPlanDetailDirty.value, false)
  } finally { app.close() }
})

test('retry after uncertain create response reuses the same idempotency key', async () => {
  let first = true
  const app = await setup(undefined, { apiSend: async () => {
    if (first) { first = false; throw new Error('network timeout') }
    return structuredClone(draft)
  } })
  try {
    await app.readySelection()
    await app.state.createProductionPlan()
    await app.state.createProductionPlan()
    const requests = app.calls.filter(([method]) => method === 'apiSend')
    assert.equal(requests.length, 2)
    assert.equal(requests[0][2].body.request_id, requests[1][2].body.request_id)
    assert.equal(app.state.productionPlanDetail.value.id, 113)
  } finally { app.close() }
})

test('create twice only writes once and enters the same URL-addressable detail as a plan number', async () => {
  const releases = []
  const app = await setup(undefined, { apiSend: () => new Promise(resolve => { releases.push(resolve) }) })
  try {
    await app.readySelection()
    const first = app.state.createProductionPlan()
    const second = app.state.createProductionPlan()
    releases.forEach(release => release(structuredClone(draft)))
    await Promise.all([first, second])
    assert.equal(app.calls.filter(([method]) => method === 'apiSend').length, 1)
    assert.equal(app.state.productionPlanDetail.value.id, 113)
    assert.equal(new URL(app.window.location.href).searchParams.get('production_plan_id'), '113')
    assert.equal(new URL(app.window.location.href).searchParams.has('selected'), false)
    assert.match(app.state.notice.value, /草稿.*已创建/)
    const fromCreate = structuredClone(Vue.toRaw(app.state.productionPlanDetail.value))
    await app.state.openProductionPlanDetail(draft)
    assert.deepEqual(Vue.toRaw(app.state.productionPlanDetail.value), fromCreate)
  } finally { app.close() }
})

test('refresh with plan id bypasses stale creation preview and restores server status', async () => {
  const app = await setup('https://dev.example/app/vue-shell?view=productionFlow&production_plan_id=113&selected=stale&plan=1', {
    apiGet: async endpoint => String(endpoint).endsWith('/113') ? { ...draft, status: 'submitted' } : { rows: [] },
  })
  try {
    await app.state.restorePlanningLocation()
    assert.equal(app.state.productionPlanDetail.value.status, 'submitted')
    assert.equal(app.calls.filter(([method]) => method === 'apiSend').length, 0)
    assert.equal(app.calls.some(([, url]) => String(url).includes('unproduced')), false)
    assert.deepEqual(Object.keys(app.state.selected), [])
    assert.equal(new URL(app.window.location.href).searchParams.has('selected'), false)
  } finally { app.close() }
})

test('creation survives detail load failure; retry reads the known draft without another create', async () => {
  let fail = true
  const app = await setup(undefined, { apiGet: async endpoint => {
    if (String(endpoint).endsWith('/113')) { if (fail) throw new Error('network unavailable'); return structuredClone(draft) }
    return { rows: [] }
  } })
  try {
    await app.readySelection()
    await app.state.createProductionPlan()
    assert.equal(app.state.productionPlanDetail.value.id, 113)
    assert.match(app.state.productionPlanDetailError.value, /network unavailable/)
    assert.equal(new URL(app.window.location.href).searchParams.get('production_plan_id'), '113')
    fail = false
    await app.state.reloadProductionPlanDetail()
    assert.equal(app.state.productionPlanDetailError.value, '')
    assert.equal(app.calls.filter(([method]) => method === 'apiSend').length, 1)
  } finally { app.close() }
})

test('dirty draft guards back and page refresh without losing edits', async () => {
  const app = await setup()
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
    app.window.confirm = () => false
    await app.state.closeProductionPlanDetail()
    assert.equal(app.state.productionPlanDetail.value.id, 113)
    let prevented = false
    app.state.handlePlanningBeforeUnload({ preventDefault() { prevented = true }, returnValue: undefined })
    assert.equal(prevented, true)
  } finally { app.close() }
})

test('an older selection preview cannot overwrite an opened draft or its address', async () => {
  let finishPreview
  const app = await setup(undefined, { apiGet: async endpoint => String(endpoint).includes('unproduced')
    ? new Promise(resolve => { finishPreview = resolve }) : String(endpoint).endsWith('/113') ? structuredClone(draft) : { rows: [] } })
  try {
    await app.readySelection()
    const preview = app.state.loadSelectedPlanPreview()
    await app.state.openProductionPlanDetail(draft)
    finishPreview({ rows: [demand], selected: { a: true }, plan_rows: [demand] })
    await preview
    assert.equal(app.state.productionPlanDetail.value.id, 113)
    assert.equal(new URL(app.window.location.href).searchParams.get('production_plan_id'), '113')
    assert.deepEqual(Object.keys(app.state.selected), [])
  } finally { app.close() }
})

test('late draft load cannot reopen a document after browser back returns to selection', async () => {
  let finishDetail
  const app = await setup('https://dev.example/app/vue-shell?view=productionFlow&production_plan_id=113', {
    apiGet: async endpoint => String(endpoint).endsWith('/113') ? new Promise(resolve => { finishDetail = resolve }) : { rows: [demand] },
  })
  try {
    const oldRestore = app.state.restorePlanningLocation()
    await Vue.nextTick()
    app.window.location.href = 'https://dev.example/app/vue-shell?view=productionFlow'
    await app.state.restorePlanningLocation()
    finishDetail(structuredClone(draft))
    await oldRestore
    assert.equal(app.state.productionPlanDetail.value, null)
    assert.equal(new URL(app.window.location.href).searchParams.has('production_plan_id'), false)
  } finally { app.close() }
})
