import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

test('dynamic BOM recipe handles refresh Vue Flow node internals after ports change', () => {
  const source = readFileSync(new URL('../views/ProductCreatorNode.vue', import.meta.url), 'utf8')

  assert.match(source, /useVueFlow/)
  assert.match(source, /updateNodeInternals/)
  assert.match(source, /watch\(inputPorts/)
})
