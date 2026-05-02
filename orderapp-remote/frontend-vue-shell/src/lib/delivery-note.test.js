import test from 'node:test'
import assert from 'node:assert/strict'
import { deliveryNotePageUrl, deliveryNoteDownloadUrl } from './delivery-note.js'
import { buildShareResourcePayload } from './external-share.js'

test('deliveryNotePageUrl keeps order id in vue shell', () => {
  assert.equal(deliveryNotePageUrl(12), '/vue-shell?view=deliveryNote&order_id=12')
})

test('deliveryNoteDownloadUrl points to latest pdf', () => {
  assert.equal(deliveryNoteDownloadUrl(12), '/orders/12/delivery-note-latest.pdf')
})

test('delivery note uses the same external share payload as sales documents', () => {
  assert.deepEqual(buildShareResourcePayload('delivery_note_pdf', 12), {
    resource_type: 'delivery_note_pdf',
    order_id: 12,
    latest: true,
  })
})
