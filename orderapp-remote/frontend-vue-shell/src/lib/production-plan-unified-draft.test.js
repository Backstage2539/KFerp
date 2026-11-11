import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import { buildProductionPlanDraftPayload } from './produce-plan.js'

const viewURL = new URL('../views/ProducePlanView.vue', import.meta.url)
const draft = { id: 113, plan_no: 'PP-0000000113', status: 'draft', draft_token: 'v1', items: [], component_sources: [], operation_splits: [], readiness: { can_submit: false, blocking_count: 1, issues: [] } }
const demand = { product_id: 1, parent_product_id: 1, product: '验收咖啡', bom_spec_id: 10, spec_label: '227g', sales_unit: '袋', sales_spec_count: 2, gap_sales_spec_count: 2, gap_units: 2, gap_g: 454, need_g: 454, inventory_unit: '袋', need_inventory_qty: 2, gap_inventory_qty: 2, demand_status: 'unplanned', selection_id: 'a', selection_key: 'a', order_nos: 'SO-1' }

test('loaded custom arrangement survives unified draft normalization, workstation refresh and preset save', async () => {
  const app = await setup()
  try {
    const cap = { id: 7, workstation_id: 2, batch_size_qty: 15, batch_size_unit: 'kg', standard_minutes: 18, hourly_rate: 102, status: 'active' }
    const saved = { production_plan_item_id: 11, operation_seq: 1, workstation_capacity_id: 7, batch_size_qty: 18.7, custom_batch_size_qty: 18.7, batch_size_unit: 'kg', planned_qty: 37.4, planned_batch_count: 2 }
    const normalized = app.state.normalizeOperationSplit(saved)
    assert.equal(normalized.custom_batch_size_qty, 18.7)
    assert.equal(normalized.batch_count_source, 'manual')
    app.state.productionPlanSplitRows.value = [normalized]
    app.state.refreshSplitWorkstations({ capacities: [cap] })
    assert.equal(app.state.productionPlanSplitRows.value[0].batch_size_qty, 18.7)
    const fingerprint = app.state.productionPlanSplitFingerprint(app.state.productionPlanSplitRows.value)
    app.state.addSavedCapacityPreset({ ...cap, id: 8, batch_size_qty: 18.7 })
    assert.equal(app.state.productionPlanSplitFingerprint(app.state.productionPlanSplitRows.value), fingerprint)
    const payload = buildProductionPlanDraftPayload({ ...draft, operation_splits: app.state.productionPlanSplitRows.value })
    assert.equal(payload.operation_splits[0].custom_batch_size_qty, 18.7)
    assert.equal(payload.operation_splits[0].planned_batch_count, 2)
    assert.equal(payload.operation_splits[0].workstation_capacity_id, 7)
  } finally { app.close() }
})

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
  globalThis.document = { visibilityState: 'hidden', addEventListener() {}, removeEventListener() {}, querySelector: () => ({ scrollIntoView() {} }), getElementById: () => null }
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

test('manual supply refresh accepts wrapped and legacy details with the current version token', async () => {
  for (const wrapped of [true, false]) {
    const calls = []
    const updated = { ...draft, draft_token: 'v2', items: [{ id: 7, output_name: '半成品' }] }
    const app = await setup(undefined, { apiSend: async (endpoint, options) => { calls.push(options.body); return wrapped ? { plan: updated, changed: true } : updated } })
    try {
      await app.state.openProductionPlanDetail(draft)
      await app.state.refreshProductionPlanDetailSupply()
      assert.equal(calls[0].draft_token, 'v1')
      assert.equal(app.state.productionPlanDetail.value.id, 113)
      assert.equal(app.state.productionPlanDetail.value.items[0].output_name, '半成品')
      assert.equal(app.state.reviewedProductionPlanTokens['113'], 'v2')
    } finally { app.close() }
  }
})

test('visible draft automatically updates supply and retains the returned tasks', async () => {
  const calls = []
  const app = await setup(undefined, { apiSend: async (endpoint, options) => {
    calls.push([endpoint, options])
    return { plan: { ...draft, draft_token: 'v2', items: [{ id: 7, output_name: '半成品' }] }, changed: true, summary: '新增烘焙、色选' }
  } })
  try {
    globalThis.document.visibilityState = 'visible'
    await app.state.openProductionPlanDetail(draft)
    assert.equal(calls.length, 1)
    assert.deepEqual(calls[0][1].body, { check_only: false, draft_token: 'v1' })
    assert.equal(app.state.productionPlanDetail.value.items[0].output_name, '半成品')
    assert.match(app.state.productionSupplyStatus.value, /新增烘焙、色选/)
  } finally { app.close() }
})

test('creating a stock planning draft immediately reconciles its supply', async () => {
  const app = await setup(undefined, { apiSend: async endpoint => String(endpoint).endsWith('/refresh-supply')
    ? { plan: { ...draft, draft_token: 'v2', items: [{ id: 7, output_name: '上游半成品' }] }, changed: true }
    : structuredClone(draft) })
  try {
    globalThis.document.visibilityState = 'visible'
    await app.state.createStockPlanning()
    await new Promise(resolve => setTimeout(resolve, 0))
    const requests = app.calls.filter(([method, endpoint]) => method === 'apiSend' && String(endpoint).endsWith('/refresh-supply'))
    assert.equal(requests.length, 1)
    assert.equal(app.state.productionPlanDetail.value.items[0].output_name, '上游半成品')
  } finally { app.close() }
})

test('supply check during editing does not overwrite user input', async () => {
  const calls = []
  const app = await setup(undefined, { apiSend: async (endpoint, options) => {
    calls.push(options.body)
    return { plan: structuredClone(draft), changed: true, summary: '库存变化' }
  } })
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
    globalThis.document.visibilityState = 'visible'
    await app.state.syncProductionPlanSupply()
    assert.equal(calls[0].check_only, true)
    assert.equal(app.state.productionPlanDetail.value.items[0].target_warehouse, 'wip')
    assert.equal(app.state.productionPlanDetailDirty.value, true)
    assert.match(app.state.productionSupplyStatus.value, /保存后自动更新/)
  } finally { app.close() }
})

test('supply response arriving after editing starts never replaces local input', async () => {
  let resolveSync
  const app = await setup(undefined, { apiSend: () => new Promise(resolve => { resolveSync = resolve }) })
  try {
    await app.state.openProductionPlanDetail(draft)
    globalThis.document.visibilityState = 'visible'
    const pending = app.state.syncProductionPlanSupply()
    app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
    resolveSync({ plan: { ...draft, draft_token: 'v2', items: [{ id: 7 }] }, changed: true })
    await pending
    assert.equal(app.state.productionPlanDetail.value.items[0].target_warehouse, 'wip')
    assert.equal(app.state.productionPlanDetailDirty.value, true)
    assert.match(app.state.productionSupplyStatus.value, /当前输入已保留/)
  } finally { app.close() }
})

test('failed automatic supply check keeps the draft and exposes a retry state', async () => {
  const app = await setup(undefined, { apiSend: async () => { throw new Error('网络中断') } })
  try {
    await app.state.openProductionPlanDetail(draft)
    globalThis.document.visibilityState = 'visible'
    await app.state.syncProductionPlanSupply()
    assert.equal(app.state.productionPlanDetail.value.id, 113)
    assert.equal(app.state.productionSupplyError.value, true)
    assert.match(app.state.productionSupplyStatus.value, /网络中断/)
  } finally { app.close() }
})

test('batch submit uses reviewed versions and blocks every unreviewed plan', async () => {
  const app = await setup()
  try {
    app.state.reviewedProductionPlanTokens['41'] = 'v-41-reviewed'
    assert.throws(() => app.state.reviewedProductionPlanSubmitTokens([41, 42]), /42.*详情.*核对/)
    app.state.reviewedProductionPlanTokens['42'] = 'v-42-reviewed'
    assert.deepEqual(app.state.reviewedProductionPlanSubmitTokens([41, 42]), { '41': 'v-41-reviewed', '42': 'v-42-reviewed' })
    assert.equal(app.calls.filter(([method]) => method === 'apiGet').length, 0)
  } finally { app.close() }
})

test('batch submit sends the versions actually reviewed and reports another user updating a plan', async () => {
  let submission
  const app = await setup(undefined, { apiSend: async (endpoint, options) => {
    if (String(endpoint).endsWith('/refresh-supply')) return { plan: structuredClone(draft), changed: false }
    submission = options.body
    return { success: [], failed: [{ id: 113, error: '生产计划草稿版本已变化，请重新核对' }] }
  } })
  try {
    globalThis.document.visibilityState = 'visible'
    await app.state.openProductionPlanDetail(draft)
    app.state.reviewedProductionPlanTokens['114'] = 'v-reviewed-114'
    app.state.selectedProductionPlans['113'] = true
    app.state.selectedProductionPlans['114'] = true
    await app.state.submitSelectedProductionPlans()
    assert.deepEqual(submission.draft_tokens, { '113': 'v1', '114': 'v-reviewed-114' })
    assert.match(app.state.error.value, /版本已变化/)
    assert.equal(app.calls.filter(([method, endpoint]) => method === 'apiGet' && String(endpoint).includes('production-plans/113')).length, 1)
  } finally { app.close() }
})

test('batch submit does not send any plans if one has not been reviewed', async () => {
  const app = await setup()
  try {
    app.state.reviewedProductionPlanTokens['113'] = 'v1'
    app.state.selectedProductionPlans['113'] = true
    app.state.selectedProductionPlans['114'] = true
    await app.state.submitSelectedProductionPlans()
    assert.match(app.state.error.value, /114.*详情.*核对/)
    assert.equal(app.calls.filter(([method]) => method === 'apiSend').length, 0)
  } finally { app.close() }
})

test('legacy current-plan submission cannot bypass unsaved workstation arrangements', async () => {
  const app = await setup()
  try {
    app.state.currentPlan.value = { ...draft, operation_splits: [] }
    app.state.reviewedProductionPlanTokens['113'] = 'v1'
    app.state.operationSplits.value = [{ production_plan_item_id: 10, operation_seq: 1, operation: '烘焙', workstation_capacity_id: 2, planned_qty: 1 }]
    await app.state.submitCurrentProductionPlan()
    assert.equal(app.calls.filter(([method]) => method === 'apiSend').length, 0)
    assert.match(app.state.previewError.value, /工位安排.*尚未保存.*详情/)
  } finally { app.close() }
})

test('legacy current-plan submission uses the reviewed token when arrangements match the saved plan', async () => {
  const app = await setup(undefined, { apiSend: async () => ({ success: [], failed: [] }) })
  try {
    app.state.currentPlan.value = { ...draft, operation_splits: [] }
    app.state.reviewedProductionPlanTokens['113'] = 'v-reviewed'
    await app.state.submitCurrentProductionPlan()
    const submission = app.calls.find(([method, endpoint]) => method === 'apiSend' && String(endpoint).endsWith('/production-plans/submit'))
    assert.deepEqual(submission[2].body.draft_tokens, { '113': 'v-reviewed' })
  } finally { app.close() }
})

test('explicit workstation review sends acknowledgement while ordinary draft save does not', async () => {
  const writes = []
  const app = await setup(undefined, { apiSend: async (endpoint, options) => { writes.push(options.body); return structuredClone(draft) } })
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
    await app.state.saveProductionPlanDetailDraft()
    assert.equal(writes[0].ack_supply_recheck, undefined)
    await app.state.saveProductionPlanDetailDraft(true)
    assert.equal(writes[1].ack_supply_recheck, true)
  } finally { app.close() }
})

test('successful supply reconciliation clears the pending submit-conflict message', async () => {
  const app = await setup(undefined, { apiSend: async endpoint => {
    if (String(endpoint).endsWith('/submit')) throw Object.assign(new Error('供应已变化'), { status: 409 })
    return { plan: { ...draft, draft_token: 'v2' }, changed: true, summary: '工序已更新，请核对' }
  } })
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.readiness.can_submit = true
    globalThis.document.visibilityState = 'visible'
    await app.state.submitProductionPlanDetail()
    await new Promise(resolve => setTimeout(resolve, 0))
    assert.equal(app.state.productionPlanDetailError.value, '')
    assert.match(app.state.productionSupplyStatus.value, /工序已更新/)
  } finally { app.close() }
})

test('failed post-submit reconciliation shows the failure and clears it after a successful retry', async () => {
  let fail = true
  const app = await setup(undefined, { apiSend: async endpoint => {
    if (String(endpoint).endsWith('/submit')) throw Object.assign(new Error('供应已变化'), { status: 409 })
    if (fail) { fail = false; throw new Error('网络故障') }
    return { plan: { ...draft, draft_token: 'v2' }, changed: true, summary: '已更新' }
  } })
  try {
    await app.state.openProductionPlanDetail(draft)
    app.state.productionPlanDetail.value.readiness.can_submit = true
    globalThis.document.visibilityState = 'visible'
    await app.state.submitProductionPlanDetail()
    await new Promise(resolve => setTimeout(resolve, 0))
    assert.match(app.state.productionPlanDetailError.value, /网络故障/)
    await app.state.syncProductionPlanSupply()
    assert.equal(app.state.productionPlanDetailError.value, '')
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

test('saving capacity before final submit keeps the detail and submits the saved revision', async () => {
 const app = await setup(undefined, { apiSend: async (endpoint, options) => {
  if (String(endpoint).endsWith('/submit')) return { plan: { ...draft, status: 'submitted', draft_token: 'v2' }, work_orders: [] }
  if (String(endpoint).includes('operation-splits/preview')) return { operation_coverage: [] }
  return { ...draft, draft_token: 'v2', operation_splits: options.body?.operation_splits || [], readiness: { can_submit: true, blocking_count: 0, issues: [] } }
 } })
 try {
  await app.state.openProductionPlanDetail(draft)
  app.state.productionPlanSplitDrawer.value = structuredClone(draft)
  app.state.productionPlanSplitReturnTarget.value = 'detail'
  app.state.productionPlanCapacityActive.value = true
  assert.equal(await app.state.saveProductionPlanSplitDrawer(false), true)
  assert.equal(app.state.productionPlanDetail.value.draft_token, 'v2')
  assert.equal(app.state.productionPlanDetail.value.readiness.can_submit, true)
  await app.state.submitFromCapacityWorkspace()
  const submission = app.calls.find(([method, endpoint]) => method === 'apiSend' && String(endpoint).endsWith('/submit'))
  assert.equal(submission[2].body.draft_token, 'v2')
  assert.equal(app.state.productionPlanDetail.value.status, 'submitted')
  assert.equal(app.state.productionPlanSplitDrawer.value, null)
 } finally { app.close() }
})

test('opening transfer saves dirty draft and uses the refreshed material identity', async () => {
 const refreshedSource = { id: 7, component_name: '新来源' }
 const app = await setup(undefined, { apiSend: async () => ({ ...draft, draft_token: 'v2', component_sources: [refreshedSource], items: [{ id: 1, target_warehouse: 'wip' }] }) })
 try {
  await app.state.openProductionPlanDetail(draft)
  app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
  await Vue.nextTick()
  await app.state.openPlanTransferDrawer('single', { id: 7, component_name: '旧来源' })
  assert.equal(app.state.productionPlanDetail.value.draft_token, 'v2')
  assert.equal(app.state.productionPlanTransferDrawer.value.source.component_name, '新来源')
 } finally { app.close() }
})

test('opening stock view preserves unsaved draft without saving', async () => {
 const app = await setup()
 try {
  await app.state.openProductionPlanDetail(draft)
  app.state.productionPlanDetail.value.items.push({ id: 1, target_warehouse: 'wip' })
  await Vue.nextTick()
  await app.state.openPlanTransferDrawer('stock', { id: 7 })
  assert.equal(app.state.productionPlanTransferDrawer.value.mode, 'stock')
  assert.equal(app.calls.filter(([method]) => method === 'apiSend').length, 0)
 } finally { app.close() }
})

test('source adjustment reports save failure without discarding local input', async () => {
 const app = await setup(undefined, { apiSend: async () => { throw new Error('保存失败，请重试') } })
 try {
  await app.state.openProductionPlanDetail(draft)
  app.state.productionPlanDetail.value.component_sources.push({ id: 7, production_plan_item_id: 1, component_type: 'material', component_id: 10, picking_version: 1, allocation_mode: 'auto', manual_allocations: [] })
  const source = app.state.productionPlanDetail.value.component_sources[0]
  let error = ''
  await app.state.savePlanTransferAdjustment(source, [{ warehouse: 'raw_materials', qty_g: 100 }], message => { error = message || '' })
  assert.match(error, /保存失败/)
  assert.equal(source.manual_allocations[0].qty_g, 100)
  assert.equal(app.state.productionPlanDetailDirty.value, true)
 } finally { app.close() }
})

test('source adjustment refreshes the drawer source after saving the draft', async () => {
 const fresh = { id: 7, production_plan_item_id: 1, component_type: 'material', component_id: 10, picking_version: 1, component_name: '已更新来源' }
 const app = await setup(undefined, { apiSend: async () => ({ ...draft, draft_token: 'v2', component_sources: [fresh] }) })
 try {
  await app.state.openProductionPlanDetail(draft)
  app.state.productionPlanDetail.value.component_sources.push({ ...fresh, component_name: '旧来源' })
  const source = app.state.productionPlanDetail.value.component_sources[0]
  app.state.productionPlanTransferDrawer.value = { mode: 'stock', source }
  let result = 'not called'
  await app.state.savePlanTransferAdjustment(source, [{ warehouse: 'raw_materials', qty_g: 100 }], error => { result = error || 'ok' })
  assert.equal(result, 'ok')
  assert.equal(app.state.productionPlanTransferDrawer.value.source.component_name, '已更新来源')
 } finally { app.close() }
})
