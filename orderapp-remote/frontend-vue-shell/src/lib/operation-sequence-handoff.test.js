import assert from 'node:assert/strict'
import { test } from 'node:test'
import * as workstation from './production-workstation.js'

test('downstream quantity shows actual predecessor output, not a planned minimum', () => {
  const task = { operation: '色选', inventory_unit: 'kg', planned_input_qty: 11627, planned_input_inventory_qty: 11.627, operation_total_input_qty: 11627, planned_output_inventory_qty: 9.534, has_predecessor: true, predecessor_completed: true, predecessor_output_qty: 11.2, predecessor_output_unit:'kg', predecessor_output_unit_state:'known' }
  assert.deepEqual(workstation.taskQuantityLines(task), ['前序实际产出 11.2 kg', '本工序按实际交接量处理'])
  assert.deepEqual(workstation.taskQuantityLines({ ...task, predecessor_completed: false }), ['前序工序未完成，完成后交接'])
  assert.deepEqual(workstation.taskQuantityLines({ ...task, predecessor_output_qty: 0 }), ['前序实际产出 0 kg', '本工序按实际交接量处理'])
})

test('completion defaults to actual upstream output only for a single downstream batch', () => {
  const task = { has_predecessor: true, predecessor_completed: true, predecessor_output_qty: 11.2, planned_input_inventory_qty: 11.627, batch_count: 1 }
  assert.equal(workstation.taskDefaultActualInput(task), 11.2)
  assert.equal(workstation.taskDefaultActualInput({ ...task, actual_input_qty: 10.8 }), 10.8)
  assert.equal(workstation.taskDefaultActualInput({ ...task, predecessor_output_qty: 0 }), 0)
  // Multiple tasks must not each prefill the whole upstream quantity.
  assert.equal(workstation.taskDefaultActualInput({ ...task, batch_count: 2 }), 0)
  assert.equal(workstation.taskDefaultActualInput({ planned_input_inventory_qty: 13 }), 13)
})

test('predecessor output uses event unit rather than packaged output unit', () => {
 const task={has_predecessor:true,predecessor_completed:true,predecessor_output_qty:83,inventory_unit:'袋',predecessor_output_unit:'kg',predecessor_output_unit_state:'known'}
 assert.deepEqual(workstation.taskQuantityLines(task),['前序实际产出 83 kg','本工序按实际交接量处理'])
 assert.deepEqual(workstation.taskQuantityLines({...task,predecessor_output_unit:'',predecessor_output_unit_state:'missing'}),['前序实际产出单位待核对','本工序按实际交接量处理'])
 assert.deepEqual(workstation.taskQuantityLines({...task,predecessor_output_unit:'',predecessor_output_unit_state:'mixed'}),['前序产出单位不一致，请核对','本工序按实际交接量处理'])
})
