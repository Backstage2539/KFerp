import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { ref } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { shanghaiToday } from './employeeOrder'

afterEach(() => vi.useRealTimers())

it.each([
  ['2026-10-18T19:10:00Z', '2026-10-19'], // Sunday UTC, Monday business week.
  ['2026-10-31T16:01:00Z', '2026-11-01'], // Month boundary.
  ['2026-10-18T15:59:59Z', '2026-10-18'],
])('opens customer bills on the Shanghai date at %s', (instant, expected) => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date(instant))
  // Execute the actual component initializer without starting its API lifecycle.
  const source = readFileSync(resolve('src/components/CustomerBillsPanel.vue'), 'utf8')
  const expression = source.match(/^const anchor = (.+)$/m)?.[1]
  expect(expression).toBeTruthy()
  const initialAnchor = new Function('ref', 'shanghaiToday', `return (${expression})`)(ref, shanghaiToday)
  expect(initialAnchor.value).toBe(expected)
})
