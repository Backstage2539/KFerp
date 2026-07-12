import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const source = readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
const pane = source.match(/<div v-show="showProductPriceManagementPane"[\s\S]*?<div v-if="productDrawerOpen"/)?.[0] || ''
const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

test('product price management exposes sibling pricing-rule and costing-parameter tabs', () => {
  assert.match(pane, /role="tablist" aria-label="商品价格管理功能"/)
  assert.match(pane, /activeProductPriceManagementTab === 'pricing-rules'[\s\S]*价格计算模板/)
  assert.match(pane, /activeProductPriceManagementTab === 'cost-parameters'[\s\S]*成本参数设置/)
  assert.ok(pane.indexOf('价格计算模板') < pane.indexOf('成本参数设置'))
  assert.match(script, /const activeProductPriceManagementTab = ref\('pricing-rules'\)/)
})

test('product price management isolates pricing actions and costing panel by active tab', () => {
  assert.match(pane, /v-if="activeProductPriceManagementTab === 'pricing-rules'" class="panel-actions"/)
  assert.match(pane, /v-show="activeProductPriceManagementTab === 'pricing-rules'"\s+class="product-price-records-panel pricing-rule-management-panel"/)
  assert.match(pane, /v-show="activeProductPriceManagementTab === 'cost-parameters'"\s+class="costing-settings-tab-panel"[\s\S]*<CostingSettingsPanel/)
  assert.match(pane, /class="muted price-list-flat-row-note" v-show="activeProductPriceManagementTab === 'pricing-rules'"/)
})
