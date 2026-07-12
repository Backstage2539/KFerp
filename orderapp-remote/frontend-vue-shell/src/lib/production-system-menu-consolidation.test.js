import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

import { groupForView, menuGroups, menuMap, primaryMenuKeys } from './menu-ia.js'
import { productionTopNavItems } from './production-workstation.js'

test('system settings moves from settings to system menu', () => {
  const settingsKeys = menuGroups.find((group) => group.id === 'settings')?.items.map((item) => item.key) || []
  const systemKeys = menuGroups.find((group) => group.id === 'system')?.items.map((item) => item.key) || []

  assert.equal(settingsKeys.includes('uiSettings'), false)
  assert.equal(systemKeys.includes('uiSettings'), true)
  assert.equal(groupForView(menuGroups, 'uiSettings')?.id, 'system')
})

test('production menu consolidates manufacturing master data and removes production cost entry', () => {
  const keys = primaryMenuKeys(menuGroups)
  const production = menuGroups.find((group) => group.id === 'production')

  assert.ok(keys.includes('productionConfig'))
  assert.equal(production?.items.find((item) => item.key === 'productionConfig')?.label, '生产配置')
  for (const key of ['processTemplates', 'manufacturingOperations', 'manufacturingWorkstations', 'productionCosts']) {
    assert.equal(keys.includes(key), false, `${key} should not remain a primary menu entry`)
    assert.ok(menuMap[key], `${key} legacy route should remain compatible`)
  }
  assert.equal(productionTopNavItems.some((item) => item.key === 'productionCosts'), false)
})

test('production plan menu uses the concise label', () => {
  assert.equal(menuMap.producePlan?.title, '生产计划')
  assert.equal(primaryMenuKeys(menuGroups).includes('producePlan'), false)
})

test('production configuration page groups route operation and workstation tabs', () => {
  const source = readFileSync(new URL('../views/ProductionSettingsView.vue', import.meta.url), 'utf8')

  assert.match(source, /工艺路线/)
  assert.match(source, /工序/)
  assert.match(source, /工位\/设备/)
  assert.match(source, /ProcessTemplatesView/)
  assert.match(source, /ManufacturingOperationsView/)
  assert.match(source, /ManufacturingWorkstationsView/)
})

test('vue shell registers production configuration and keeps legacy production routes', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /productionConfig:\s*ProductionSettingsView/)
  assert.match(source, /processTemplates:\s*ProcessTemplatesView/)
  assert.match(source, /manufacturingOperations:\s*ManufacturingOperationsView/)
  assert.match(source, /manufacturingWorkstations:\s*ManufacturingWorkstationsView/)
  assert.match(source, /productionCosts:\s*ProductionCostsView/)
})
