import test from 'node:test'
import assert from 'node:assert/strict'
import { orderConfirmationLabel, visibleOrderRefresh } from './order-confirmation.js'
test('the order keeps acceptance separate from production and shipping', () => {
  assert.equal(orderConfirmationLabel('pending'), '待确认')
  assert.equal(orderConfirmationLabel('accepted'), '已接单')
  assert.equal(orderConfirmationLabel('rejected'), '已拒绝')
  assert.equal(orderConfirmationLabel(''), '已接单')
})
test('visible refresh polls every fifteen seconds, catches failures, and cleans up', async () => {
  let callback, removed = 0, calls = 0
  const listeners = new Map()
  const doc = { hidden: false, addEventListener: (key, fn) => listeners.set(key, fn), removeEventListener: key => listeners.delete(key) }
  const stop = visibleOrderRefresh(async () => { calls++; throw new Error('offline') }, { document: doc, setInterval: (fn, ms) => { assert.equal(ms, 15000); callback = fn; return 1 }, clearInterval: () => removed++ })
  await callback(); assert.equal(calls, 1)
  doc.hidden = true; await callback(); assert.equal(calls, 1)
  doc.hidden = false; await listeners.get('visibilitychange')(); assert.equal(calls, 2)
  stop(); assert.equal(removed, 1); assert.equal(listeners.size, 0)
})
