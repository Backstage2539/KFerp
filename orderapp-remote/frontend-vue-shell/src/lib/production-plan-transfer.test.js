import test from 'node:test'
import assert from 'node:assert/strict'
import { remainingTransfer, transferSelections, validateTransferSelections, replaceTransferSelection, pendingTransferAmount, reservedAmount, transferDocumentStatus, transferRowIssue } from './production-plan-transfer.js'

test('WIP shortage includes upstream demand and deducts pending documents exactly once', () => {
  const source = { unit: 'kg', demand_g: 69149, wip_covered_g: 15000 }
  assert.deepEqual(remainingTransfer(source, 5000), { missing: 54.149, pending: 5, remaining: 49.149, unit: 'kg' })
  assert.equal(remainingTransfer(source, 60000).remaining, 0)
})

test('upstream planned production is shown as WIP shortage but never as transferable quantity', () => {
  const source = { unit: 'kg', demand_g: 43584, required_g: 15000, upstream_g: 28584, wip_covered_g: 0 }
  assert.deepEqual(remainingTransfer(source, 1000), { missing: 43.584, pending: 1, remaining: 14, unit: 'kg' })
})

test('selected batches keep exact task, owner and gram quantities', () => {
  const source = { id: 12, unit: 'kg', demand_g: 6767, wip_covered_g: 0 }
  const selection = transferSelections(source, 'raw_materials', [
    { batch_code: 'A', owner_customer_id: 7, available_g: 5000, quantity: 4 },
    { batch_code: 'B', owner_customer_id: 7, available_g: 4000, quantity: 2.767 },
  ])
  assert.equal(selection.source_id, 12)
  assert.equal(selection.owner_customer_id, 7)
  assert.deepEqual(selection.batches.map(b => b.qty_g), [4000, 2767])
  assert.equal(validateTransferSelections(source, selection, 0), '')
})

test('kilogram inputs preserve exact whole grams while rejecting fractional grams', () => {
  const source = { id: 12, unit: 'kg', demand_g: 5000, wip_covered_g: 0 }
  const row = { batch_code: 'A', owner_customer_id: 0, available_g: 5000, quantity: 1.001 }
  const exact = transferSelections(source, 'raw', [row])
  assert.equal(exact.batches[0].qty_g, 1001)
  assert.equal(validateTransferSelections(source, exact, 0), '')
  row.quantity = 1.0001
  assert.match(validateTransferSelections(source, transferSelections(source, 'raw', [row]), 0), /精确到克/)
})

test('invalid or excessive quantities and mixed owners cannot be submitted', () => {
  const source = { id: 12, unit: 'kg', demand_g: 6767, wip_covered_g: 0 }
  const rows = [{ batch_code: 'A', owner_customer_id: 1, available_g: 5000, quantity: 5 }, { batch_code: 'B', owner_customer_id: 2, available_g: 4000, quantity: 2 }]
  assert.match(validateTransferSelections(source, transferSelections(source, 'raw', rows), 0), /货主/)
  rows[1].owner_customer_id = 1
  assert.match(validateTransferSelections(source, transferSelections(source, 'raw', rows), 0), /超过/)
  rows[0].quantity = -1
  assert.match(validateTransferSelections(source, transferSelections(source, 'raw', rows), 0), /数量/)
})

test('batch edit replaces only its selected entry and strips local input metadata', () => {
  const old = [{ source_id: 11, warehouse: 'a', batches: [] }, { source_id: 12, warehouse: 'a', batches: [] }]
  const updated = replaceTransferSelection(old, 1, { source_id: 12, warehouse: 'b', owner_customer_id: 3, batches: [{ batch_code: 'x', qty_g: 2000, qty_units: 0 }], _rows: [{ quantity: 2 }] })
  assert.equal(updated[0].warehouse, 'a')
  assert.equal(updated[1].warehouse, 'b')
  assert.equal(updated[1]._rows, undefined)
  assert.equal(old[1].warehouse, 'a')
})

test('pending quantities ignore cancelled and completed documents and stay bound to source', () => {
  const docs = [{ status: 'draft', source_id: 4, qty_g: 1000 }, { status: 'draft', source_id: 5, qty_g: 900 }, { status: 'cancelled', source_id: 4, qty_g: 500 }, { status: 'submitted', source_id: 4, qty_g: 1000 }]
  assert.equal(pendingTransferAmount(docs, { id: 4, unit: 'kg', demand_g: 5000 }), 1000)
})

test('unavailable inventory is not falsely displayed as reserved inventory', () => {
  const source = { id: 4, unit: 'kg', demand_g: 10000 }
  assert.equal(reservedAmount(source, { qty_g: 10000, available_g: 0, unavailable_reason: '质量冻结' }), null)
  assert.equal(reservedAmount(source, { qty_g: 10000, available_g: 4000, reserved_g: 1000 }), 1)
})

test('transfer document statuses are shown as warehouse workflow states', () => {
  assert.equal(transferDocumentStatus('draft'), '待仓库确认')
  assert.equal(transferDocumentStatus('submitted'), '已确认')
  assert.equal(transferDocumentStatus('cancelled'), '已取消')
})

test('product with frozen grams and count transfers integer pieces, never loose grams', () => {
  const source = { id: 15, component_type: 'product', component_spec_g: 454, unit: 'kg', demand_g: 4540, demand_units: 10, required_g: 4540, required_units: 10, wip_covered_g: 908, wip_covered_units: 2 }
  assert.deepEqual(remainingTransfer(source, 1), { missing: 8, pending: 1, remaining: 7, unit: '件' })
  const rows = [{ batch_code: 'A', owner_customer_id: 0, available_g: 50000, available_units: 6, quantity: 4 }]
  const selected = transferSelections(source, 'raw', rows)
  assert.deepEqual(selected.batches, [{ batch_code: 'A', qty_g: 0, qty_units: 4 }])
  assert.equal(validateTransferSelections(source, selected, 1), '')
  rows[0].quantity = 1.5
  assert.match(validateTransferSelections(source, transferSelections(source, 'raw', rows), 1), /整数/)
  rows[0].quantity = 1; rows[0].available_units = 0
  assert.match(validateTransferSelections(source, transferSelections(source, 'raw', rows), 1), /散装/)
})

test('loose product stock explains why it cannot satisfy piece transfers', () => {
  const source = { component_type: 'product', component_spec_g: 454, demand_units: 10, demand_g: 4540, unit: 'kg' }
  assert.match(transferRowIssue(source, { available_units: 0, available_g: 5000 }), /散装/)
  assert.equal(transferRowIssue(source, { available_units: 3, available_g: 5000 }), '')
  assert.equal(transferRowIssue(source, { available_units: 0, available_g: 5000, unavailable_reason: '质量冻结' }), '质量冻结')
})
