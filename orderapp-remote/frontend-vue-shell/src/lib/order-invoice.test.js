import test from 'node:test'
import assert from 'node:assert/strict'
import {
  invoiceStatusLabel,
  invoiceStatusTone,
  orderInvoiceAssetName,
  orderInvoiceFileAccept,
  orderInvoiceFileAllowed,
} from './order-invoice.js'

test('invoice status helpers expose human labels and tones', () => {
  assert.equal(invoiceStatusLabel('requested'), '已申请')
  assert.equal(invoiceStatusLabel('uploaded'), '已上传')
  assert.equal(invoiceStatusLabel(''), '未申请')
  assert.equal(invoiceStatusTone('uploaded'), 'ok')
  assert.equal(invoiceStatusTone('requested'), 'warn')
  assert.equal(invoiceStatusTone(''), 'muted')
})

test('invoice file selection accepts PDF and common image types only', () => {
  assert.equal(orderInvoiceFileAccept, '.pdf,image/png,image/jpeg,image/gif,image/webp')
  assert.equal(orderInvoiceFileAllowed({ name: 'invoice.pdf', type: 'application/pdf' }), true)
  assert.equal(orderInvoiceFileAllowed({ name: 'invoice.PNG', type: 'image/png' }), true)
  assert.equal(orderInvoiceFileAllowed({ name: 'invoice.jpg', type: 'image/jpeg' }), true)
  assert.equal(orderInvoiceFileAllowed({ name: 'invoice.txt', type: 'text/plain' }), false)
  assert.equal(orderInvoiceFileAllowed({ name: 'invoice.svg', type: 'image/svg+xml' }), false)
})

test('invoice asset display name falls back to URL and placeholder', () => {
  assert.equal(orderInvoiceAssetName({ filename: 'kf-invoice.pdf' }), 'kf-invoice.pdf')
  assert.equal(orderInvoiceAssetName({ url: '/assets/sales_order_assets/order_invoices/1/abc.png' }), 'abc.png')
  assert.equal(orderInvoiceAssetName(null), '暂无发票文件')
})
