import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

import { menuMap, menuGroups, primaryMenuKeys } from './menu-ia.js'

test('settings menu removes outsource and standalone costing entries while preserving legacy routes', () => {
  const keys = primaryMenuKeys(menuGroups)

  assert.equal(keys.includes('outsourceSettings'), false)
  assert.equal(keys.includes('costingSettings'), false)
  assert.ok(menuMap.outsourceSettings)
  assert.ok(menuMap.costingSettings)
  assert.ok(keys.includes('companyProfile'))
  assert.ok(keys.includes('productPriceManagement'))
})

test('company settings embeds shared seal asset settings', () => {
  const source = readFileSync(new URL('../views/CompanyProfileView.vue', import.meta.url), 'utf8')

  assert.match(source, /import CompanySealSettingsView from '.\/CompanySealSettingsView\.vue'/)
  assert.match(source, /<CompanySealSettingsView\s+embedded\s+:closable="false"/)
  assert.match(source, /公司资料与公章/)
})

test('product price management embeds costing parameters', () => {
  const source = readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const panel = readFileSync(new URL('../components/CostingSettingsPanel.vue', import.meta.url), 'utf8')
  const priceManagement = source.match(/<div v-show="showProductPriceManagementPane"[\s\S]*?<div v-if="productDrawerOpen"/)?.[0] || ''

  assert.match(source, /import CostingSettingsPanel from '\.\.\/components\/CostingSettingsPanel\.vue'/)
  assert.match(priceManagement, /<CostingSettingsPanel/)
  assert.match(panel, /成本参数设置/)
})

test('legacy settings components remain mapped for direct URL compatibility', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /costingSettings:\s*CostingSettingsView/)
  assert.match(source, /outsourceSettings:\s*OutsourceSettingsView/)
})
