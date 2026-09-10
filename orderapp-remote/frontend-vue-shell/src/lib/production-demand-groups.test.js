import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileTemplate } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import { renderToString } from '@vue/server-renderer'
import * as plan from './produce-plan.js'

test('actual current-plan template renders a selection preview without a persisted draft', async () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const start = source.indexOf('<section :class="[\'panel current-plan-panel\'')
  const end = source.indexOf('\n      </section>', start) + '\n      </section>'.length
  const { code, errors } = compileTemplate({ source: source.slice(start, end), filename: 'ProducePlanView.vue', id: 'preview-regression' })
  assert.deepEqual(errors, [])
  const js = code.replace(/import \{([^}]+)\} from "vue"/g, (_, names) => `const {${names.replace(/ as /g, ':')}} = Vue`).replace('export function render', 'function render')
  const render = new Function('Vue', js + '\nreturn render')(Vue)
  for (const state of [
    { hasSelectedRows: true, planReady: true },
    { hasSelectedRows: true, previewLoading: true },
    { hasSelectedRows: true, previewError: '预览失败，请重试' },
    { hasSelectedRows: false },
  ]) {
    const html = await renderToString(Vue.createSSRApp({ render, components: { ProductionSupplyAllocations: { render: () => null } }, setup: () => ({
      previewSupplyAllocations: [], currentPlan: null, currentPlanPanelCollapsed: false, currentPlanDraft: false,
      hasSelectedRows: false, planReady: false, previewLoading: false, previewError: '',
      computedPlanRows: [], computedMaterials: [], computedManufacturingPlanRows: [], postSubmitActions: [],
      saving: false, loading: false, toggleCurrentPlanPanelCollapsed() {}, ...state,
    }) }))
    assert.match(html, /当前生产计划/)
    assert.doesNotMatch(html, /保存<\/button>/)
  }
})

const demand = (overrides = {}) => ({ product_id: 1, parent_product_id: 1, product: '同名咖啡', bom_spec_id: 10, spec_label: '227g', sales_unit: '袋', sales_spec_count: 2, gap_sales_spec_count: 2, inventory_unit: '袋', need_inventory_qty: 2, gap_inventory_qty: 2, demand_status: 'unplanned', selection_key: 'a', order_nos: 'SO-1', ...overrides })

test('group by product identity, then spec; preserve source selections and mixed units', () => {
  const groups = plan.groupProductionDemands([
    demand(), demand({ selection_key: 'b', order_nos: 'SO-2', customer_id: 2 }),
    demand({ bom_spec_id: 11, spec_label: '454g', selection_key: 'c' }),
    demand({ bom_spec_id: 12, spec_label: '礼盒', sales_unit: '盒', selection_key: 'd' }),
    demand({ product_id: 2, parent_product_id: 2, selection_key: 'e' }),
  ])
  assert.equal(groups.length, 2)
  assert.equal(groups[0].specs.length, 3)
  assert.equal(groups[0].specs[0].need_label, '4 袋')
  assert.equal(groups[0].specs[0].order_nos, 'SO-1、SO-2')
  assert.deepEqual(groups[0].specs[0].rows.map(r => r.selection_key), ['a', 'b'])
  assert.equal(groups[0].need_label, '6 袋 / 2 盒')
  assert.deepEqual(plan.buildProductionGroupSelection(groups[0].rows, { e: true }, true), { e: true, a: true, b: true, c: true, d: true })
})

test('group selection preserves other pages and skips blocked, planned, covered demand', () => {
  const rows = [demand(), demand({ selection_key: 'b', blocking_reason: '无法换算' }), demand({ selection_key: 'c', demand_status: 'in_production' }), demand({ selection_key: 'd', gap_inventory_qty: 0 })]
  assert.deepEqual(plan.buildProductionGroupSelection(rows, { elsewhere: true }, true), { elsewhere: true, a: true })
  assert.deepEqual(plan.buildProductionGroupSelection(rows, { elsewhere: true, a: true }, false), { elsewhere: true })
  const group = plan.groupProductionDemands([demand({ sales_spec_count: 0, need_units: 0, need_g: 1000, bom_spec_id: 0, spec_g: 0 })])[0]
  assert.equal(group.specs[0].need_label, '件数待确认')
})

test('actual demand table renders backend order quantities and customer traceability', async () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const start = source.indexOf('<table class="demand-table"')
  const end = source.indexOf('</table>', start) + '</table>'.length
  const { code, errors } = compileTemplate({ source: source.slice(start, end), filename: 'ProducePlanView.vue', id: 'demand-contract' })
  assert.deepEqual(errors, [])
  const js = code.replace(/import \{([^}]+)\} from "vue"/g, (_, names) => `const {${names.replace(/ as /g, ':')}} = Vue`).replace('export function render', 'function render')
  const render = new Function('Vue', js + '\nreturn render')(Vue)
  const rows = [demand({ order_details: [{ order_item_id: 17, order_no: 'SO-17', customer_name: '验收客户', quantity: 2, sales_unit: '袋' }] }), demand({selection_key: 'b'})]
  const html = await renderToString(Vue.createSSRApp({ render, setup: () => ({ ...plan,
    stockInsufficientRows: rows, pagedDemandGroups: plan.groupProductionDemands(rows), collapsedDemandGroups: {}, selected: {a: true},
    productionDemandSelectionKey: row => row.selection_key, toggleDemandGroup() {},
  }) }))
  assert.match(html, /SO-17 · 验收客户 · 2 袋/)
  assert.deepEqual(plan.productionDemandSelectionState(rows, {a: true}), {checked: false, indeterminate: true, selectedCount: 1, total: 2})
  assert.equal(plan.productionDemandSelectionState(rows, {a: true,b: true}).checked, true)
  assert.equal(plan.productionDemandSelectionState(rows, {}).indeterminate, false)
})

test('refresh with selection query executes the actual mounted callback and requests a preview', async () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const start = source.indexOf('onMounted(async () => {') + 'onMounted(async () => {'.length
  const end = source.indexOf('\n})', start)
  const selected = {}, filters = {}, calls = []
  const run = new (Object.getPrototypeOf(async function(){}).constructor)('window','filters','props','selected','load','loadWorkstationCapacities','loadProductionPlans','loadWarehouses','productionDemandStatusFilterValue','defaultProductionDemandStatusFilter',source.slice(start,end))
  await run({location:{href:'https://example.invalid/app/production?plan=1&selected=v2%3Atest%2C1-227'}},filters,{},selected,async preview => calls.push(preview),async()=>{},async()=>{},async()=>{},plan.productionDemandStatusFilterValue,plan.defaultProductionDemandStatusFilter)
  assert.deepEqual(selected, {'v2:test':true,'1-227':true})
  assert.deepEqual(calls,[true])
})
