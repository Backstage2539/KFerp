import test from 'node:test'
import assert from 'node:assert/strict'

import {
  importSummaryCards,
  importTypeOptions,
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
