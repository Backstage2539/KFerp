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

test('product settings keeps customer context in code without showing legacy SKU scope selector', () => {
  const view = source('../views/ProductSettingsView.vue')

  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /watch\(selectedCustomerSkuCustomerID/)
  assert.doesNotMatch(view, /class="sku-context-controls"/)
  assert.doesNotMatch(view, /SKU归属/)
})

test('BOM settings uses customer context without showing legacy SKU scope selector', () => {
  const view = source('../views/BomView.vue')

  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /客户账户模式下只显示该客户的 BOM 行/)
  assert.doesNotMatch(view, /class="bom-sku-context-controls"/)
  assert.doesNotMatch(view, /SKU归属/)
  assert.match(view, /canEditCurrentBomProduct/)
})

test('bean list page locks scope to the current customer in customer workspace', () => {
  const view = source('../views/CostingView.vue')

  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /v-if="!isWorkspaceCustomerLocked"\s+class="scope-select"/)
  assert.match(view, /versionListScope\.value = `customer:\$\{pageCustomerID\}`/)
})

test('finance expenses uses customer context without exposing a customer selector', () => {
  const app = source('../App.vue')
  const view = source('../views/FinanceExpensesView.vue')

  assert.match(app, /:customer-account-actor="isCustomerActor"/)
  assert.match(view, /customerAccountActor:\s*\{\s*type:\s*Boolean/)
  assert.match(view, /isWorkspaceCustomerLocked/)
  assert.match(view, /isCustomerFinanceReadOnly/)
  assert.match(view, /v-if="!isWorkspaceCustomerLocked"/)
  assert.match(view, /v-if="!isCustomerFinanceReadOnly"/)
  assert.match(view, /if \(!isCustomerFinanceReadOnly\.value\)\s*\{\s*await loadEmployees\(\)/)
  assert.match(view, /客户账户费用固定为当前客户/)
})

test('customer finance report and closing use customer context as read-only financial views', () => {
  const report = source('../views/FinanceReportView.vue')
  const closing = source('../views/FinanceClosingView.vue')

  for (const view of [report, closing]) {
    assert.match(view, /customerContextId:\s*\{\s*type:\s*\[Number,\s*String\]/)
    assert.match(view, /isWorkspaceCustomerLocked/)
    assert.match(view, /contextCustomerID/)
    assert.match(view, /客户账户/)
  }

  assert.match(report, /fetchFinanceReport\(month\.value,\s*contextCustomerID\.value\)/)
  assert.match(report, /fetchFinanceReportDrilldown\(month\.value,\s*contextCustomerID\.value\)/)
  assert.match(report, /financeReportExportUrls\(month\.value,\s*contextCustomerID\.value\)/)
  assert.match(closing, /fetchFinanceReport\(month\.value,\s*contextCustomerID\.value\)/)
  assert.match(closing, /fetchFinanceClosingReview\(month\.value,\s*contextCustomerID\.value\)/)
  assert.match(closing, /v-if="!isWorkspaceCustomerLocked"/)
})
