import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

test('dynamic BOM recipe handles refresh Vue Flow node internals after ports change', () => {
  const source = readFileSync(new URL('../views/ProductCreatorNode.vue', import.meta.url), 'utf8')

  assert.match(source, /useVueFlow/)
  assert.match(source, /updateNodeInternals/)
  assert.match(source, /watch\(inputPorts/)
})

test('new connections are added through Vue Flow so the rendered graph and template model stay in sync', () => {
  const source = readFileSync(new URL('../views/ProductCreatorView.vue', import.meta.url), 'utf8')
  const connectNodes = source.match(/function connectNodes\(connection\) \{([\s\S]*?)\n\}\n\nfunction selectNode/)?.[1]

  assert.ok(connectNodes, 'connectNodes handler should remain easy to inspect')
  assert.match(source.match(/const \{ ([^\n]+) \} = useVueFlow\(\)/)?.[1] || '', /addEdges/)
  assert.match(connectNodes, /addEdges\(\[edge\]\)/)
  assert.match(connectNodes, /edges\.value\s*=\s*replaceGraphEdge\(edges\.value, edge\)/)
  assert.doesNotMatch(connectNodes, /if\s*\(!edges\.value\.some\(/)
  assert.doesNotMatch(connectNodes, /edges\.value\.push\(/)
})
