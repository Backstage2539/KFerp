import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import { renderToString } from '@vue/server-renderer'
import * as plan from './produce-plan.js'

test('complete plan setup initializes real Vue watchers and keeps group pagination in range', async () => {
  const viewURL = new URL('../views/ProducePlanView.vue', import.meta.url)
  const { descriptor } = parse(fs.readFileSync(viewURL, 'utf8'))
  const compiled = compileScript(descriptor, { id: 'plan-setup-regression' })
  const bindings = {}
  const modules = new Map()
  for (const [name, binding] of Object.entries(compiled.imports)) {
    if (!modules.has(binding.source)) {
      const module = binding.source === 'vue'
        // Defer browser lifecycle work, but execute actual computed/watch logic.
        ? { ...Vue, onMounted() {}, onBeforeUnmount() {} }
        : binding.source.endsWith('.vue')
          ? { default: {} }
          : await import(new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, viewURL))
      modules.set(binding.source, module)
    }
    bindings[name] = modules.get(binding.source)[binding.imported]
  }
  const code = compiled.content
    .replace(/^import\s+[\s\S]*?\s+from\s+(['"])[^'"]+\1;?\n/gm, '')
    .replace('export default', 'return')
  const component = new Function(...Object.keys(bindings), code)(...Object.values(bindings))
  const scope = Vue.effectScope()
  try {
    const state = scope.run(() => component.setup({ embedded: true, viewParams: {}, customerContextId: 0 }, { expose() {} }))
    assert.deepEqual(state.pagedDemandGroups.value, [])
    state.rows.value = Array.from({ length: 21 }, (_, i) => demand({ product_id: i + 1, parent_product_id: i + 1, selection_key: `row-${i}` }))
    await Vue.nextTick()
    assert.equal(state.pagedDemandGroups.value.length, 20)
    state.demandPage.value = 2
    assert.equal(state.pagedDemandGroups.value.length, 1)
    state.rows.value = state.rows.value.slice(0, 2)
    await Vue.nextTick()
    assert.equal(state.demandPage.value, 1)
    assert.equal(state.pagedDemandGroups.value.length, 2)
  } finally {
    scope.stop()
  }
})

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

test('BOM configuration errors preserve known sales quantities and frozen stock conversion', () => {
  const rows = [3, 1].map((quantity, index) => demand({
    selection_key: `blocked-${index}`, bom_spec_id: 0, spec_g: 1000,
    sales_unit: 'kg', inventory_unit: 'kg', inventory_qty_per_sales_unit: 1,
    sales_spec_count: quantity, gap_sales_spec_count: quantity,
    need_g: quantity * 1000, gap_g: quantity * 1000,
    blocking_reason: 'BOM 缺少物料明细，请完善对应版本后刷新需求。',
  }))
  assert.equal(plan.productionSalesQuantityLabel(rows), '4 kg')
  assert.equal(plan.productionSalesQuantityLabel(rows, 'available'), '0 kg')
  assert.equal(plan.productionSalesQuantityLabel(rows, 'gap'), '4 kg')
  assert.equal(plan.productionDemandSelectionState(rows, {}).total, 0)

  const invalidConversion = demand({ inventory_qty_per_sales_unit: 0, blocking_reason: '销售单位无法换算到库存单位' })
  assert.equal(plan.productionSalesQuantityLabel([invalidConversion]), '2 袋')
  assert.equal(plan.productionSalesQuantityLabel([invalidConversion], 'available'), '件数待确认')
  assert.equal(plan.productionSalesQuantityLabel([invalidConversion], 'gap'), '件数待确认')
  const legacy = demand({ bom_spec_id: 0, sales_unit: '', sales_spec_count: 0, need_units: 10, spec_g: 10, need_g: 100, blocking_reason: 'BOM 不可用' })
  assert.equal(plan.productionSalesQuantityLabel([legacy]), '件数待确认')
})

test('configuration reasons are visible without expanding orders and known BOM errors explain the remedy', async () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const start = source.indexOf('<table class="demand-table"')
  const end = source.indexOf('</table>', start) + '</table>'.length
  const { code, errors } = compileTemplate({ source: source.slice(start, end), filename: 'ProducePlanView.vue', id: 'blocking-reasons' })
  assert.deepEqual(errors, [])
  const js = code.replace(/import \{([^}]+)\} from "vue"/g, (_, names) => `const {${names.replace(/ as /g, ':')}} = Vue`).replace('export function render', 'function render')
  const render = new Function('Vue', js + '\nreturn render')(Vue)
  const reason = 'BOM 缺少物料明细，请完善对应版本后刷新需求。'
  const rows = [demand({ blocking_reason: reason }), demand({ selection_key: 'b', blocking_reason: reason })]
  const html = await renderToString(Vue.createSSRApp({ render, setup: () => ({ ...plan,
    stockInsufficientRows: rows, pagedDemandGroups: plan.groupProductionDemands(rows), collapsedDemandGroups: {}, selected: {},
    productionDemandSelectionKey: row => row.selection_key, toggleDemandGroup() {},
  }) }))
  const outsideOrderDetails = html.replace(/<details>[\s\S]*?<\/details>/g, '')
  assert.ok(outsideOrderDetails.includes(reason), 'blocking reason must be visible outside collapsed order details')
  assert.equal(outsideOrderDetails.split(reason).length - 1, 1, 'duplicate reasons should be combined per specification')
  assert.match(html, /aria-label="选择规格 [^"]+"[^>]*disabled/)
  for (const [raw, expected] of [
    ['BOM 配置待完善：default production BOM is no longer an active output BOM: 咖啡', '默认 BOM 已停用或不再产出此商品'],
    ['BOM 配置待完善：conflicting default production BOM configuration: 咖啡', '存在冲突的默认 BOM 配置'],
    ['BOM 配置待完善：BOM specification does not belong to an active BOM of the frozen parent product or its frozen version is unavailable: 咖啡', '订单锁定的 BOM 规格或版本不可用'],
  ]) {
    assert.ok(plan.productionDemandBlockingReasons([demand({ blocking_reason: raw })])[0].includes(expected))
  }
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
