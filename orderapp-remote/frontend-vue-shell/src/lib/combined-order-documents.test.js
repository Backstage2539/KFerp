import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildCombinedDocumentQuery,
  combinedDocumentSelectionSummary,
  selectedOrdersShareSameCustomer,
} from './combined-order-documents.js'

test('selectedOrdersShareSameCustomer requires selected orders to belong to one customer', () => {
  const rows = [
    { id: 1, customer_id: 3, customer: '测试客户' },
    { id: 2, customer_id: 3, customer: '测试客户' },
    { id: 3, customer_id: 8, customer: '其他客户' },
  ]

  assert.equal(selectedOrdersShareSameCustomer([1, 2], rows), true)
  assert.equal(selectedOrdersShareSameCustomer([1, 3], rows), false)
  assert.equal(selectedOrdersShareSameCustomer([1], rows), false)
})

test('combinedDocumentSelectionSummary reports count and selected customer label', () => {
  const rows = [
    { id: 1, customer_id: 3, customer: '测试客户' },
    { id: 2, customer_id: 3, customer: '测试客户' },
  ]

  assert.deepEqual(combinedDocumentSelectionSummary([1, 2], rows), {
    count: 2,
    customerId: 3,
    customer: '测试客户',
    valid: true,
  })
})

test('buildCombinedDocumentQuery keeps selected order ids in the request', () => {
  assert.equal(buildCombinedDocumentQuery([2, 1]), 'order_ids=2%2C1')
  assert.equal(buildCombinedDocumentQuery([]), '')
})
