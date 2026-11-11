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
    expect(login).not.toContain('loginWithPassword')
    expect(login).toContain('loginWithPhoneIdentify')
    expect(login).not.toContain('用户名或手机号')
    expect(login).not.toContain('password placeholder="密码"')
    expect(login).toContain('手机号登录')
    expect(login).toContain('uni.login')
    expect(login).not.toContain('type="text" placeholder="密码"')
    expect(login).not.toContain('微信一键登录')

    expect(profile).toContain('switchCurrentCustomer')
    expect(profile).toContain('customerPickerLabels')
    expect(profile).toContain('handleCustomerSwitch')
    expect(profile).not.toContain('切换用户')
    expect(profile).toContain('退出登录')

    expect(home).toContain("{ key: 'employeeProfile', label: '个人中心', url: '/pages/profile/profile' }")

    for (const source of [home, mall, service]) {
      expect(source).toContain('MainTabBar')
      expect(source).not.toContain('handleCustomerSwitch')
      expect(source).not.toContain('退出登录')
    }
    for (const source of [mall, service]) expect(source).not.toContain('个人中心')
  })
})

describe('verified phone routing', () => {
  it('returns authenticated recipients to their validated notification details', async () => {
    const { phoneVerifiedDestination } = await import('./customerSwitch')
    const receipt = '/pages/notification-detail/notification-detail?type=receipt&id=42'
    const bill = '/pages/notification-detail/notification-detail?type=bill&id=9'
    const employeeOrder = '/pages/employee-order-detail/employee-order-detail?id=12'
    const customerOrder = '/pages/service/service?key=orders&source=notice&order_id=12&customer_id=74'
    expect(phoneVerifiedDestination({current_customer_id:74,bindings:[{customer_id:74,status:'approved'}]}, receipt)).toBe(receipt)
    expect(phoneVerifiedDestination({current_customer_id:74,bindings:[{customer_id:74,status:'approved'}]}, customerOrder)).toBe(customerOrder)
    expect(phoneVerifiedDestination({account_type:'employee'}, employeeOrder)).toBe(employeeOrder)
    expect(phoneVerifiedDestination({current_customer_id:0,bindings:[{customer_id:74,status:'approved'},{customer_id:75,status:'approved'}]}, bill)).toBe(`/pages/customer-select/customer-select?return_to=${encodeURIComponent(bill)}`)
  })
  it('does not accept forged notification returns or bypass account type', async () => {
    const { phoneVerifiedDestination } = await import('./customerSwitch')
    const receipt = '/pages/notification-detail/notification-detail?type=receipt&id=42'
    const customer = {current_customer_id:74,bindings:[{customer_id:74,status:'approved'}]}
    expect(phoneVerifiedDestination(customer, receipt + '&customer_id=75')).toBe('/pages/home/home')
    expect(phoneVerifiedDestination({account_type:'employee'}, receipt)).toBe('/pages/home/home')
    expect(phoneVerifiedDestination(customer, '/pages/employee-order-detail/employee-order-detail?id=12')).toBe('/pages/home/home')
  })
  it('sends an employee to the employee home', async () => {
    const { phoneVerifiedDestination } = await import('./customerSwitch')
    expect(phoneVerifiedDestination({account_type:'employee',bindings:[]}, '/pages/page-entry/page-entry?entry=abc')).toBe('/pages/home/home')
  })
  it('sends one verified customer to their home even with a price-list return target', async () => {
    const { phoneVerifiedDestination } = await import('./customerSwitch')
    expect(phoneVerifiedDestination({ current_customer_id: 74, bindings: [{customer_id:74, status:'approved'}] }, '/pages/page-entry/page-entry?entry=abc')).toBe('/pages/home/home')
  })
  it('never treats an unapproved binding as customer authorization', async () => {
    const { phoneVerifiedDestination } = await import('./customerSwitch')
    expect(phoneVerifiedDestination({current_customer_id:74,bindings:[{customer_id:74,status:'revoked'}]}, '')).toBe('/pages/index/index')
  })
  it('asks a verified multi-customer user to choose before entering a customer area', async () => {
    const { phoneVerifiedDestination } = await import('./customerSwitch')
    expect(phoneVerifiedDestination({current_customer_id:0, bindings:[{customer_id:1,status:'approved'},{customer_id:2,status:'approved'}]}, '/pages/page-entry/page-entry?entry=abc')).toBe('/pages/customer-select/customer-select?return_to=%2Fpages%2Fhome%2Fhome')
  })
  it('keeps the return destination for a visitor without approved customer bindings', async () => {
    const { phoneVerifiedDestination } = await import('./customerSwitch')
    const visitorEntry = '/pages/page-entry/page-entry?entry=abcdef0123456789abcdef0123456789'
    expect(phoneVerifiedDestination({current_customer_id:0,bindings:[]}, visitorEntry)).toBe(visitorEntry)
    expect(phoneVerifiedDestination({current_customer_id:0,bindings:[]}, 'https://evil.test')).toBe('/pages/index/index')
    expect(phoneVerifiedDestination({current_customer_id:0,bindings:[{customer_id:1,status:'revoked'}]}, '')).toBe('/pages/index/index')
  })
})

it('offers only verified phone login and no username/password entry', () => {
  const login = readFileSync(resolve('src/pages/login/login.vue'), 'utf8')
  expect(login).toContain('loginWithPhoneIdentify')
  expect(login).not.toContain('loginWithPassword')
  expect(login).not.toContain('用户名或手机号')
  expect(login).not.toContain('password placeholder')
})
