import { describe, expect, it } from 'vitest'
import { visibleOrderNotificationChoices, toggleOrderNotificationChoice } from './orderNotificationChoices'

describe('service account notification options', () => {
  it('shows each available option once, according to the current identity', () => {
    const all = visibleOrderNotificationChoices({ accountType: 'customer', capabilities: [{ code: 'direct_ship', enabled: true }, { code: 'settlement', enabled: true }] })
    expect(all.map(([key]) => key)).toEqual(['order_accepted', 'shipment_created', 'receipt_received', 'bill_issued'])
    expect(visibleOrderNotificationChoices({ accountType: 'employee', capabilities: [] }).map(([key]) => key)).toEqual(['order_accepted', 'shipment_created'])
    expect(visibleOrderNotificationChoices({ accountType: 'customer', capabilities: [{ code: 'settlement', enabled: true }] }).map(([key]) => key)).toEqual(['receipt_received', 'bill_issued'])
    expect(visibleOrderNotificationChoices({ accountType: 'customer', capabilities: [{ code: 'settlement', enabled: false }] })).toEqual([])
  })

  it('preserves old preferences and does not turn on the new shipment event implicitly', () => {
    const old = ['created', 'shipped']
    expect(toggleOrderNotificationChoice(old, 'order_accepted')).toEqual(['created', 'shipped', 'order_accepted'])
    expect(toggleOrderNotificationChoice(old, 'shipment_created')).toEqual(['created', 'shipped', 'shipment_created'])
    expect(toggleOrderNotificationChoice(['shipped', 'shipment_created'], 'shipment_created')).toEqual(['shipped'])
  })
})
