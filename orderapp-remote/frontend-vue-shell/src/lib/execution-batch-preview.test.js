import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {parse} from '@vue/compiler-sfc'

test('prestart batch preview reads persisted or legacy preview rows without writing', () => {
  const source = readFileSync(new URL('../components/ExecutionBatchPreview.vue', import.meta.url), 'utf8')
  const {descriptor, errors} = parse(source)
  assert.deepEqual(errors, [])
  assert.match(descriptor.scriptSetup.content, /apiGet\(`/)
  assert.doesNotMatch(descriptor.scriptSetup.content, /apiSend|fetch\(/)
  for (const term of ['preview_batches', 'planned_input_qty', 'planned_output_qty', 'planned_minutes']) assert.ok(source.includes(term), term)
  assert.match(source, /@media\s*\(max-width:\s*760px\)/)
})
