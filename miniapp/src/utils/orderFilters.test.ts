import { describe, expect, it } from 'vitest'
import { buildOrderServiceFilters, datePresetRange, normalizeDateRange } from './orderFilters'

describe('order filter helpers', () => {
  const today = new Date('2026-05-03T09:30:00+08:00')

  it('builds preset date ranges for today, recent days, and current month', () => {
    expect(datePresetRange('today', today)).toEqual({ date_from: '2026-05-03', date_to: '2026-05-03' })
    expect(datePresetRange('last3', today)).toEqual({ date_from: '2026-05-01', date_to: '2026-05-03' })
    expect(datePresetRange('last7', today)).toEqual({ date_from: '2026-04-27', date_to: '2026-05-03' })
    expect(datePresetRange('month', today)).toEqual({ date_from: '2026-05-01', date_to: '2026-05-03' })
  })

  it('normalizes manual date ranges by ordering from/to and trimming invalid input', () => {
    expect(normalizeDateRange('2026-05-07', '2026-05-01')).toEqual({
      date_from: '2026-05-01',
      date_to: '2026-05-07',
    })
    expect(normalizeDateRange('bad', '2026-05-03')).toEqual({ date_to: '2026-05-03' })
  })

  it('builds query params only from meaningful keyword and date filters', () => {
    expect(buildOrderServiceFilters({ keyword: ' 乌拉嘎 上海 ', date_from: '2026-05-01', date_to: '2026-05-03' })).toEqual({
      q: '乌拉嘎 上海',
      date_from: '2026-05-01',
      date_to: '2026-05-03',
    })
    expect(buildOrderServiceFilters({ keyword: '   ' })).toEqual({})
  })
})
