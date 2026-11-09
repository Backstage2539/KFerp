import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, compileTemplate, parse } from '@vue/compiler-sfc'
import * as Vue from 'vue'
import { renderToString } from '@vue/server-renderer'
import { JSDOM } from 'jsdom'
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
        ? { ...Vue, onMounted() {}, onBeforeUnmount() {}, onActivated() {}, onDeactivated() {} }
        : binding.source.endsWith('.vue')
          ? { default: {} }
          : await import(binding.source.startsWith('.') ? new URL(binding.source.endsWith('.js') ? binding.source : `${binding.source}.js`, viewURL) : binding.source)
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
    state.rows.value = [
      demand({ selection_key: 'needs-production' }),
      demand({ selection_key: 'covered', gap_inventory_qty: 0, gap_sales_spec_count: 0 }),
      demand({ selection_key: 'blocked', blocking_reason: 'BOM 配置待完善：最新可用 BOM 版本未配置工艺路线' }),
    ]
    await Vue.nextTick()
    assert.deepEqual(state.stockInsufficientRows.value.map(row => row.selection_key), ['needs-production', 'blocked'])
    assert.deepEqual(plan.buildProductionGroupSelection(state.rows.value, {}, true), { 'needs-production': true })
  } finally {
    scope.stop()
  }
})

test('actual gap-review template renders a selection preview without a persisted draft', async () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const start = source.indexOf('<section v-if="currentPlanStepKey === \'reviewGap\'"')
  const end = source.indexOf('\n    <!-- Creation ends here;', start)
  const { code, errors } = compileTemplate({ source: source.slice(start, end), filename: 'ProducePlanView.vue', id: 'preview-regression' })
  assert.deepEqual(errors, [])
  const js = code.replace(/import \{([^}]+)\} from "vue"/g, (_, names) => `const {${names.replace(/ as /g, ':')}} = Vue`).replace('export function render', 'function render')
  const render = new Function('Vue', js + '\nreturn render')(Vue)
  for (const state of [
    { reviewProductRows: [{ key: 'p-1', product: '验收咖啡', spec_label: '227g', need_label: '2 袋', available_label: '0 袋', gap_label: '2 袋', order_count: 1, order_details: [{ order_id: 1, order_no: 'SO-1', customer_name: '验收客户', quantity: 2, sales_unit: '袋' }] }] },
    { previewLoading: true },
    { previewError: '预览失败，请重试' },
    {},
  ]) {
    const html = await renderToString(Vue.createSSRApp({ render, setup: () => ({
      currentPlanStepKey: 'reviewGap', reviewConclusion: '本次需要生产 1 个商品规格', reviewConclusionHint: '已核对库存和在产供应',
      reviewProductRows: [], visibleMaterialRows: [], materialFilterOptions: [], materialFilter: 'all',
      materialStatusCounts: { manufacture: 0, purchase: 0, waiting: 0, satisfied: 0 }, saving: false, previewLoading: false, previewError: '', loadSelectedPlanPreview() {}, returnToDemandSelection() {}, runPlanNextStep() {}, ...state,
    }) }))
    assert.match(html, /商品缺口/)
    assert.match(html, /生产用料/)
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
    filteredStockInsufficientRows: rows, demandPanelEmptyText: '暂无待计划需求',
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

test('demand table reserves readable exception and quantity columns at its minimum width', () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const { descriptor } = parse(source)
  const header = source.match(/<table class="demand-table"[^>]*>([\s\S]*?<\/thead>)/)[1]
  const dom = new JSDOM(`<style>${descriptor.styles.map(style => style.content).join('\n')}</style><div class="table-wrap"><table class="demand-table">${header}<tbody><tr>${'<td>20 pic</td>'.repeat(8)}<td class="demand-status-cell"><p class="demand-blocking-reason">BOM 配置待完善：最新可用 BOM 版本未配置工艺路线</p></td></tr></tbody></table></div>`)
  try {
    const style = selector => dom.window.getComputedStyle(dom.window.document.querySelector(selector))
    assert.ok(parseFloat(style('.demand-table th:nth-child(9)').width) >= 220, 'configuration messages need an explicit readable column width in the fixed table')
    assert.ok(parseFloat(style('.demand-table th:nth-child(8)').width) >= 80, 'gap quantities need space for both the number and unit')
    assert.ok(parseFloat(style('.demand-table').minWidth) >= 1100, 'the table must scroll instead of squeezing the final two columns')
    assert.equal(style('.demand-table td:nth-child(8)').whiteSpace, 'nowrap')
    assert.equal(style('.demand-blocking-reason').whiteSpace, 'normal')
    assert.equal(style('.table-wrap').overflow, 'auto')
  } finally {
    dom.window.close()
  }
})

test('production plan omits the stock-covered informational table', async () => {
  const source = fs.readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')
  const start = source.indexOf('<!-- Creation ends here;')
  const end = source.indexOf('<section v-if="planRecordsOpen"', start)
  assert.ok(start > 0 && end > start, 'locate the area between plan creation and saved records')
  const { code, errors } = compileTemplate({ source: source.slice(start, end), filename: 'ProducePlanView.vue', id: 'covered-demand-removal' })
  assert.deepEqual(errors, [])
  const js = code.replace(/import \{([^}]+)\} from "vue"/g, (_, names) => `const {${names.replace(/ as /g, ':')}} = Vue`).replace('export function render', 'function render')
  const render = new Function('Vue', js + '\nreturn render')(Vue)
  const html = await renderToString(Vue.createSSRApp({ render, setup: () => ({
    stockSufficientRows: [demand({ product: '库存覆盖商品', gap_inventory_qty: 0 })], rowKey: row => row.selection_key,
  }) }))
  assert.doesNotMatch(html, /库存充足（只提示）|库存覆盖商品|库存充足商品横向滚动表格/)
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
    filteredStockInsufficientRows: rows, demandPanelEmptyText: '暂无待计划需求',
    productionDemandSelectionKey: row => row.selection_key, toggleDemandGroup() {},
  }) }))
  assert.match(html, /SO-17 · 验收客户 · 2 袋/)
  assert.deepEqual(plan.productionDemandSelectionState(rows, {a: true}), {checked: false, indeterminate: true, selectedCount: 1, total: 2})
  assert.equal(plan.productionDemandSelectionState(rows, {a: true,b: true}).checked, true)
  assert.equal(plan.productionDemandSelectionState(rows, {}).indeterminate, false)
})

// Mounted refresh and selection revalidation are covered by production-plan-unified-draft.test.js.
