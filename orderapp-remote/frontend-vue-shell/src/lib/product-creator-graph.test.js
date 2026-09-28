import test from 'node:test'
import assert from 'node:assert/strict'
import { reactive } from 'vue'

import { appendGraphSnapshot, autoLayout, cloneValue, connectionIsValid, toCanvasGraph, toWorkflowGraph } from './product-creator-graph.js'

const modules = [
  { kind: 'product', name: '商品档案', inputs: [], outputs: [{ id: 'product', types: ['product.ref'] }], fields: [] },
  { kind: 'material', name: '物料档案', inputs: [], outputs: [{ id: 'material', types: ['material.ref'] }], fields: [] },
  { kind: 'bom', name: 'BOM 与规格', inputs: [{ id: 'output', types: ['product.ref', 'material.ref'] }], outputs: [{ id: 'bom', types: ['bom.draft'] }], fields: [] },
  { kind: 'publish', name: '发布与默认绑定', inputs: [{ id: 'bom', types: ['bom.draft'] }], outputs: [], fields: [] },
  { kind: 'pricing', name: '价格配置', inputs: [], outputs: [], fields: [] },
]

test('graph history cloning accepts Vue reactive node and module values', () => {
  const graph = reactive({ nodes: [{ id: 'product', data: { module: modules[0], config: { action: 'create' } } }], edges: [] })
  const snapshot = cloneValue(graph)
  assert.deepEqual(snapshot.nodes[0].data.module, modules[0])
  assert.equal(snapshot.nodes[0].data.config.action, 'create')
})

test('graph history records edits so undo, redo and editing after undo use the right snapshots', () => {
  const empty = { nodes: [], edges: [] }
  const withProduct = { nodes: [{ id: 'product' }], edges: [] }
  const withMaterial = { nodes: [{ id: 'product' }, { id: 'material' }], edges: [] }
  let state = appendGraphSnapshot([empty], 0, withProduct)
  assert.equal(state.index, 1)
  assert.deepEqual(state.history[state.index - 1], empty)
  state = appendGraphSnapshot(state.history, state.index, withMaterial)
  assert.equal(state.index, 2)
  assert.deepEqual(state.history[state.index - 1], withProduct)
  const editedAfterUndo = appendGraphSnapshot(state.history, state.index - 1, { nodes: [{ id: 'assembly' }], edges: [] })
  assert.deepEqual(editedAfterUndo.history, [empty, withProduct, { nodes: [{ id: 'assembly' }], edges: [] }])
  assert.equal(editedAfterUndo.index, 2)
})

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

test('BOM workflow graph renders material sources before assembly and BOM output on generated objects', () => {
  const workflow = {
    version: 2,
    nodes: [
      { id: 'raw', kind: 'material', config: { data_role: 'input' } },
      { id: 'semi-bom', kind: 'bom', config: { output_type: 'material', output_qty: 1, output_unit: 'kg' } },
      { id: 'semi', kind: 'material', config: { data_role: 'output' } },
    ],
    edges: [
      { id: 'ingredient', source: 'raw', source_handle: 'material', target: 'semi-bom', target_handle: 'components', kind: 'data' },
      { id: 'output', source: 'semi-bom', source_handle: 'assembly', target: 'semi', target_handle: 'from_bom', kind: 'data' },
    ],
  }
  const catalog = [
    { kind: 'material', name: '物料档案', workflow_version: 1, palette_visible: false, inputs: [], outputs: [{ id: 'material', types: ['material.ref'] }] },
    { kind: 'material', name: '物料', category: '数据类型', workflow_version: 2, palette_visible: true, inputs: [], outputs: [{ id: 'material', types: ['material.ref'] }] },
    { kind: 'bom', name: 'BOM组装', category: '动作', workflow_version: 2, palette_visible: true, inputs: [{ id: 'components', types: ['material.ref'] }], outputs: [{ id: 'assembly', types: ['bom.output'] }] },
  ]
  const canvas = toCanvasGraph(workflow, catalog)
  assert.equal(canvas.nodes.find((node) => node.id === 'raw').data.module.name, '物料')
  assert.deepEqual(canvas.nodes.find((node) => node.id === 'raw').data.module.inputs, [])
  assert.equal(canvas.nodes.find((node) => node.id === 'semi').data.module.inputs[0].id, 'from_bom')
  const saved = toWorkflowGraph(canvas.nodes, canvas.edges, 2)
  assert.equal(saved.version, 2)
  assert.equal(saved.edges[1].source_handle, 'assembly')
})

test('new template catalog hides advanced and pricing nodes while preserving legacy module rendering', () => {
  const catalog = [
    ...modules.map((module) => ({ ...module, workflow_version: 1, palette_visible: false })),
    { kind: 'material', name: '物料', category: '数据类型', workflow_version: 2, palette_visible: true, inputs: [], outputs: [{ id: 'material', types: ['material.ref'] }] },
    { kind: 'product', name: '商品', category: '数据类型', workflow_version: 2, palette_visible: true, inputs: [], outputs: [{ id: 'product', types: ['product.ref'] }] },
    { kind: 'bom', name: 'BOM组装', category: '动作', workflow_version: 2, palette_visible: true, inputs: [], outputs: [{ id: 'assembly', types: ['bom.output'] }] },
    { kind: 'purchase', name: '物料购入', category: '动作', workflow_version: 2, palette_visible: true, inputs: [], outputs: [{ id: 'purchase', types: ['purchase.order'] }] },
    { kind: 'process', name: '工艺', category: '数据类型', workflow_version: 2, palette_visible: true, inputs: [], outputs: [{ id: 'route', types: ['process.route'] }] },
  ]
  const visible = catalog.filter((module) => module.palette_visible)
  assert.deepEqual(visible.map(({ kind }) => kind).sort(), ['bom', 'material', 'process', 'product', 'purchase'])
  const legacy = toCanvasGraph({ nodes: [{ id: 'old-pricing', kind: 'pricing' }], edges: [] }, catalog)
  assert.equal(legacy.nodes[0].data.module.workflow_version, 1)
})
