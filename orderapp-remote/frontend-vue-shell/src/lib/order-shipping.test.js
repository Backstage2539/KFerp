import test from 'node:test'
import assert from 'node:assert/strict'

import { formatTrackingSummary, normalizeTrackingInput } from './order-shipping.js'

test('normalizeTrackingInput keeps unique tracking numbers in entry order', () => {
  assert.deepEqual(normalizeTrackingInput(' SF123, SF124\nSF123；SF125 '), ['SF123', 'SF124', 'SF125'])
})

test('formatTrackingSummary makes multi tracking visible without hiding the first number', () => {
  assert.equal(formatTrackingSummary('SF123\nSF124\nSF125'), 'SF123 等 3 个单号')
  assert.equal(formatTrackingSummary('SF123'), 'SF123')
  assert.equal(formatTrackingSummary(''), '未回填')
})
