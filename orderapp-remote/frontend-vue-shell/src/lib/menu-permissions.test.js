import test from 'node:test'
import assert from 'node:assert/strict'

import { actorHasFullViewAccess, filterMenuGroups, isViewAllowed } from './menu-permissions.js'

const groups = [
  { name: '订单', items: [{ key: 'order', label: '录单' }, { key: 'orders', label: '订单列表' }] },
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
