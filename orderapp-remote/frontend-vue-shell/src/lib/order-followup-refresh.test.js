import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { rowUsesStaleBeanListPublication } from './order-entry.js'

test('current public release is compared with public releases when a customer default exists', () => {
  const options = [
    { id: 124, customer_id: 301, is_customer_owned: true, product_type_id: 1532, list_type: 'commercial', version_no: 'V3.0.5', published_at: '2026-09-07 21:51:40' },
    { id: 122, customer_id: 301, is_customer_owned: false, product_type_id: 1532, list_type: 'commercial', version_no: 'V3.0.22', published_at: '2026-09-07 17:42:03' },
    { id: 121, customer_id: 301, is_customer_owned: false, product_type_id: 1532, list_type: 'commercial', version_no: 'V3.0.21', published_at: '2026-08-27 23:19:00' },
  ]
  assert.equal(rowUsesStaleBeanListPublication({ product_id: 1063, bean_list_publication_id: 122 }, options), false)
  assert.equal(rowUsesStaleBeanListPublication({ product_id: 1063, bean_list_publication_id: 124 }, options), false)
  assert.equal(rowUsesStaleBeanListPublication({ product_id: 1063, bean_list_publication_id: 121 }, options), true)
})

test('saving an edited order reloads its source details after the list refresh', async () => {
  const source = readFileSync(new URL('../views/OrdersView.vue', import.meta.url), 'utf8')
  const body = source.split('async function handleOrderEditSaved(data = {}) {')[1].split('\nfunction handleTrackingExcelFile')[0].trim().slice(0, -1)
  const activeOrderDetail = { value: { id: 1611, quote_source_trace: [{ price_list_version: 'old' }] } }
  const rows = { value: [] }
  let requestedID = 0
  const run = new (Object.getPrototypeOf(async function(){}).constructor)('activeOrderDetail','activeOrderCopyID','load','rows','loadOrderDetail', body)
  await run(activeOrderDetail, () => 0, async () => { rows.value = [{ id: 1611, order_date: '2025-01-02' }] }, rows, async id => { requestedID = id; activeOrderDetail.value.quote_source_trace = [{ price_list_version: 'V3.0.5' }] })
  assert.equal(requestedID, 1611)
  assert.equal(activeOrderDetail.value.quote_source_trace[0].price_list_version, 'V3.0.5')
})
