import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { ordinaryOrderVoidable } from './customer-stock-entry.js'

test('ERP新场景代发使用专用取消入口，批量普通作废排除代发', () => {
  const view = readFileSync(new URL('../views/OrdersView.vue', import.meta.url), 'utf8')
  assert.match(view, /<DirectShipCancelButton/)
  assert.match(view, /filter\(ordinaryOrderVoidable\)/)
})

test('普通批量失效保留普通订单、排除两种代发与已失效订单', () => {
  assert.deepEqual([{ id: 1 }, { id: 2, fulfillment_mode: 'stock' }, { id: 3, fulfillment_mode: 'on_demand' }, { id: 4, is_void: true }].filter(ordinaryOrderVoidable).map(row => row.id), [1])
})
