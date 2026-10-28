import { describe, expect, it } from 'vitest'
import { customerFeeRange, validCustomerFeeDates } from './customerBillDate'
import { buildCustomerAccountDocumentPath } from '../api/customerPortal'

describe('customer fee period', () => {
  it('uses Shanghai natural months across a year boundary', () => {
    expect(customerFeeRange('current', '2026-01-01')).toEqual({ date_from: '2026-01-01', date_to: '2026-01-31' })
    expect(customerFeeRange('previous', '2026-01-01')).toEqual({ date_from: '2025-12-01', date_to: '2025-12-31' })
    expect(customerFeeRange('previous', '2024-03-01')).toEqual({ date_from: '2024-02-01', date_to: '2024-02-29' })
  })
  it('validates inclusive custom dates and retains all-unpaid scope for exports', () => {
    expect(validCustomerFeeDates('2026-10-01', '2026-10-01')).toBe(true)
    expect(validCustomerFeeDates('2026-10-02', '2026-10-01')).toBe(false)
    expect(customerFeeRange('unpaid', '2026-10-28')).toEqual({ unpaid_only: true })
    const url = buildCustomerAccountDocumentPath('xlsx', { view: 'order_fees', unpaid_only: true, charge_type: 'shipping', page: 4 })
    expect(url).toContain('unpaid_only=true')
    expect(url).not.toContain('charge_type')
    expect(url).not.toContain('page=')
  })
})
