import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

import { menuGroups, menuMap, primaryMenuKeys } from './menu-ia.js'
import { productionTopNavItems } from './production-workstation.js'

test('production menu consolidates five workflow pages and keeps the manual last', () => {
  const production = menuGroups.find((group) => group.id === 'production')
  const keys = production?.items.map((item) => item.key) || []
  const primaryKeys = primaryMenuKeys(menuGroups)

  assert.ok(keys.includes('productionFlow'))
  assert.equal(production?.items.find((item) => item.key === 'productionFlow')?.label, '生产流程')
  for (const key of ['producePlan', 'workOrders', 'jobCards', 'qualityInspections', 'productionAcceptance']) {
    assert.equal(primaryKeys.includes(key), false, `${key} should not remain a primary menu entry`)
    assert.ok(menuMap[key], `${key} legacy route should remain compatible`)
  }
  assert.equal(keys.at(-1), 'productionManual')
})

test('production flow page groups the five workflow tabs', () => {
  const source = readFileSync(new URL('../views/ProductionFlowView.vue', import.meta.url), 'utf8')

  for (const label of ['生产计划', '生产工单', '工序卡', '生产质检', '生产验收']) {
    assert.match(source, new RegExp(label))
  }
  for (const component of ['ProducePlanView', 'WorkOrdersView', 'JobCardsView', 'QualityInspectionsView', 'ProductionAcceptanceView']) {
    assert.match(source, new RegExp(component))
  }
})

test('production module top navigation points to the consolidated flow', () => {
  const keys = productionTopNavItems.map((item) => item.key)

  assert.ok(keys.includes('productionFlow'))
  for (const key of ['producePlan', 'workOrders', 'jobCards', 'qualityInspections']) {
    assert.equal(keys.includes(key), false)
  }
})

test('embedded production workflow pages hide their nested module navigation', () => {
  for (const file of ['ProducePlanView.vue', 'WorkOrdersView.vue', 'JobCardsView.vue', 'QualityInspectionsView.vue']) {
    const source = readFileSync(new URL(`../views/${file}`, import.meta.url), 'utf8')
    assert.match(source, /<ProductionTopNav\s+v-if="!props\.embedded"/)
    assert.match(source, /embedded:\s*\{\s*type:\s*Boolean,\s*default:\s*false\s*\}/)
  }
})

test('embedded production plan preserves the production flow route', () => {
  const source = readFileSync(new URL('../views/ProducePlanView.vue', import.meta.url), 'utf8')

  assert.match(source, /if \(!props\.embedded\) url\.searchParams\.set\('view', 'producePlan'\)/)
})

test('vue shell registers production flow and keeps legacy workflow mappings', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(source, /productionFlow:\s*ProductionFlowView/)
  assert.match(source, /producePlan:\s*ProducePlanView/)
  assert.match(source, /workOrders:\s*WorkOrdersView/)
  assert.match(source, /jobCards:\s*JobCardsView/)
  assert.match(source, /qualityInspections:\s*QualityInspectionsView/)
  assert.match(source, /productionAcceptance:\s*ProductionAcceptanceView/)
})
