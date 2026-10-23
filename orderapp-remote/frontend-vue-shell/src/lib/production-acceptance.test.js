import test from 'node:test'
import assert from 'node:assert/strict'
import {
  completionNoteForWorkOrder,
  executeFinishedReceiptBatch,
  finishedReceiptBatchPlan,
  groupFinishedReceiptTasks,
} from './production-acceptance.js'

const row = (overrides = {}) => ({
  work_order_id: 1,
  work_order_no: 'WO-001',
  output_type: 'product',
  output_product_id: 21,
  output_material_id: 0,
  bom_spec_id: 8,
  spec_g: 454,
  output_unit: '袋',
  target_warehouse: 'virtual-customer-a',
  customer_id: 77,
  output_name: '日晒咖啡',
  reported_qty: 10,
  received_qty: 0,
  available_qty: 10,
  receipt_status: 'pending',
  quality_blocked: false,
  ...overrides,
})

test('finished receipts group only identical output, spec, unit, owner and target warehouse', () => {
  const groups = groupFinishedReceiptTasks([
    row(),
    row({ work_order_id: 2, work_order_no: 'WO-002', available_qty: 4, reported_qty: 4 }),
    row({ work_order_id: 3, output_product_id: 22 }),
    row({ work_order_id: 4, spec_g: 227 }),
    row({ work_order_id: 5, target_warehouse: 'finished_goods' }),
    row({ work_order_id: 6, output_unit: '盒' }),
    row({ work_order_id: 7, customer_id: 78 }),
  ], [{ code: 'virtual-customer-a', name: '客户甲成品仓' }])

  assert.equal(groups.length, 6)
  const merged = groups.find((item) => item.rows.length === 2)
  assert.equal(merged.available_qty, 14)
  assert.equal(merged.warehouse_name, '客户甲成品仓')
})

test('merged receipt plan posts each eligible work order quantity once and names its source', () => {
  const group = groupFinishedReceiptTasks([
    row({ available_qty: 3 }),
    row({ work_order_id: 2, work_order_no: 'WO-002', available_qty: 4 }),
    row({ work_order_id: 3, available_qty: 8, quality_blocked: true }),
    row({ work_order_id: 4, available_qty: 0 }),
  ])[0]

  assert.deepEqual(finishedReceiptBatchPlan(group), [
    { work_order_id: 1, work_order_no: 'WO-001', quantity: 3 },
    { work_order_id: 2, work_order_no: 'WO-002', quantity: 4 },
  ])
  assert.equal(completionNoteForWorkOrder(row()), '合并入库，来源工单：WO-001')
})

test('warehouse label falls back to the frozen-warehouse label instead of exposing an empty code', () => {
  const groups = groupFinishedReceiptTasks([row({ target_warehouse: '' })])

  assert.equal(groups[0].warehouse_name, '按工单冻结仓库')
  const unresolved = groupFinishedReceiptTasks([row({ target_warehouse: 'virtual-opaque' })])
  assert.equal(unresolved[0].warehouse_name, '仓库名称未加载')
  assert.doesNotMatch(unresolved[0].warehouse_name, /virtual-opaque/)
})

test('tasks without a stable product identity are not merged by a matching display name', () => {
  const groups = groupFinishedReceiptTasks([
    row({ work_order_id: 81, output_product_id: 0, product_id: 0, output_name: '未识别商品' }),
    row({ work_order_id: 82, output_product_id: 0, product_id: 0, output_name: '未识别商品' }),
  ])

  assert.equal(groups.length, 2)
})

test('merged completion continues after a work order fails and keeps per-order results', async () => {
  const group = groupFinishedReceiptTasks([
    row({ available_qty: 2 }),
    row({ work_order_id: 2, work_order_no: 'WO-002', available_qty: 3 }),
    row({ work_order_id: 3, work_order_no: 'WO-003', available_qty: 4 }),
  ])[0]
  const attempted = []
  const result = await executeFinishedReceiptBatch(group, group.processable_qty, async (source, item) => {
    attempted.push(source.work_order_no)
    if (source.work_order_id === 2) throw new Error('工单剩余件数不足')
    return { entry_no: `SE-${item.work_order_id}` }
  })

  assert.deepEqual(attempted, ['WO-001', 'WO-002', 'WO-003'])
  assert.deepEqual(result.succeeded.map((item) => item.result.entry_no), ['SE-1', 'SE-3'])
  assert.deepEqual(result.failed.map((item) => [item.work_order_no, item.reason]), [['WO-002', '工单剩余件数不足']])
})
