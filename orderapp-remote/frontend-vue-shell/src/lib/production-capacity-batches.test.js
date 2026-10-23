import test from 'node:test'
import assert from 'node:assert/strict'
import * as plan from './produce-plan.js'

const item = { id: 11, planned_g: 103797, spec_g: 1000, sales_spec_count: 103.797 }
const op = { seq: 1, operation_id: 9, operation: '烘焙' }
const cap = { id: 7, workstation_id: 2, workstation: '布勒', name: '20kg', batch_size_qty: 20, batch_size_unit: 'kg', standard_minutes: 25, hourly_rate: 102, applicable_operation_ids: [9], status: 'active' }
const row = { production_plan_item_id: 11, operation_seq: 1, operation_id: 9, workstation_capacity_id: 7, ...cap, planned_qty: 100, planned_batch_count: 5 }

test('integer batches drive quantities, time, cost and count-only wire payload', () => {
  const changed = plan.withCapacityBatchCount(row, 6)
  const metrics = plan.plannedCapacitySplitMetrics(changed)
  assert.equal(metrics.planned_qty, 120)
  assert.equal(metrics.planned_batch_count, 6)
  assert.equal(metrics.planned_minutes, 150)
  assert.equal(metrics.planned_operation_cost, 255)
  const payload = plan.buildProductionPlanOperationSplitPayload([changed]).items[0]
  assert.equal(payload.planned_batch_count, 6)
  assert.equal(Object.hasOwn(payload, 'planned_qty'), false)
  assert.ok(plan.productionPlanSplitBatchCards(changed).every(batch => batch.planned_qty === 20 && !batch.underfilled))
})

test('invalid batch input stays invalid and is never silently rounded or removed', () => {
  for (const input of ['', 0, -1, 1.5, 'abc', Infinity]) {
    const changed = plan.withCapacityBatchCount(row, input)
    assert.match(plan.capacitySplitInputError(changed), /正整数/)
    assert.throws(() => plan.buildProductionPlanOperationSplitPayload([changed]), /正整数/)
  }
})

test('take all rounds up in weight/count units and replaces only the same task and operation', () => {
  for (const [target, capacity, count, qty] of [
    [item, cap, 6, 120],
    [{ ...item, planned_g: 100000 }, cap, 5, 100],
    [{ ...item, planned_g: 400 }, { ...cap, batch_size_qty: 200, batch_size_unit: 'g' }, 2, 400],
    [{ ...item, sales_spec_count: 103 }, { ...cap, batch_size_qty: 20, batch_size_unit: '件', cost_method: 'piece', piece_rate: 0.2 }, 6, 120],
  ]) {
    const selected = { ...row, ...capacity, workstation_capacity_id: capacity.id }
    const otherTask = { ...row, production_plan_item_id: 12 }
    const otherOp = { ...row, operation_seq: 2 }
    const rows = plan.takeAllCapacityBatches([selected, { ...row, workstation_capacity_id: 8 }, otherTask, otherOp], selected, target)
    assert.equal(rows.length, 3)
    assert.ok(rows.includes(otherTask) && rows.includes(otherOp))
    const result = rows.find(split => split.production_plan_item_id === 11 && split.operation_seq === 1)
    assert.equal(result.planned_batch_count, count)
    assert.equal(result.planned_qty, qty)
  }
})

test('automatic integer splitting uses whole smallest capacity for the remainder', () => {
  const rows = plan.buildOperationCapacityAutoSplits(item, op, [cap, { ...cap, id: 8, batch_size_qty: 3 }, { ...cap, id: 9, batch_size_qty: 2 }], { wholeBatches: true })
  assert.deepEqual(rows.map(r => [r.workstation_capacity_id, r.planned_qty, r.planned_batch_count]), [[7,100,5],[8,3,1],[9,2,1]])
})

test('legacy partial rows stay unchanged until edited; capacity changes preserve batch count', () => {
  const legacy = { ...row, planned_qty: 3.797, planned_batch_count: 1 }
  assert.equal(plan.isLegacyPartialCapacitySplit(legacy), true)
  assert.equal(plan.buildProductionPlanOperationSplitPayload([legacy]).items[0].planned_qty, 3.797)
  assert.equal(plan.applyCapacityBatchSnapshot(legacy, { ...cap, name: '新名称', hourly_rate: 110 }, true).planned_qty, 3.797)
  const resized = plan.applyCapacityBatchSnapshot(legacy, { ...cap, batch_size_qty: 12 }, true)
  assert.equal(resized.planned_qty, 12)
  const switched = plan.applyCapacityBatchSnapshot(row, { ...cap, batch_size_qty: 12 })
  assert.equal(switched.planned_qty, 60)
  assert.equal(switched.planned_batch_count, 5)
})

test('repeated use of the same operation is isolated by frozen sequence', () => {
 assert.equal(plan.capacitySplitMatchesOperation({ operation_seq: 2, operation_id: 9 }, { seq: 1, operation_id: 9 }), false)
 assert.equal(plan.capacitySplitMatchesOperation({ operation_seq: 1, operation_id: 9 }, { seq: 1, operation_id: 9 }), true)
})

test('explicit edits use current capacity even when an old draft carries a frozen size', () => {
 const current = { ...cap, batch_size_qty: 12 }
 const edited = plan.withCapacityBatchCount(row, 6, current)
 assert.equal(edited.planned_qty, 72)
 const rows = plan.takeAllCapacityBatches([row], row, item, current)
 assert.equal(rows[0].planned_batch_count, 9)
 assert.equal(rows[0].planned_qty, 108)
 assert.equal(row.planned_qty, 100)
})

test('saved rows without operation id validate against the frozen task sequence', () => {
 const target = { ...item, process_snapshot_json: JSON.stringify({ operations: [op] }) }
 const saved = { ...row, operation_id: 0, operation: op.operation }
 assert.equal(plan.capacitySplitAvailabilityError(saved, target, { ...saved, seq: 1 }, [cap]), '')
})
