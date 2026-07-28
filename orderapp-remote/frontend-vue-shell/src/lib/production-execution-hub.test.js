import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

import {
  buildExecutionHubActions,
  buildExecutionHubFocus,
  executionHubTimelineFilters,
  filterExecutionHubTimeline,
  inventoryUnitWeightInGrams,
  productionStockDocumentPreviewAction,
  productionContextParams,
  readinessBadgeTone,
  stockCanonicalQuantity,
  stockDocumentPositiveItems,
  stockQuantityUsesCount,
} from './production-execution-hub.js'

test('execution hub actions carry production context to work order, WIP, job card, quality, costs and logs pages', () => {
  const hub = {
    work_order: { id: 88, work_order_no: 'WO-00088', running_item_id: 99, batch_id: 'BATCH-WO-88' },
    job_cards: [{ id: 91, work_order_id: 88, status: 'running' }],
    readiness: {
      suggested_action: 'open_wip_issue',
      related_links: [{ key: 'wip', label: '处理 WIP', view: 'stockOperations', params: { tab: 'wip', shortage_g: 1200 } }],
    },
  }
  const actions = buildExecutionHubActions(hub).map((action) => [action.key, action.label, action.view, action.params])

  assert.deepEqual(actions.slice(0, 10), [
    ['startProduction', '开始生产', 'workOrders', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['productionIssue', '生产领料', 'stockOperations', { tab: 'stockEntries', action: 'issue', return_source: 'work_order', work_order_id: 88, work_order_no: 'WO-00088', batch_id: 'BATCH-WO-88' }],
    ['productionSupplement', '补料', 'stockOperations', { tab: 'stockEntries', action: 'supplement', return_source: 'work_order', work_order_id: 88, work_order_no: 'WO-00088', batch_id: 'BATCH-WO-88' }],
    ['productionReturn', '退回未用原料', 'stockOperations', { tab: 'stockEntries', action: 'return', return_source: 'work_order', work_order_id: 88, work_order_no: 'WO-00088', batch_id: 'BATCH-WO-88' }],
    ['productionConsume', '记录生产消耗', 'stockOperations', { tab: 'stockEntries', action: 'consume', return_source: 'work_order', work_order_id: 88, work_order_no: 'WO-00088', batch_id: 'BATCH-WO-88' }],
    ['finishedReceipt', '完工入库', 'stockOperations', { tab: 'stockEntries', action: 'finish', return_source: 'work_order', work_order_id: 88, work_order_no: 'WO-00088', batch_id: 'BATCH-WO-88' }],
    ['openJobCard', '打开工序卡', 'jobCards', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['openQuality', '打开质检', 'qualityInspections', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88', reference_no: 'WO-00088' }],
    ['openCost', '成本', 'productionCosts', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
    ['openLogs', '日志', 'produceLogs', { work_order_id: 88, job_card_id: 91, running_item_id: 99, batch_id: 'BATCH-WO-88' }],
  ])
})

test('execution hub readiness and timeline helpers expose filters and focused areas', () => {
  assert.equal(readinessBadgeTone({ severity: 'blocked' }), 'danger')
  assert.equal(readinessBadgeTone({ severity: 'warning' }), 'warning')
  assert.equal(readinessBadgeTone({ can_start: true }), 'success')
  assert.deepEqual(executionHubTimelineFilters().map((item) => item.key), ['all', 'operation', 'inventory', 'quality', 'cost', 'log'])

  const timeline = [
    { type: 'operation', title: '工序开始' },
    { type: 'inventory', title: '领料' },
    { type: 'quality', title: '质检冻结' },
  ]
  assert.deepEqual(filterExecutionHubTimeline(timeline, 'quality').map((item) => item.title), ['质检冻结'])
  assert.equal(buildExecutionHubFocus({ focus: 'blocked' }).section, 'readiness')
  assert.equal(buildExecutionHubFocus({ job_card_id: 91 }).section, 'job_card')
})

test('production context params preserve work order, job card, running item, material and shortage context', () => {
  assert.deepEqual(productionContextParams({
    work_order_id: '88',
    work_order_no: 'WO-00088',
    job_card_id: '91',
    running_item_id: '99',
    material_id: '12',
    shortage_g: '1200',
    batch_id: 'BATCH-WO-88',
    tab: 'stockEntries',
    action: 'issue',
    return_source: 'work_order',
    ignored: 'x',
  }), {
    tab: 'stockEntries',
    action: 'issue',
    return_source: 'work_order',
    work_order_id: 88,
    work_order_no: 'WO-00088',
    job_card_id: 91,
    running_item_id: 99,
    material_id: 12,
    shortage_g: 1200,
    batch_id: 'BATCH-WO-88',
  })
})

test('production pages mount the shared execution hub drawer instead of separate work order drawers', () => {
  const files = [
    'src/views/ProductionOverviewView.vue',
    'src/views/WorkstationView.vue',
    'src/views/WorkOrdersView.vue',
    'src/views/JobCardsView.vue',
  ]
  for (const file of files) {
    const source = fs.readFileSync(new URL(`../${file.replace(/^src\//, '')}`, import.meta.url), 'utf8')
    assert.match(source, /ProductionExecutionHubDrawer/, `${file} should use shared execution hub drawer`)
  }

  const appSource = fs.readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  for (const key of ['work_order_id', 'work_order_no', 'job_card_id', 'running_item_id', 'material_id', 'shortage_g', 'reference_no', 'focus', 'batch_id', 'action', 'return_source']) {
    assert.match(appSource, new RegExp(`'${key}'`), `App.vue should preserve ${key} in view params`)
  }
})

test('execution hub and stock-entry UI expose WIP shortage detail and business-facing production issue fields', () => {
  const drawerSource = fs.readFileSync(new URL('../components/ProductionExecutionHubDrawer.vue', import.meta.url), 'utf8')
  assert.match(drawerSource, /WIP库存不足/)
  assert.match(drawerSource, /wipStatus\.materials/)
  for (const field of ['required_qty', 'available_qty', 'shortage_qty', 'inventory_unit']) {
    assert.match(drawerSource, new RegExp(field), `execution hub should display ${field}`)
  }
  assert.match(drawerSource, /productionIssue/)

  const operationsSource = fs.readFileSync(new URL('../views/StockOperationsView.vue', import.meta.url), 'utf8')
  assert.match(operationsSource, /工单号：/)
  assert.doesNotMatch(operationsSource, /工单 #\$\{params\.work_order_id\}/)
  assert.doesNotMatch(operationsSource, /生产中 #/)

  const entrySource = fs.readFileSync(new URL('../views/StockEntriesView.vue', import.meta.url), 'utf8')
  assert.match(entrySource, /row\.work_order_no/)
  assert.match(entrySource, /工单号：\{\{ form\.work_order_no/)
  assert.doesNotMatch(entrySource, /<span>工序卡<\/span><input/)
  assert.doesNotMatch(entrySource, /<span>生产中<\/span><input/)
  assert.match(entrySource, /v-model="form\.purpose_key" :disabled="!isDraft \|\| isBoundProductionDocument"/)
  assert.match(entrySource, /v-if="usesSingleQuantity\(item\)"/)
  assert.match(entrySource, /v-model\.number="item\.quantity"/)
  assert.match(entrySource, /item\.inventory_unit \|\| '-'/)
  assert.match(entrySource, /v-if="!usesSingleQuantity\(item\)"><span>数量\(g\)/)
  assert.match(entrySource, /v-if="!usesSingleQuantity\(item\)"><span>数量\(件\)/)
  assert.match(entrySource, /v-if="isReceipt"[^>]*><span>单位成本/)
  assert.doesNotMatch(entrySource, /material_id: Number\(params\.material_id/)
  assert.doesNotMatch(entrySource, /running_item_id: Number\(params\.running_item_id/)
  assert.doesNotMatch(entrySource, /form\.items\[0\]\.qty_g = Number\(params\.shortage_g/)
})

test('stock issue quantity maps inventory units to canonical fields without parsing the product specification label', () => {
  assert.equal(inventoryUnitWeightInGrams('kg'), 1000)
  assert.equal(inventoryUnitWeightInGrams('公斤'), 1000)
  assert.equal(inventoryUnitWeightInGrams('lb'), 453.59237)
  assert.equal(inventoryUnitWeightInGrams('磅'), 453.59237)
  assert.deepEqual(stockCanonicalQuantity({ quantity: 7.751, quantity_basis: 'weight', inventory_unit: 'kg', spec_label: '任意名称' }), {
    qty_g: 7751,
    qty_units: 0,
  })
  assert.deepEqual(stockCanonicalQuantity({ quantity: 12, quantity_basis: 'count', inventory_unit: '袋', spec_label: '454g' }), {
    qty_g: 0,
    qty_units: 12,
  })
  assert.throws(
    () => stockCanonicalQuantity({ quantity: 1.6, quantity_basis: 'count', inventory_unit: '袋' }),
    /计数物料数量必须为整数/,
  )
  assert.equal(stockQuantityUsesCount({ inventory_unit: '箱', qty_units: 60, qty_g: 0 }), true)
  assert.deepEqual(stockCanonicalQuantity({ quantity: 60, inventory_unit: '箱', qty_units: 60, qty_g: 0 }), {
    qty_g: 0,
    qty_units: 60,
  })
  assert.equal(stockQuantityUsesCount({ inventory_unit: '自定义重量', qty_g: 60000, qty_units: 0 }), false)
})

test('stock document payload omits zero rows but still validates every positive row', () => {
  const rows = stockDocumentPositiveItems(
    [{ name: '已覆盖', quantity: 0 }, { name: '待领用', quantity: 60 }],
    (item) => ({ qty_g: item.quantity * 1000, qty_units: 0 }),
  )
  assert.deepEqual(rows, [{
    item: { name: '待领用', quantity: 60 },
    quantity: { qty_g: 60000, qty_units: 0 },
  }])
  assert.throws(
    () => stockDocumentPositiveItems([{ quantity: 0 }], () => ({ qty_g: 0, qty_units: 0 })),
    /至少填写一个大于 0 的领用数量/,
  )
  assert.throws(
    () => stockDocumentPositiveItems([{ quantity: 1 }], () => { throw new Error('物料缺少库存单位换算') }),
    /缺少库存单位换算/,
  )
})

test('production stock document drafts map to the matching work-order preview action', () => {
  assert.equal(productionStockDocumentPreviewAction({ purpose: 'material_transfer_for_manufacture' }), 'issue')
  assert.equal(productionStockDocumentPreviewAction({ purpose: 'material_transfer_for_manufacture', is_return: true }), 'return')
  assert.equal(productionStockDocumentPreviewAction({ entry_type: 'material_return_from_manufacture' }), 'return')
  assert.equal(productionStockDocumentPreviewAction({ purpose: 'material_consumption_for_manufacture' }), 'consume')
  assert.equal(productionStockDocumentPreviewAction({ purpose: 'manufacture' }), 'finish')
  assert.equal(productionStockDocumentPreviewAction({ purpose: 'material_receipt' }), '')
})
