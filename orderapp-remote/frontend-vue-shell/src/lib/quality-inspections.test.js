import assert from 'node:assert/strict'
import test from 'node:test'
import {
  qualityInspectionErrorMessage,
  qualityTargetActionLabel,
  qualityTargetAPIPath,
  qualityTargetDrawerTitle,
  qualityTargetFromRow,
  qualityTargetStatus,
  qualityTargetTabs,
  workOrderQualityStatusLabel,
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

test('quality target drawer labels match the active inspection scope', () => {
  assert.equal(qualityTargetActionLabel('work_order'), '选择工单')
  assert.equal(qualityTargetActionLabel('raw_material'), '选择原料批次')
  assert.equal(qualityTargetActionLabel('finished_batch'), '选择产品批次')

  assert.equal(qualityTargetDrawerTitle('work_order'), '选择工单')
  assert.equal(qualityTargetDrawerTitle('raw_material'), '选择原料批次')
  assert.equal(qualityTargetDrawerTitle('finished_batch'), '选择产品批次')
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

test('work order target statuses and validation errors are localized for quality inspection', () => {
  assert.equal(workOrderQualityStatusLabel('released'), '未开工')
  assert.equal(workOrderQualityStatusLabel('partially_completed'), '部分完成')
  assert.equal(workOrderQualityStatusLabel('completed'), '已完成')

  assert.equal(
    qualityInspectionErrorMessage('scope, reference_no and result required'),
    '请先选择质检对象并填写检查结果',
  )
  assert.equal(qualityInspectionErrorMessage('network failed'), 'network failed')
})
