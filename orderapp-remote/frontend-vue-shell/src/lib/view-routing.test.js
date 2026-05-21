import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

test('costing view routes to the bean list and pricing workspace', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  assert.match(source, /import\s+CostingView\s+from\s+['"]\.\/views\/CostingView\.vue['"]/)
  assert.match(source, /\bcosting:\s*CostingView\b/)
})

test('delivery note date field keeps ISO date text visible to Chinese operators', () => {
  const source = readFileSync(new URL('../views/DeliveryNoteView.vue', import.meta.url), 'utf8')
  assert.doesNotMatch(source, /v-model\.trim="form\.posting_date"\s+type="date"/)
  assert.match(source, /v-model\.trim="form\.posting_date"[^>]+placeholder="YYYY-MM-DD"/)
})

test('sidebar navigation sanitizes stale edit identifiers before switching views', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
  assert.match(source, /viewNavigationURL/)
  assert.match(source, /replaceHistoryURL\(applyWorkspaceToUrl\(viewNavigationURL\(url,\s*key,\s*workspaceViewParams\(params,\s*workspaceContext\(\)\)\)\)\)/)
})

test('customer fulfillment workbench renders sections from capability helpers', () => {
  const source = readFileSync(new URL('../views/CustomerFulfillmentView.vue', import.meta.url), 'utf8')
  assert.match(source, /visibleImportTypes/)
  assert.match(source, /visibleImports/)
  assert.match(source, /workbenchSections\.processing/)
  assert.match(source, /workbenchSections\.directShip/)
  assert.match(source, /workbenchSections\.inventory/)
  assert.match(source, /workbenchSections\.settlement/)
})

test('orders view exposes recipient snapshots and fee breakdowns', () => {
  const source = readFileSync(new URL('../views/OrdersView.vue', import.meta.url), 'utf8')
  assert.match(source, /收件信息/)
  assert.match(source, /activeOrderDetail\.receiver_name/)
  assert.match(source, /orderFeeLines\(row\)/)
  assert.match(source, /customerFulfillmentOrderFees/)
  assert.match(source, /emphasized/)
  assert.match(source, /委外合计/)
  assert.match(source, /outsource_total_fee/)
})

test('orders view exposes irreversible invalidation and copy through the shared orders API', () => {
  const source = readFileSync(new URL('../views/OrdersView.vue', import.meta.url), 'utf8')
  assert.match(source, /失效/)
  assert.match(source, /voidOrder\(row\)/)
  assert.match(source, /copyOrder\(row\)/)
  assert.match(source, /voidSelectedOrders/)
  assert.match(source, /批量失效/)
  assert.match(source, /togglePageVoidSelection/)
  assert.match(source, /allVisibleVoidableOrdersSelected/)
  assert.match(source, /当前页正常订单全选/)
  assert.doesNotMatch(source, /选择本页正常订单/)
  assert.doesNotMatch(source, /selectVisibleVoidableOrders/)
  assert.match(source, /:copy-id=/)
  assert.match(source, /已失效/)
  assert.match(source, /失效后不可恢复/)
  assert.match(source, /`\/api\/orders\/\$\{id\}\/void`/)
  assert.match(source, /`\/api\/orders\/void`/)
  assert.doesNotMatch(source, /restoreOrder/)
  assert.doesNotMatch(source, /unvoid/)
})

test('order entry supports copying an existing order without editing the source order', () => {
  const source = readFileSync(new URL('../views/OrderEntryView.vue', import.meta.url), 'utf8')
  assert.match(source, /copyId/)
  assert.match(source, /copy_id/)
  assert.match(source, /复制订单/)
  assert.match(source, /copyMode/)
  assert.match(source, /edit_id:\s*copyID \? 0/)
})
