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
