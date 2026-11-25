import { describe, expect, it, vi } from 'vitest'
import { cancelEmployeeOrder, employeeCancelReasons } from '../api/customerPortal'
import { miniRequest } from '../api/client'
vi.mock('../api/client', async importOriginal => ({
  ...await importOriginal<typeof import('../api/client')>(), miniRequest: vi.fn(),
}))
describe('employee order cancellation', () => {
  it('offers only reasons that require no note', () => {
    expect(employeeCancelReasons.map(option => option.label)).toEqual(['客户取消', '录单错误', '重复订单'])
  })
  it('sends just the selected reason for this order', async () => {
    await cancelEmployeeOrder('token', 42, 'duplicate')
    expect(miniRequest).toHaveBeenCalledWith('/api/mini/employee/orders/42/cancel', { method: 'POST', token: 'token', data: { reason: 'duplicate' } })
  })
})
