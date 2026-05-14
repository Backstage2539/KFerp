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
  assert.match(source, /replaceHistoryURL\(viewNavigationURL\(url,\s*key,\s*params\)\)/)
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
