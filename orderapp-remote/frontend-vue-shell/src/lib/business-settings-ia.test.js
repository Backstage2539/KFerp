import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

import { groupForView, menuMap, menuGroups, primaryMenuKeys } from './menu-ia.js'

test('product and settings menus expose the consolidated information architecture', () => {
  const product = menuGroups.find((group) => group.id === 'product')
  const settings = menuGroups.find((group) => group.id === 'settings')
  const keys = primaryMenuKeys(menuGroups)

  assert.equal(product?.name, '商品')
  assert.ok(keys.includes('businessSettings'))
  assert.equal(groupForView(menuGroups, 'businessSettings')?.id, 'settings')
  assert.equal(settings?.items.find((item) => item.key === 'businessSettings')?.label, '业务设置')

  for (const key of ['salesOrderSettings', 'logisticsSettings', 'senderSettings', 'groupTemplates', 'notificationSettings', 'machines']) {
    assert.equal(keys.includes(key), false, `${key} should no longer be a standalone menu item`)
  }

  for (const key of ['salesOrderSettings', 'logisticsSettings', 'senderSettings', 'groupTemplates', 'groupManagement', 'notificationSettings', 'machines']) {
    assert.ok(menuMap[key], `${key} should remain available as a compatible direct route`)
  }
})

test('business settings composes five existing settings into tabs', () => {
  const source = readFileSync(new URL('../views/BusinessSettingsView.vue', import.meta.url), 'utf8')

  for (const label of ['销售单设置', '物流设置', '发货人设置', '分组模板', '全局单位字典']) {
    assert.match(source, new RegExp(label))
  }
  for (const component of ['SalesOrderSettingsView', 'LogisticsSettingsView', 'SenderSettingsView', 'GroupTemplatesView', 'GlobalUnitDefinitionsView']) {
    assert.match(source, new RegExp(component))
  }
})

test('system settings owns notification settings while global units move to business settings', () => {
  const systemSource = readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
  const unitsSource = readFileSync(new URL('../views/GlobalUnitDefinitionsView.vue', import.meta.url), 'utf8')

  assert.match(systemSource, /系统基础设置/)
  assert.match(systemSource, /通知设置/)
  assert.match(systemSource, /NotificationSettingsView/)
  assert.doesNotMatch(systemSource, /全局单位字典/)
  assert.match(unitsSource, /全局单位字典/)
  assert.match(unitsSource, /\/api\/product-settings\/units/)
})

test('Vue shell maps the business settings page and keeps legacy settings routes', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /import BusinessSettingsView from '.\/views\/BusinessSettingsView\.vue'/)
  assert.match(source, /businessSettings:\s*BusinessSettingsView/)
  assert.match(source, /salesOrderSettings:\s*SalesOrderSettingsView/)
  assert.match(source, /logisticsSettings:\s*LogisticsSettingsView/)
  assert.match(source, /senderSettings:\s*SenderSettingsView/)
  assert.match(source, /groupTemplates:\s*GroupTemplatesView/)
  assert.match(source, /notificationSettings:\s*NotificationSettingsView/)
  assert.match(source, /machines:\s*MachinesView/)
})
