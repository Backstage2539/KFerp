import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { customerOrderAmounts, customerOrderPaymentStatus, priceUsageForMode } from './customerOrderFees'

describe('customer order fees and price selection', () => {
  it('uses the confirmed fee summary rather than a zero sales total', () => {
    expect(customerOrderAmounts({ grand_total: '0.00', fee_summary: { goods_cents: 0, shipping_cents: 1110, service_cents: 300, payable_cents: 1410, paid_cents: 0, unpaid_cents: 1410 } })).toEqual({ payable: '14.10', paid: '0.00', unpaid: '14.10', goods: '0.00', shipping: '11.10', service: '3.00' })
  })
  it('reports payment state from the whole order fee summary', () => {
    expect(customerOrderPaymentStatus({fee_summary:{goods_cents:0,shipping_cents:1110,service_cents:300,payable_cents:1410,paid_cents:0,unpaid_cents:1410},pay_status:'无需收款'})).toBe('未付款')
    expect(customerOrderPaymentStatus({fee_summary:{goods_cents:0,shipping_cents:1110,service_cents:300,payable_cents:1410,paid_cents:1410,unpaid_cents:0},pay_status:'未付款'})).toBe('已付款')
  })
  it('preserves legacy order amount compatibility', () => {
    expect(customerOrderAmounts({ grand_total: '100.00', pay_status: '未付款', prepayment_amount: '20.00' })).toMatchObject({ payable: '100.00', paid: '20.00', unpaid: '80.00' })
  })
  it('uses purchase pricing for stock and direct-ship pricing for on-demand', () => {
    expect(priceUsageForMode('stock')).toBe('product_order')
    expect(priceUsageForMode('on_demand')).toBe('direct_ship')
  })
  it('removes customer registration and outdated order controls', () => {
    const login = readFileSync(new URL('../pages/login/login.vue', import.meta.url), 'utf8')
    const orders = readFileSync(new URL('../pages/service/service.vue', import.meta.url), 'utf8')
    const profile = readFileSync(new URL('../pages/profile/profile.vue', import.meta.url), 'utf8')
    expect(login).toContain('loginWithPhoneVerify')
    expect(login).not.toContain('请先填写昵称')
    expect(profile).not.toContain('<RegistrationProfile')
    expect(orders).not.toContain('已购商品货款不重复收取')
    expect(orders).not.toContain('orderSearch.process_status ||')
    expect(orders).not.toContain('orderSearch.pay_status ||')
    expect(orders).not.toContain('orderSearch.ship_status ||')
  })
})
