import test from 'node:test'
import assert from 'node:assert/strict'

import {
  activeCustomerFulfillmentCustomers,
  buildImportPreviewEffects,
  customerFulfillmentCustomerOptionLabel,
  customerFulfillmentCustomerOptionMeta,
  groupInvalidImportRows,
  importSummaryCards,
  importTypeOptions,
  latestParsedBatchForType,
  rowStatusLabel,
} from './customer-fulfillment.js'

test('importTypeOptions returns the three customer fulfillment workbook types', () => {
  assert.deepEqual(importTypeOptions(), [
    { value: 'processing_workbook', label: '代加工工单' },
    { value: 'direct_ship_workbook', label: '代发清单' },
    { value: 'settlement_workbook', label: '结算单' },
  ])
})

test('importSummaryCards includes only relevant import counters', () => {
  const cards = importSummaryCards({
    valid_rows: 7,
    invalid_rows: 1,
    direct_ship_orders: 2,
    processing_orders: 3,
    fee_items: 4,
  })
  assert.deepEqual(cards.map((card) => [card.label, card.value]), [
    ['有效行', 7],
    ['错误行', 1],
    ['代发订单', 2],
    ['加工工单', 3],
    ['费用明细', 4],
  ])
  assert.deepEqual(importSummaryCards({ valid_rows: 1, invalid_rows: 0 }).map((card) => card.label), ['有效行'])
})

test('rowStatusLabel maps invalid rows to Chinese operation labels', () => {
  assert.equal(rowStatusLabel('invalid'), '错误')
  assert.equal(rowStatusLabel('applied'), '已应用')
  assert.equal(rowStatusLabel('valid'), '有效')
})

test('customer fulfillment customer selector labels active customer options for humans', () => {
  const rows = activeCustomerFulfillmentCustomers({
    customers: [
      { id: 147, name: '誉观山', company_name: '誉观山咖啡', contact: '王总', phone: '13800138075', active: true },
      { id: 148, name: '停用客户', active: false },
    ],
  })

  assert.equal(rows.length, 1)
  assert.equal(customerFulfillmentCustomerOptionLabel(rows[0]), '誉观山')
  assert.equal(customerFulfillmentCustomerOptionMeta(rows[0]), '誉观山咖啡 / 王总 / 13800138075')
})

test('latestParsedBatchForType chooses the newest parsed batch for the selected workbook type', () => {
  const batches = [
    { id: 3, import_type: 'settlement_workbook', status: 'parsed', source_filename: '结算单.xlsx' },
    { id: 2, import_type: 'direct_ship_workbook', status: 'parsed', source_filename: '代发.xlsx' },
    { id: 1, import_type: 'processing_workbook', status: 'parsed', source_filename: '工单.xlsx' },
  ]

  assert.equal(latestParsedBatchForType(batches, null, 'processing_workbook')?.id, 1)
  assert.equal(latestParsedBatchForType(batches, { id: 9, import_type: 'processing_workbook', status: 'parsed' }, 'processing_workbook')?.id, 9)
  assert.equal(latestParsedBatchForType(batches, { id: 10, import_type: 'settlement_workbook', status: 'parsed' }, 'processing_workbook')?.id, 1)
  assert.equal(latestParsedBatchForType([{ id: 4, import_type: 'processing_workbook', status: 'applied' }], null, 'processing_workbook'), null)
})

test('groupInvalidImportRows summarizes validation errors for operators', () => {
  const rows = [
    { sheet_name: '生产工单', row_type: 'processing_work_order', error: '投豆量无效' },
    { sheet_name: '生产工单', row_type: 'processing_work_order', error: '投豆量无效' },
    { sheet_name: 'SKU', row_type: 'customer_sku', error: '产品名为空' },
  ]

  assert.deepEqual(groupInvalidImportRows(rows), [
    { key: '生产工单|processing_work_order|投豆量无效', sheet_name: '生产工单', row_type: 'processing_work_order', error: '投豆量无效', count: 2 },
    { key: 'SKU|customer_sku|产品名为空', sheet_name: 'SKU', row_type: 'customer_sku', error: '产品名为空', count: 1 },
  ])
})

test('buildImportPreviewEffects converts batch summary into apply preview counters', () => {
  const effects = buildImportPreviewEffects({
    valid_rows: 44,
    invalid_rows: 194,
    processing_orders: 136,
    direct_ship_orders: 0,
    fee_items: 2,
  })

  assert.deepEqual(effects, [
    { label: '将应用有效行', value: 44 },
    { label: '需先处理错误行', value: 194 },
    { label: '加工工单', value: 136 },
    { label: '费用明细', value: 2 },
  ])
})
