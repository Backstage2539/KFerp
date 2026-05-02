import assert from 'node:assert/strict'
import test from 'node:test'
import {
  qualityTargetAPIPath,
  qualityTargetFromRow,
  qualityTargetStatus,
  qualityTargetTabs,
} from './quality-inspections.js'

test('qualityTargetTabs exposes work order raw material and finished product drawers', () => {
  assert.deepEqual(qualityTargetTabs.map((tab) => tab.scope), ['work_order', 'raw_material', 'finished_batch'])
  assert.deepEqual(qualityTargetTabs.map((tab) => tab.label), ['工单质检', '原料质检', '产品质检'])
})

test('qualityTargetAPIPath maps every drawer tab to its API source', () => {
  assert.equal(qualityTargetAPIPath('work_order'), '/api/produce/work-orders?limit=100')
  assert.equal(qualityTargetAPIPath('raw_material'), '/api/stock/material-batches?active_only=1&limit=100')
  assert.equal(qualityTargetAPIPath('finished_batch'), '/api/stock/batches?item_type=finished_product&limit=100')
})

test('qualityTargetFromRow fills the quality form from selected target rows', () => {
  assert.deepEqual(qualityTargetFromRow('work_order', {
    work_order_no: 'WO-0000000020',
    product_name: '测试拼配',
  }), {
    scope: 'work_order',
    reference_type: 'work_order',
    reference_no: 'WO-0000000020',
    item_name: '测试拼配',
  })

  assert.deepEqual(qualityTargetFromRow('raw_material', {
    batch_code: 'MB-0000000007',
    material_name: '孟连水洗5T批次',
  }), {
    scope: 'raw_material',
    reference_type: 'raw_material',
    reference_no: 'MB-0000000007',
    item_name: '孟连水洗5T批次',
  })

  assert.deepEqual(qualityTargetFromRow('finished_batch', {
    batch_code: 'FP-0000000042',
    item_name: '耶加雪菲 227g',
  }), {
    scope: 'finished_batch',
    reference_type: 'finished_batch',
    reference_no: 'FP-0000000042',
    item_name: '耶加雪菲 227g',
  })
})

test('qualityTargetStatus keeps quality state visible in target lists', () => {
  assert.equal(qualityTargetStatus({ quality_status: 'reject' }), 'reject')
  assert.equal(qualityTargetStatus({}), 'unchecked')
})
