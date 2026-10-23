import assert from 'node:assert/strict'
import { test } from 'node:test'
import { issueSelectionState, toggleIssueSelection, batchIssuePayload, batchIssueQuantity, mergeBatchIssueGroups } from './workstation-batch-issue.js'

test('selection uses one tri-state checkbox and ignores ineligible tasks', () => {
  const tasks = [{ job_card_id: 1, stock_action: 'issue' }, { job_card_id: 2, stock_action: 'issue' }, { job_card_id: 3 }]
  assert.deepEqual(issueSelectionState(tasks, [1]), { checked: false, indeterminate: true, count: 1, total: 2 })
  assert.deepEqual(toggleIssueSelection(tasks, [1]), [1, 2])
  assert.deepEqual(toggleIssueSelection(tasks, [1, 2]), [])
})

test('same material across source warehouses stays one editable row, with priority allocation and separate posting versions', () => {
  const base = { item_type: 'material', item_id: 3, bom_spec_id: 0, bom_variant_id: 0, spec_g: 0, owner_customer_id: 0, unit: '个', quantity_basis: 'count', to_warehouse: 'wip', name: '袋', wip: 0 }
  const groups = [
    { ...base, key: 'a', version: 'av', from_warehouse: 'raw_materials', required: 20, remaining: 20, available: 20, max_quantity: 30, allocations: [{ job_card_id: 2, priority: 1, planned_start: '2026-10-23', available: 20, extra_available: 10 }] },
    { ...base, key: 'b', version: 'bv', from_warehouse: 'finished_goods', required: 10, remaining: 10, available: 10, max_quantity: 50, allocations: [{ job_card_id: 1, priority: 2, planned_start: '2026-10-23', available: 10, extra_available: 40 }] },
    { ...base, key: 'owner', version: 'ov', owner_customer_id: 77, required: 2, remaining: 2, available: 2, max_quantity: 2, allocations: [] },
  ]
  const merged = mergeBatchIssueGroups(groups)
  assert.equal(merged.length, 2)
  const group = merged[0]
  assert.equal(group.remaining, 30)
  assert.equal(group.max_quantity, 80)
  assert.equal(group.allocations[0].from_warehouse, 'finished_goods')
  const preview = { job_card_ids: [1, 2], version: 'v', groups: merged }
  const payload = quantity => batchIssuePayload(preview, [group.key], { [group.key]: quantity }, 'r').groups
  assert.deepEqual(payload(15), [{ key: 'a', version: 'av', quantity: 5 }, { key: 'b', version: 'bv', quantity: 10 }])
  assert.deepEqual(payload(45), [{ key: 'a', version: 'av', quantity: 20 }, { key: 'b', version: 'bv', quantity: 25 }])
  assert.deepEqual(payload(5), [{ key: 'b', version: 'bv', quantity: 5 }])
})

test('submission uses selected groups and actual quantities, preserves request id on retry', () => {
  const preview = { job_card_ids: [1, 2], version: 'v', groups: [{ key: 'a', version: 'av' }, { key: 'b', version: 'bv' }] }
  assert.deepEqual(batchIssuePayload(preview, ['a'], { a: 60, b: 7 }, 'retry-id'), {
    job_card_ids: [1, 2], preview_version: 'v', request_id: 'retry-id', groups: [{ key: 'a', version: 'av', quantity: 60 }],
  })
  assert.equal(batchIssueQuantity({ unit: 'kg', quantity_basis: 'weight' }, 15570), 15.57)
  assert.equal(batchIssueQuantity({ unit: '个', quantity_basis: 'count' }, 15), 15)
})
