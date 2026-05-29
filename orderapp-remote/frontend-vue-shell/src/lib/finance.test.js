import test from 'node:test'
import assert from 'node:assert/strict'

import {
  currentMonth,
  financeMetricCards,
  financeReportExportUrls,
  financeStatusLabel,
  monthFromDate,
  money,
  percent,
  rateFromPercent,
  rateToPercent,
} from './finance.js'

test('currentMonth formats a date as YYYY-MM', () => {
  assert.equal(currentMonth(new Date('2026-05-02T08:30:00+08:00')), '2026-05')
})

test('monthFromDate returns the posting month for finance expense dates', () => {
  assert.equal(monthFromDate('2026-04-15'), '2026-04')
  assert.equal(monthFromDate(''), '')
  assert.equal(monthFromDate('bad'), '')
})

test('finance labels and formatters keep boss brief values readable', () => {
  assert.equal(money(1234.5), '1,234.50')
  assert.equal(percent(0.3278), '32.78%')
  assert.equal(financeStatusLabel('closed'), '已结账')
  assert.equal(financeStatusLabel('adjusted'), '已调整')
  assert.equal(financeStatusLabel('draft'), '未结账')
})

test('finance report export URLs point to PDF and Excel endpoints for the month', () => {
  assert.deepEqual(financeReportExportUrls('2026-05'), {
    pdf: '/api/finance/reports/2026-05/pdf',
    excel: '/api/finance/reports/2026-05/excel',
  })
  assert.deepEqual(financeReportExportUrls('2026-05', 18), {
    pdf: '/api/finance/reports/2026-05/pdf?customer_id=18',
    excel: '/api/finance/reports/2026-05/excel?customer_id=18',
  })
})

test('finance metric cards prefer adjusted net profit and tax totals when available', () => {
  const cards = financeMetricCards({
    tax_exclusive_revenue: 100000,
    gross_profit: 42000,
    operating_net_profit: 26000,
    tax: { total_tax: 3100 },
    adjusted_net_profit: 24500,
    adjusted_tax_total: 3600,
  })
  assert.deepEqual(cards.map((card) => card.label), ['不含税收入', '毛利', '净利', '税费估算'])
  assert.equal(cards.find((card) => card.label === '净利')?.value, 24500)
  assert.equal(cards.find((card) => card.label === '税费估算')?.value, 3600)
  assert.equal(financeMetricCards({ tax: { total_tax: 3100 } }).find((card) => card.label === '税费估算')?.value, 3100)
})

test('rate helpers convert user-facing percentages to decimal rates', () => {
  assert.equal(rateToPercent(0.13), '13')
  assert.equal(rateFromPercent('13'), 0.13)
})
