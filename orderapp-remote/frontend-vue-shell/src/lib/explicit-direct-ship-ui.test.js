import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { ordinaryOrderVoidable } from './customer-stock-entry.js'

test('ERP三类订单统一进入显式订单处理，旧普通作废不再由页面调用', () => {
  const view = readFileSync(new URL('../views/OrdersView.vue', import.meta.url), 'utf8')
  assert.match(view, /<OrderResolutionButton/)
  assert.doesNotMatch(view, /<DirectShipCancelButton/)
  assert.match(view, /filter\(row => !row.is_void\)/)
})

test('普通批量失效保留普通订单、排除两种代发与已失效订单', () => {
  assert.deepEqual([{ id: 1 }, { id: 2, fulfillment_mode: 'stock' }, { id: 3, fulfillment_mode: 'on_demand' }, { id: 4, is_void: true }].filter(ordinaryOrderVoidable).map(row => row.id), [1])
})
