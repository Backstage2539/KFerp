import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  customerEntryRoute,
  customerPickerIndex,
  customerPickerLabels,
  selectedCustomerID,
  shouldShowCustomerSwitcher,
} from './customerSwitch'

describe('miniapp customer switching', () => {
  const bindings = [
    { customer_id: 101, customer_name: '13800138075', role: 'owner', status: 'approved' },
    { customer_id: 202, customer_name: '公共SKU客户', role: 'member', status: 'approved' },
  ]

  it('builds customer picker labels and selected ids from approved bindings', () => {
    expect(shouldShowCustomerSwitcher(bindings)).toBe(true)
    expect(customerPickerLabels(bindings, 101)).toEqual(['13800138075（当前）', '公共SKU客户'])
    expect(customerPickerIndex(bindings, 202)).toBe(1)
    expect(selectedCustomerID(bindings, 1)).toBe(202)
    expect(selectedCustomerID(bindings, 99)).toBe(0)
  })

  it('routes a switched customer to the correct miniapp entry page', () => {
    expect(customerEntryRoute({ miniapp_entry_mode: 'mall', capabilities: [{ code: 'mall', enabled: true }] })).toBe('/pages/mall/mall')
    expect(customerEntryRoute({ miniapp_entry_mode: 'mall', capabilities: [{ code: 'mall', enabled: false }] })).toBe('/pages/home/home')
    expect(customerEntryRoute({ miniapp_entry_mode: 'services', capabilities: [{ code: 'direct_ship', enabled: true }] })).toBe('/pages/home/home')
  })

  it('wires customer switch and logout controls into all authenticated pages', () => {
    const home = readFileSync(resolve('src/pages/home/home.vue'), 'utf8')
    const mall = readFileSync(resolve('src/pages/mall/mall.vue'), 'utf8')
    const service = readFileSync(resolve('src/pages/service/service.vue'), 'utf8')

    for (const source of [home, mall, service]) {
      expect(source).toContain('switchCurrentCustomer')
      expect(source).toContain('customerPickerLabels')
      expect(source).toContain('handleCustomerSwitch')
      expect(source).toContain('退出登录')
    }
  })
})
