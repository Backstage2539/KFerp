import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import * as Vue from 'vue'

const file = new URL('../views/ProducePlanView.vue', import.meta.url)
const original = {
  id: 113, plan_no: 'PP-TEST', status: 'draft', draft_token: 'v1',
  items: [{ id: 12, target_warehouse: 'finished_goods' }], operation_splits: [],
  component_sources: [{ id: 7, production_plan_item_id: 12, component_type: 'material', component_id: 8, component_bom_spec_id: 0, component_spec_g: 0, required_g: 1000, demand_g: 1000, wip_covered_g: 0, allocation_mode: 'auto', manual_allocations: [], picking_version: 1 }],
  material_transfer_documents: [{ id: 50, source_id: 7, status: 'draft' }],
  readiness: { can_submit: false, blocking_count: 0, issues: [] },
}

async function setup(fresh) {
  const previousWindow = globalThis.window, previousDocument = globalThis.document
  globalThis.window = { location: { href: 'https://example.test/app/vue-shell?view=productionFlow', origin: 'https://example.test' }, confirm: () => true, alert() {}, dispatchEvent() {}, sessionStorage: { getItem: () => null, setItem() {}, removeItem() {} }, addEventListener() {}, removeEventListener() {}, history: { state: {}, replaceState() {}, pushState() {} } }
  globalThis.document = { visibilityState: 'hidden', addEventListener() {}, removeEventListener() {}, querySelector: () => null, getElementById: () => null }
  const calls = []
  const api = { apiGet: async () => structuredClone(original), apiSend: async (url, options) => { calls.push([url, options]); return { plan: structuredClone(fresh), changed: false } } }
  const { descriptor } = parse(fs.readFileSync(file, 'utf8'))
  const compiled = compileScript(descriptor, { id: 'plan-live-readonly' })
  const bindings = {}, modules = new Map()
  for (const [name, binding] of Object.entries(compiled.imports)) {
    if (!modules.has(binding.source)) modules.set(binding.source, binding.source === 'vue' ? { ...Vue, onMounted() {}, onBeforeUnmount() {}, onActivated() {}, onDeactivated() {} } : binding.source === '../api/client' ? api : binding.source.endsWith('.vue') ? { default: {} } : await import(binding.source.startsWith('.') ? new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, file) : binding.source))
    bindings[name] = modules.get(binding.source)[binding.imported]
  }
  const code = compiled.content.replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\n/gm, '').replace('export default', 'return')
  const component = new Function(...Object.keys(bindings), code)(...Object.values(bindings))
  const scope = Vue.effectScope()
  const state = scope.run(() => component.setup({ embedded: true, viewParams: {}, customerContextId: 0 }, { expose() {} }))
  await state.openProductionPlanDetail(original)
  return { state, calls, close() { scope.stop(); globalThis.window = previousWindow; globalThis.document = previousDocument } }
}

test('dirty material inputs retain edits while WIP and linked transfer status refresh', async () => {
  const fresh = structuredClone(original)
  fresh.component_sources[0].wip_covered_g = 1000
  fresh.material_transfer_documents[0].status = 'submitted'
  const app = await setup(fresh)
  try {
    app.state.productionPlanDetail.value.items[0].target_warehouse = 'wip'
    app.state.productionPlanDetail.value.component_sources[0].allocation_mode = 'manual'
    app.state.productionPlanDetail.value.component_sources[0].manual_allocations = [{ warehouse: 'raw_materials', qty_g: 1000 }]
    await Vue.nextTick()
    globalThis.document.visibilityState = 'visible'
    await app.state.syncProductionPlanSupply()
    const plan = app.state.productionPlanDetail.value
    assert.equal(app.calls[0][1].body.check_only, true)
    assert.equal(plan.items[0].target_warehouse, 'wip')
    assert.equal(plan.component_sources[0].allocation_mode, 'manual')
    assert.equal(plan.component_sources[0].manual_allocations[0].qty_g, 1000)
    assert.equal(plan.component_sources[0].wip_covered_g, 1000)
    assert.equal(plan.material_transfer_documents[0].status, 'submitted')
    assert.equal(app.state.productionPlanDetailDirty.value, true)
  } finally { app.close() }
})

test('capacity edits remain intact while readonly WIP refreshes in both visible views', async () => {
  const fresh = structuredClone(original)
  fresh.component_sources[0].wip_covered_g = 600
  const app = await setup(fresh)
  try {
    app.state.productionPlanSplitDrawer.value = structuredClone(original)
    app.state.productionPlanSplitRows.value = [{ production_plan_item_id: 12, planned_qty: 5 }]
    app.state.productionPlanSplitSavedFingerprint.value = 'old-value'
    globalThis.document.visibilityState = 'visible'
    await app.state.syncProductionPlanSupply()
    assert.equal(app.state.productionPlanDetail.value.component_sources[0].wip_covered_g, 600)
    assert.equal(app.state.productionPlanSplitDrawer.value.component_sources[0].wip_covered_g, 600)
    assert.equal(app.state.productionPlanSplitRows.value[0].planned_qty, 5)
  } finally { app.close() }
})

test('mismatched material identity is not replaced during editing', async () => {
  const fresh = structuredClone(original)
  fresh.component_sources[0].component_id = 99
  fresh.component_sources[0].wip_covered_g = 999
  const app = await setup(fresh)
  try {
    app.state.productionPlanDetail.value.items[0].target_warehouse = 'wip'
    globalThis.document.visibilityState = 'visible'
    await app.state.syncProductionPlanSupply()
    assert.equal(app.state.productionPlanDetail.value.component_sources[0].component_id, 8)
    assert.equal(app.state.productionPlanDetail.value.component_sources[0].wip_covered_g, 0)
  } finally { app.close() }
})
