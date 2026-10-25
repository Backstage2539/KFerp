import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'

const read = path => fs.readFileSync(new URL(path, import.meta.url), 'utf8')

test('all production warehouse pickers use the registered stock warehouse endpoint', () => {
  const sources = [
    '../views/ProcessTemplatesView.vue',
    '../components/WorkOrderReceiptForm.vue',
    '../components/OperationConversionForm.vue',
    '../components/OperationMaterialConfig.vue',
  ].map(read)
  for (const source of sources) {
    assert.match(source, /\/api\/stock\/warehouses/)
    assert.doesNotMatch(source, /\/api\/warehouses/)
  }
})

test('work order actions are exposed without print or more menu', () => {
  const source = read('../views/WorkOrdersView.vue')
  const template = source.slice(0, source.indexOf('<script setup>'))
  const actions = template.slice(template.indexOf('<td class="row-actions"'), template.indexOf('</td>', template.indexOf('<td class="row-actions"')))
  for (const label of ['编辑拆分', '取消工单']) assert.match(actions, new RegExp(label))
  for (const obsolete of ['更多', '打印', 'print-sheet']) assert.doesNotMatch(template, new RegExp(obsolete))
})

test('completed old work orders and actionable work orders stay distinct in the primary action', () => {
  const source = read('../views/WorkOrdersView.vue')
  assert.match(source, /title: '确认数量并入库'/)
  assert.match(source, /job_card_id:.*workstation_id/)
  assert.match(source, /focus: 'workstation_task'/)
  assert.doesNotMatch(source, /生产记录与核对/)
})

test('linked replenishment previews the shortage and opens its created draft from the work-order list', () => {
  const workOrders = read('../views/WorkOrdersView.vue')
  const producePlan = read('../views/ProducePlanView.vue')
  for (const marker of [
    '关联补产核对',
    '/api/production/work-orders/replenishment/preview',
    '/api/production/work-orders/replenishment',
    '刷新缺口',
    'expected_version: state.preview.version',
    'request_id: state.requestID',
    "key: 'producePlan'",
    'workOrderLinkedReplenishmentSources(row)',
  ]) assert.ok(workOrders.includes(marker), `missing linked replenishment UI contract: ${marker}`)
  assert.match(producePlan, /watch\(\(\) => Number\(props\.viewParams\?\.production_plan_id/)
})

test('warehouse location repair requires an audited preview, a reason and balanced physical-location totals', () => {
  const source = read('../views/WipMaterialsView.vue')
  for (const marker of [
    '核对库位',
    '/api/stock/material-batch-locations/repair-preview',
    '/api/stock/material-batch-locations/repair',
    'expected_version: preview.version',
    'request_id: requestID',
    'reason: repairReason.value',
    'repairTotalG.value !== Number(preview.remaining_g || 0)',
    'reservation_moves:',
    '不会搬动物料',
  ]) assert.ok(source.includes(marker), `missing location repair contract: ${marker}`)
})
