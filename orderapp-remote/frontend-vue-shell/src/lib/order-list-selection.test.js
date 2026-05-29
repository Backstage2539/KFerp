import test from 'node:test'
import assert from 'node:assert/strict'
import {
  orderListSelectionState,
  selectableOrderIDs,
  toggleOrderPageSelection,
} from './order-list-selection.js'

const rows = [
  { id: 1, order_no: 'SO-1' },
  { id: 2, order_no: 'SO-2', is_void: true },
  { id: 3, order_no: 'SO-3' },
]

test('selectableOrderIDs excludes voided orders and keeps current page order', () => {
  assert.deepEqual(selectableOrderIDs(rows), [1, 3])
})

test('orderListSelectionState exposes unchecked checked and indeterminate header states', () => {
  assert.deepEqual(orderListSelectionState(rows, []), {
    checked: false,
    indeterminate: false,
    selectableCount: 2,
    selectedCount: 0,
  })
  assert.deepEqual(orderListSelectionState(rows, [1]), {
    checked: false,
    indeterminate: true,
    selectableCount: 2,
    selectedCount: 1,
  })
  assert.deepEqual(orderListSelectionState(rows, [1, 3]), {
    checked: true,
    indeterminate: false,
    selectableCount: 2,
    selectedCount: 2,
  })
})

test('toggleOrderPageSelection selects all visible selectable rows or clears them while preserving off-page selections', () => {
  assert.deepEqual(toggleOrderPageSelection(rows, [99]), [99, 1, 3])
  assert.deepEqual(toggleOrderPageSelection(rows, [99, 1, 3]), [99])
})
