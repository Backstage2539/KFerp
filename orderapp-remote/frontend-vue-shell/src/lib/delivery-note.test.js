import test from 'node:test'
import assert from 'node:assert/strict'
import { deliveryNotePageUrl, deliveryNoteDownloadUrl } from './delivery-note.js'

test('deliveryNotePageUrl keeps order id in vue shell', () => {
  assert.equal(deliveryNotePageUrl(12), '/vue-shell?view=deliveryNote&order_id=12')
})

test('deliveryNoteDownloadUrl points to latest pdf', () => {
  assert.equal(deliveryNoteDownloadUrl(12), '/orders/12/delivery-note-latest.pdf')
})
