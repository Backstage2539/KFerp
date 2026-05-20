import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import { actorHasFullViewAccess, filterMenuGroups, isCustomerAccountMode, isViewAllowed } from './menu-permissions.js'

const groups = [
  { name: '订单', items: [{ key: 'order', label: '录单' }, { key: 'orders', label: '订单列表' }] },
  { name: '客户履约', items: [{ key: 'customerFulfillment', label: '履约运营台' }, { key: 'customerPortalSettings', label: '门户客户配置' }] },
  { name: '设置', items: [{ key: 'machines', label: '设备产能配置' }] },
]

test('filterMenuGroups keeps only allowed items and removes empty groups', () => {
  assert.deepEqual(filterMenuGroups(groups, ['orders']), [
    { name: '订单', items: [{ key: 'orders', label: '订单列表' }] },
  ])
})

test('isViewAllowed treats null allowed views as admin access', () => {
  assert.equal(isViewAllowed('machines', null), true)
})

test('isViewAllowed denies unknown view when allowed list is present', () => {
  assert.equal(isViewAllowed('machines', ['orders']), false)
})

test('actorHasFullViewAccess treats admin role as full access even when allowed_views is null', () => {
  assert.equal(actorHasFullViewAccess({ roles: [{ code: 'admin' }], allowed_views: null }), true)
  assert.equal(actorHasFullViewAccess({ basic_auth_admin: true }), true)
  assert.equal(actorHasFullViewAccess({ roles: [{ code: 'sales' }], allowed_views: ['orders'] }), false)
})

test('isCustomerAccountMode detects channel customer actors', () => {
  assert.equal(isCustomerAccountMode({ account_type: 'channel_customer' }), true)
  assert.equal(isCustomerAccountMode({ account_type: 'internal_employee', allowed_views: ['orders'] }), false)
  assert.equal(isCustomerAccountMode({ allowed_views: ['customerProcessingPortal'], roles: [] }), true)
})

test('filterMenuGroups hides fulfillment console in customer account mode when setting is enabled', () => {
  const actor = { account_type: 'channel_customer' }
  const filtered = filterMenuGroups(groups, ['customerFulfillment', 'customerPortalSettings'], {
    actor,
    hideCustomerAccountFulfillment: true,
  })

  assert.equal(JSON.stringify(filtered).includes('customerFulfillment'), false)
  assert.equal(JSON.stringify(filtered).includes('履约运营台'), false)
  assert.equal(JSON.stringify(filtered).includes('customerPortalSettings'), true)
})

test('filterMenuGroups keeps fulfillment console when customer account hiding is disabled', () => {
  const actor = { account_type: 'channel_customer' }
  const filtered = filterMenuGroups(groups, ['customerFulfillment'], {
    actor,
    hideCustomerAccountFulfillment: false,
  })

  assert.equal(JSON.stringify(filtered).includes('customerFulfillment'), true)
})

test('filterMenuGroups hides fulfillment console for full-access actors in customer workspace', () => {
  const actor = { roles: [{ code: 'admin' }], allowed_views: null }
  const filtered = filterMenuGroups(groups, null, {
    actor,
    workspaceMode: 'customer',
    hideCustomerAccountFulfillment: true,
  })

  assert.equal(JSON.stringify(filtered).includes('customerFulfillment'), false)
  assert.equal(JSON.stringify(filtered).includes('履约运营台'), false)
  assert.equal(JSON.stringify(filtered).includes('order'), true)
  assert.equal(JSON.stringify(filtered).includes('machines'), true)
})

test('Vue shell passes workspace mode into menu filtering', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /filterMenuGroups\(workspaceMenuGroups\.value,\s*allowedViewKeys\.value,\s*\{[\s\S]*workspaceMode:\s*workspaceMode\.value/)
})
