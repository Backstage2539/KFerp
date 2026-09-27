import test from 'node:test'
import assert from 'node:assert/strict'

import { autoLayout, connectionIsValid, toCanvasGraph, toWorkflowGraph } from './product-creator-graph.js'

const modules = [
  { kind: 'product', name: '商品档案', inputs: [], outputs: [{ id: 'product', types: ['product.ref'] }], fields: [] },
  { kind: 'material', name: '物料档案', inputs: [], outputs: [{ id: 'material', types: ['material.ref'] }], fields: [] },
  { kind: 'bom', name: 'BOM 与规格', inputs: [{ id: 'output', types: ['product.ref', 'material.ref'] }], outputs: [{ id: 'bom', types: ['bom.draft'] }], fields: [] },
  { kind: 'publish', name: '发布与默认绑定', inputs: [{ id: 'bom', types: ['bom.draft'] }], outputs: [], fields: [] },
]

test('workflow graph survives canvas conversion with stable node, edge and row identifiers', () => {
  const workflow = {
    nodes: [{ id: 'product-a', kind: 'product', name: '商品', x: 52, y: 80, config: { rows: [{ row_id: 'variant-a' }] }, condition: { node_id: 'source', field: 'action', operator: 'equals', value: 'create' } }],
    edges: [{ id: 'edge-a', source: 'product-a', source_handle: 'product', target: 'bom-a', target_handle: 'output', kind: 'data', label: '产出对象' }],
  }
  const canvas = toCanvasGraph(workflow, modules)
  assert.equal(canvas.nodes[0].id, 'product-a')
  assert.equal(canvas.nodes[0].data.config.rows[0].row_id, 'variant-a')
  const saved = toWorkflowGraph(canvas.nodes, canvas.edges)
  assert.equal(saved.nodes[0].x, 52)
  assert.deepEqual(saved.nodes[0].condition, workflow.nodes[0].condition)
  assert.equal(saved.edges[0].id, 'edge-a')
  assert.equal(saved.edges[0].target_handle, 'output')
})

test('connections reject wrong business types and accept compatible data ports', () => {
  const nodes = [
    { id: 'product', data: { module: modules[0] } },
    { id: 'material', data: { module: modules[1] } },
    { id: 'bom', data: { module: modules[2] } },
  ]
  assert.equal(connectionIsValid({ source: 'product', sourceHandle: 'product', target: 'bom', targetHandle: 'output' }, nodes, modules), true)
  assert.equal(connectionIsValid({ source: 'material', sourceHandle: 'material', target: 'bom', targetHandle: 'output' }, nodes, modules), true)
  assert.equal(connectionIsValid({ source: 'product', sourceHandle: 'product', target: 'bom', targetHandle: 'missing' }, nodes, modules), false)
  assert.equal(connectionIsValid({ source: 'product', sourceHandle: 'product', target: 'product', targetHandle: 'product' }, nodes, modules), false)
})

test('automatic layout orders a multi-level graph from sources to downstream steps', () => {
  const nodes = ['leaf', 'semi', 'pack', 'final'].map((id) => ({ id, position: { x: 0, y: 0 }, data: { label: id } }))
  const edges = [
    { source: 'leaf', target: 'semi' },
    { source: 'semi', target: 'final' },
    { source: 'pack', target: 'final' },
  ]
  const layout = autoLayout(nodes, edges)
  const x = Object.fromEntries(layout.map((node) => [node.id, node.position.x]))
  assert.ok(x.leaf < x.semi && x.semi < x.final)
  assert.ok(x.pack < x.final)
})
