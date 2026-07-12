import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { test } from 'node:test'

const productSettings = readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
const costing = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')
const app = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
const menu = readFileSync(new URL('./menu-ia.js', import.meta.url), 'utf8')

test('product price management only exposes pricing rules after obsolete cost parameters are removed', () => {
  const pane = productSettings.match(/<div v-show="showProductPriceManagementPane"[\s\S]*?<p class="muted price-list-flat-row-note"/)?.[0] || ''

  assert.match(pane, /价格计算模板 \/ Pricing Rule/)
  assert.doesNotMatch(pane, /成本参数设置|CostingSettingsPanel|cost-parameters|activeProductPriceManagementTab/)
})

test('costing workbench and legacy routes expose no cost parameter editor', () => {
  assert.doesNotMatch(costing, /参数设置|快速成本参数设置|CostingSettingsPanel|settingsOpen|handleSettingSaved/)
  assert.doesNotMatch(app, /CostingSettingsView|costingSettings:/)
  assert.doesNotMatch(menu, /costingSettings:/)
})

test('obsolete cost parameter Vue components and helpers are deleted', () => {
  for (const relative of [
    '../components/CostingSettingsPanel.vue',
    '../views/CostingSettingsView.vue',
    './costing-settings.js',
    './costing-settings.test.js',
  ]) {
    assert.equal(existsSync(new URL(relative, import.meta.url)), false, `${relative} should be deleted`)
  }
})
