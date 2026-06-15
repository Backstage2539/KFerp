import test from 'node:test'
import assert from 'node:assert/strict'

import { formatTrackingSummary, isOrderShipReady, normalizeTrackingInput } from './order-shipping.js'

test('normalizeTrackingInput keeps unique tracking numbers in entry order', () => {
  assert.deepEqual(normalizeTrackingInput(' SF123, SF124\nSF123；SF125 '), ['SF123', 'SF124', 'SF125'])
})

test('formatTrackingSummary makes multi tracking visible without hiding the first number', () => {
  assert.equal(formatTrackingSummary('SF123\nSF124\nSF125'), 'SF123 等 3 个单号')
  assert.equal(formatTrackingSummary('SF123'), 'SF123')
  assert.equal(formatTrackingSummary(''), '未回填')
})

test('isOrderShipReady excludes voided and already shipped orders', () => {
  assert.equal(isOrderShipReady({ process_status: '生产完成', ship_status: '未发货' }), true)
  assert.equal(isOrderShipReady({ process_status: '无需生产', ship_status: '待发货' }), true)
  assert.equal(isOrderShipReady({ process_status: '库存待发货', ship_status: '未发货' }), true)
  assert.equal(isOrderShipReady({ process_status: '生产完成', ship_status: '已发货' }), false)
  assert.equal(isOrderShipReady({ process_status: '生产完成', is_void: true }), false)
  assert.equal(isOrderShipReady({ process_status: '待计划', ship_status: '未发货' }), false)
})
