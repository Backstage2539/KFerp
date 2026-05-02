import test from 'node:test'
import assert from 'node:assert/strict'
import { salesOrderPageUrl, salesOrderDownloadUrl, salesOrderImageDownloadUrl } from './sales-order.js'
import { beginSalesOrderSealDrag, moveSalesOrderSealDrag } from './sales-order-seal.js'

test('salesOrderPageUrl keeps order id in vue shell', () => {
  assert.equal(salesOrderPageUrl(12), '/vue-shell?view=salesOrder&order_id=12')
})

test('salesOrderDownloadUrl points to latest pdf', () => {
  assert.equal(salesOrderDownloadUrl(12), '/orders/12/sales-order-latest.pdf')
})

test('salesOrderImageDownloadUrl points to latest png image', () => {
  assert.equal(salesOrderImageDownloadUrl(12), '/orders/12/sales-order-image-latest.png')
})

test('sales order seal drag keeps the clicked offset instead of snapping the seal away', () => {
  const drag = beginSalesOrderSealDrag({
    seal: { x_mm: 32, y_mm: 22, width_mm: 42 },
    clientX: 146.3,
    clientY: 83.4,
    scale: 2.2,
  })

  assert.deepEqual(moveSalesOrderSealDrag(drag, { clientX: 146.3, clientY: 83.4 }), {
    x_mm: 32,
    y_mm: 22,
    width_mm: 42,
  })
  assert.deepEqual(moveSalesOrderSealDrag(drag, { clientX: 168.3, clientY: 105.4 }), {
    x_mm: 42,
    y_mm: 32,
    width_mm: 42,
  })
})
