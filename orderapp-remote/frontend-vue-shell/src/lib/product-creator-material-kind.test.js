import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const designer = readFileSync(new URL('../views/ProductCreatorView.vue', import.meta.url), 'utf8')
const runtime = readFileSync(new URL('../views/ProductCreatorRunView.vue', import.meta.url), 'utf8')

test('BOM-centric material selectors only offer kinds accepted by the material domain', () => {
  const templateSelect = designer.match(/<label class="pc-field-label">物料类别与取得方式<\/label>\s*<div class="pc-inline-controls">\s*<select[^>]*>([\s\S]*?)<\/select>/)
  const runtimeSelect = runtime.match(/<label><span>物料类别<\/span><select[^>]*>([\s\S]*?)<\/select>/)

  assert.ok(templateSelect, 'template designer should expose material kind defaults')
  assert.ok(runtimeSelect, 'run form should expose material kind')
  for (const options of [templateSelect[1], runtimeSelect[1]]) {
    assert.match(options, /value="other"/)
    assert.match(options, /value="bean"/)
    assert.match(options, /value="pack"/)
    assert.doesNotMatch(options, /value="roasted"/)
  }
})
