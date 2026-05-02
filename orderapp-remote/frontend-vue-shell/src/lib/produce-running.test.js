import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildFinishInput,
  buildFinishPayload,
  formatActualYield,
  markYieldDirty,
} from './produce-running.js'

const row = {
  id: 9,
  spec_g: 454,
  input_g: 16000,
  bom_yield_rate: 0.82,
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
    yield_dirty: false,
  })
  assert.equal(formatActualYield(row, input), '82.00%')
})

test('formatActualYield recalculates only after production quantities are edited', () => {
  const input = buildFinishInput(row)
  input.finished_units = 27
  input.finished_loose_g = 200
  input.consumed_input_g = 15000

  assert.equal(formatActualYield(row, input), '82.00%')
  markYieldDirty(input)
  assert.equal(formatActualYield(row, input), '83.05%')
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
    bom_yield_rate: 0.82,
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
  assert.equal(formatActualYield(mergedRow, input), '82.00%')

  markYieldDirty(input)
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
