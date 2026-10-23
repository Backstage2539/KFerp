import assert from 'node:assert/strict'
import { test } from 'node:test'
import { issueSelectionState, toggleIssueSelection, batchIssuePayload, batchIssueQuantity } from './workstation-batch-issue.js'

test('selection uses one tri-state checkbox and ignores ineligible tasks', () => {
  const tasks = [{ job_card_id: 1, stock_action: 'issue' }, { job_card_id: 2, stock_action: 'issue' }, { job_card_id: 3 }]
  assert.deepEqual(issueSelectionState(tasks, [1]), { checked: false, indeterminate: true, count: 1, total: 2 })
  assert.deepEqual(toggleIssueSelection(tasks, [1]), [1, 2])
  assert.deepEqual(toggleIssueSelection(tasks, [1, 2]), [])
})

test('submission uses selected groups and actual quantities, preserves request id on retry', () => {
  const preview = { job_card_ids: [1, 2], version: 'v', groups: [{ key: 'a', version: 'av' }, { key: 'b', version: 'bv' }] }
  assert.deepEqual(batchIssuePayload(preview, ['a'], { a: 60, b: 7 }, 'retry-id'), {
    job_card_ids: [1, 2], preview_version: 'v', request_id: 'retry-id', groups: [{ key: 'a', version: 'av', quantity: 60 }],
  })
  assert.equal(batchIssueQuantity({ unit: 'kg', quantity_basis: 'weight' }, 15570), 15.57)
  assert.equal(batchIssueQuantity({ unit: '个', quantity_basis: 'count' }, 15), 15)
})
