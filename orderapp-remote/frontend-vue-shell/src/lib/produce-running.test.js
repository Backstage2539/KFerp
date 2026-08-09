import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildFinishInput,
  buildFinishPanelModel,
  buildFinishPayload,
  formatActualYield,
  productionFinishErrorDetail,
} from './produce-running.js'

const row = {
  id: 9,
  spec_g: 454,
  input_g: 16000,
  bom_yield_rate: 0.5,
  plan_units: 28,
  plan_loose_g: 408,
}

test('buildFinishInput initializes editable production values from generated plan data', () => {
  const input = buildFinishInput(row)

  assert.deepEqual(input, {
    finished_units: 28,
    finished_loose_g: 408,
    consumed_input_g: 16000,
    partial: false,
    warehouse: 'finished_goods',
  })
  assert.equal(formatActualYield(row, input), '82.00%')
})

test('formatActualYield always derives actual yield from entered production quantities', () => {
  const input = buildFinishInput(row)
  input.finished_units = 27
  input.finished_loose_g = 200
  input.consumed_input_g = 15000

  assert.equal(formatActualYield(row, input), '83.05%')
})

test('formatActualYield does not fall back to legacy BOM expected yield when input is empty', () => {
  const input = buildFinishInput({ ...row, input_g: 0 })

  assert.equal(formatActualYield(row, input), '0.00%')
})

test('buildFinishPayload submits the same editable input and output values shown in the row', () => {
  const input = buildFinishInput(row)
  input.finished_units = 27
  input.finished_loose_g = 200
  input.consumed_input_g = 15000
  input.partial = true
  input.warehouse = 'finished_shop'

  assert.deepEqual(buildFinishPayload(row, input), {
    id: 9,
    finished_units: 27,
    finished_loose_g: 200,
    consumed_input_g: 15000,
    partial: true,
    warehouse: 'finished_shop',
  })
})

test('multi-spec running rows submit separate finished outputs and calculate combined yield', () => {
  const mergedRow = {
    id: 21,
    spec_g: 0,
    input_g: 16600,
    bom_yield_rate: 0.5,
    outputs: [
      { spec_g: 454, plan_units: 24, plan_loose_g: 0 },
      { spec_g: 227, plan_units: 2, plan_loose_g: 0 },
    ],
  }
  const input = buildFinishInput(mergedRow)

  assert.deepEqual(input.outputs, [
    { spec_g: 454, finished_units: 24, finished_loose_g: 0 },
    { spec_g: 227, finished_units: 2, finished_loose_g: 0 },
  ])
  assert.equal(formatActualYield(mergedRow, input), '68.37%')

  input.outputs[1].finished_units = 3
  assert.deepEqual(buildFinishPayload(mergedRow, input), {
    id: 21,
    finished_units: 0,
    finished_loose_g: 0,
    consumed_input_g: 16600,
    partial: false,
    warehouse: 'finished_goods',
    outputs: [
      { spec_g: 454, finished_units: 24, finished_loose_g: 0 },
      { spec_g: 227, finished_units: 3, finished_loose_g: 0 },
    ],
  })
})

test('finish panel model keeps full and partial completion in one operation surface', () => {
  const input = buildFinishInput(row)
  const full = buildFinishPanelModel(row, input, 'complete')
  const partial = buildFinishPanelModel(row, input, 'partial')

  assert.equal(full.title, '完成生产')
  assert.equal(full.primaryLabel, '完成并入库')
  assert.equal(full.payload.partial, false)
  assert.deepEqual(full.fields, ['投料', '成品件数', '余料', '入库仓', '异常/备注'])

  assert.equal(partial.title, '部分完成')
  assert.equal(partial.primaryLabel, '记录部分完成')
  assert.equal(partial.payload.partial, true)
  assert.deepEqual(partial.fields, ['投料', '成品件数', '余料', '入库仓', '异常/备注'])
})

test('finish error details separate reason, affected object, and next action', () => {
  assert.deepEqual(productionFinishErrorDetail('WIP stock insufficient: 耶加雪菲 need 900g, available 200g, reserved 0g'), {
    reason: 'WIP库存不足',
    affectedObject: '耶加雪菲 need 900g, available 200g, reserved 0g',
    action: '打开库存作业',
    actionKey: 'stockOperations',
  })

  assert.deepEqual(productionFinishErrorDetail('quality hold: MB-RAW-001 is held by QC'), {
    reason: '质检冻结',
    affectedObject: 'MB-RAW-001 is held by QC',
    action: '打开质检',
    actionKey: 'qualityInspections',
  })
})
