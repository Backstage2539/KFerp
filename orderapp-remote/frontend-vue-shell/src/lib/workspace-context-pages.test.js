import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

function source(path) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

test('warehouse inventory applies customer workspace context and hides cross-customer trace tools', () => {
  const view = source('../views/WarehouseInventoryView.vue')

  assert.match(view, /customerContextId:\s*\{\s*type:\s*\[Number,\s*String\]/)
  assert.match(view, /isCustomerInventoryContext/)
  assert.match(view, /url\.searchParams\.set\('customer_id'/)
  assert.match(view, /v-if="!isCustomerInventoryContext"/)
})

test('SKU settings locks the customer selector when the shell supplies customer context', () => {
  const view = source('../views/ProductSettingsView.vue')

  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /v-if="!isWorkspaceCustomerLocked"\s+class="sku-context-controls"/)
  assert.match(view, /客户账户模式下由顶部当前客户控制/)
})

test('BOM settings shows public plus current customer products but locks customer switching', () => {
  const view = source('../views/BomView.vue')

  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /v-if="!isWorkspaceCustomerLocked"\s+class="bom-sku-context-controls"/)
  assert.match(view, /canEditCurrentBomProduct/)
})

test('bean list page locks scope to the current customer in customer workspace', () => {
  const view = source('../views/CostingView.vue')

  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /v-if="!isWorkspaceCustomerLocked"\s+class="scope-select"/)
  assert.match(view, /versionListScope\.value = `customer:\$\{pageCustomerID\}`/)
})

test('finance expenses uses customer context without exposing a customer selector', () => {
  const view = source('../views/FinanceExpensesView.vue')

  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /v-if="!isWorkspaceCustomerLocked"/)
  assert.match(view, /客户账户费用固定为当前客户/)
})
