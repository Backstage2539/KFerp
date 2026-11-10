import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { previewSafeEntries } from './customerPreview'

describe('customer preview navigation', () => {
  it('exposes only the inventory read entry, never ordering or creation flows', () => {
    const entries = ['mall', 'productOrder', 'directShip', 'onDemandShip', 'processing', 'inventory'].map(key => ({ key, label: key, url: '/' + key }))
    expect(previewSafeEntries(entries).map(entry => entry.key)).toEqual(['inventory'])
  })
  it('provides admin entry and a return control without logout during preview', () => {
    const home = readFileSync(new URL('../pages/home/home.vue', import.meta.url), 'utf8')
    const tabs = readFileSync(new URL('../components/MainTabBar.vue', import.meta.url), 'utf8')
    const profile = readFileSync(new URL('../pages/profile/profile.vue', import.meta.url), 'utf8')
    expect(home).toContain("session.roles.includes('admin')")
    expect(home).toContain('客户只读预览')
    expect(tabs).toContain('返回员工工作台')
    expect(profile).toContain('!session.isCustomerPreview')
  })
})

it('treats an explicitly scoped preview as authenticated for the customer price list', async () => {
  const { hasAuthenticatedCustomerSession } = await import('./priceCenterAccess')
  expect(hasAuthenticatedCustomerSession({ token: 'preview-token', miniUserID: 0, currentCustomerID: 74, accountType: 'preview', isCustomerPreview: true, bindings: [] })).toBe(true)
  expect(hasAuthenticatedCustomerSession({ token: 'preview-token', miniUserID: 9, currentCustomerID: 0, accountType: 'preview', isCustomerPreview: true, bindings: [] })).toBe(false)
})
