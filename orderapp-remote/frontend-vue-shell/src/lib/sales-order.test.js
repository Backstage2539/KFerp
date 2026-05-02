import test from 'node:test'
import assert from 'node:assert/strict'
import { salesOrderPageUrl, salesOrderDownloadUrl, salesOrderImageDownloadUrl } from './sales-order.js'

test('salesOrderPageUrl keeps order id in vue shell', () => {
  assert.equal(salesOrderPageUrl(12), '/vue-shell?view=salesOrder&order_id=12')
})

test('salesOrderDownloadUrl points to latest pdf', () => {
  assert.equal(salesOrderDownloadUrl(12), '/orders/12/sales-order-latest.pdf')
})

test('salesOrderImageDownloadUrl points to latest png image', () => {
  assert.equal(salesOrderImageDownloadUrl(12), '/orders/12/sales-order-image-latest.png')
})
