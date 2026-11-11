import { describe, expect, it } from 'vitest'
import { defaultOrderNotificationEvents, orderNotificationChoices } from './orderNotificationChoices'

describe('order notification choices', () => {
  it('requires an explicit choice for the two new service-account tests', () => {
    expect(orderNotificationChoices.map(([key]) => key)).toContain('order_accepted')
    expect(orderNotificationChoices.map(([key]) => key)).toContain('shipment_created')
    expect(defaultOrderNotificationEvents).not.toContain('order_accepted')
    expect(defaultOrderNotificationEvents).not.toContain('shipment_created')
    expect(orderNotificationChoices.map(([key]) => key)).toContain('receipt_received')
    expect(orderNotificationChoices.map(([key]) => key)).toContain('bill_issued')
    expect(defaultOrderNotificationEvents).not.toContain('receipt_received')
    expect(defaultOrderNotificationEvents).not.toContain('bill_issued')
  })
})
