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
  assert.match(source, /replaceHistoryURL\(applyViewContextToUrl\(viewNavigationURL\(url,\s*key,\s*viewContextViewParams\(params,\s*currentViewContext\.value\)\)\)\)/)
})

test('KFerp Vue workflow documents return navigation for cross-page jumps', () => {
  const skill = readFileSync(new URL('../../../../.agents/skills/kferp-vue-change/SKILL.md', import.meta.url), 'utf8')
  const appSource = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  assert.match(skill, /跨页面跳转/)
  assert.match(skill, /returnNavigation/)
  assert.match(skill, /刷新后消失/)
  assert.match(appSource, /transientReturnNavigation/)
  assert.match(appSource, /kferp:navigate-view/)
})

test('vue shell confines sidebar/content scrolling, returns routed pages to top, and supports mobile swipe menu', () => {
  const source = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')

  for (const marker of [
    'ref="content"',
    'scrollCurrentViewToTop',
    'content.value.scrollTo',
    'handleTouchStart',
    'handleTouchEnd',
    'touchStartX',
    'mobileSwipeMinDistance',
    "window.addEventListener('touchstart', handleTouchStart",
    "window.addEventListener('touchend', handleTouchEnd",
    "window.removeEventListener('touchstart', handleTouchStart",
    "window.removeEventListener('touchend', handleTouchEnd",
  ]) {
    assert.ok(source.includes(marker), `App.vue should include ${marker}`)
  }

  assert.match(source, /function open\(key,\s*params\s*=\s*\{\}(?:,\s*options\s*=\s*\{\})?\)[\s\S]*scrollCurrentViewToTop\(\)/)
  assert.match(source, /function setCurrentViewContext\(context,[\s\S]*scrollCurrentViewToTop\(\)/)
  assert.match(source, /\.layout\s*\{[^}]*height:\s*100vh;[^}]*overflow:\s*hidden;/)
  assert.match(source, /\.sidebar\s*\{[^}]*height:\s*100vh;[^}]*overflow-y:\s*auto;[^}]*overscroll-behavior:\s*contain;/)
  assert.match(source, /\.content\s*\{[^}]*height:\s*100vh;[^}]*overflow-y:\s*auto;[^}]*overscroll-behavior:\s*contain;/)
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
  assert.match(source, /row\.document_date/)
  assert.match(source, /单据：/)
  assert.match(source, /订单：/)
  assert.match(source, /activeOrderDetail\?\.document_date/)
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
  assert.match(source, /togglePageOrderSelection/)
  assert.match(source, /pageSelectionState\.indeterminate/)
  assert.match(source, /当前页正常订单全选/)
  assert.match(source, /selectedVoidableOrderIDs/)
  assert.doesNotMatch(source, /选择本页正常订单/)
  assert.doesNotMatch(source, /selectVisibleVoidableOrders/)
  assert.doesNotMatch(source, /togglePageVoidSelection/)
  assert.doesNotMatch(source, /allVisibleVoidableOrdersSelected/)
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
