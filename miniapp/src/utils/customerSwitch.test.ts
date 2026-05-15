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
    expect(customerEntryRoute({ miniapp_entry_mode: 'mall', capabilities: [{ code: 'mall', enabled: true }] })).toBe('/pages/home/home')
    expect(customerEntryRoute({ miniapp_entry_mode: 'mall', capabilities: [{ code: 'mall', enabled: false }] })).toBe('/pages/home/home')
    expect(customerEntryRoute({ miniapp_entry_mode: 'services', capabilities: [{ code: 'direct_ship', enabled: true }] })).toBe('/pages/home/home')
  })

  it('moves account actions into profile and keeps quick plus password login entry', () => {
    const pages = JSON.parse(readFileSync(resolve('src/pages.json'), 'utf8')) as { pages: { path: string }[] }
    const login = readFileSync(resolve('src/pages/login/login.vue'), 'utf8')
    const home = readFileSync(resolve('src/pages/home/home.vue'), 'utf8')
    const mall = readFileSync(resolve('src/pages/mall/mall.vue'), 'utf8')
    const service = readFileSync(resolve('src/pages/service/service.vue'), 'utf8')
    const profile = readFileSync(resolve('src/pages/profile/profile.vue'), 'utf8')

    expect(pages.pages.map((page) => page.path)).toContain('pages/profile/profile')
    expect(login).toContain('loginWithPassword')
    expect(login).toContain('loginWithPhoneVerify')
    expect(login).toContain('用户名或手机号')
    expect(login).toContain('password placeholder="密码"')
    expect(login).toContain('手机号快捷登录')
    expect(login).toContain('uni.login')
    expect(login).not.toContain('type="text" placeholder="密码"')
    expect(login).not.toContain('微信一键登录')

    expect(profile).toContain('switchCurrentCustomer')
    expect(profile).toContain('customerPickerLabels')
    expect(profile).toContain('handleCustomerSwitch')
    expect(profile).toContain('切换用户')
    expect(profile).toContain('退出登录')

    for (const source of [home, mall, service]) {
      expect(source).toContain('MainTabBar')
      expect(source).not.toContain('个人中心')
      expect(source).not.toContain('handleCustomerSwitch')
      expect(source).not.toContain('退出登录')
    }
  })
})
