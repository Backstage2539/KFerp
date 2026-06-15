import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import { formatLocalDateInput } from './local-date.js'

test('formatLocalDateInput formats the local calendar day without UTC rollover', () => {
  assert.equal(formatLocalDateInput(new Date(2026, 5, 16, 0, 30)), '2026-06-16')
  assert.equal(formatLocalDateInput(new Date(2026, 0, 2, 3, 4)), '2026-01-02')
})

test('production date defaults use local date helper instead of toISOString slicing', () => {
  const overview = readFileSync(new URL('../views/ProductionOverviewView.vue', import.meta.url), 'utf8')
  const schedule = readFileSync(new URL('../views/ProductionScheduleView.vue', import.meta.url), 'utf8')

  assert.match(overview, /formatLocalDateInput/)
  assert.match(schedule, /formatLocalDateInput/)
  assert.doesNotMatch(overview, /toISOString\(\)\.slice\(0,\s*10\)/)
  assert.doesNotMatch(schedule, /toISOString\(\)\.slice\(0,\s*10\)/)
})
