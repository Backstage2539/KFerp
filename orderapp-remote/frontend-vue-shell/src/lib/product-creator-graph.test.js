import test from 'node:test'
import assert from 'node:assert/strict'
import { reactive } from 'vue'

import { appendGraphSnapshot, autoLayout, cloneValue, connectionIsValid, connectionRoleUpdates, connectionValidation, initialRecipeInputs, moduleForNode, replaceGraphEdge, toCanvasGraph, toWorkflowGraph } from './product-creator-graph.js'

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

test('V3 workflow serializes variable definitions and dynamic BOM recipe targets canonically', () => {
  const sources = Array.from({ length: 4 }, (_, index) => ({ id: `material-${index}`, kind: 'material', name: `来源${index}`, config: { data_role: 'input' } }))
  const workflow = {
    version: 3,
    variables: [{ id: 'product-name', name: '商品名称', default_value: '' }],
    nodes: [...sources, { id: 'bom', kind: 'bom', name: 'BOM组装 1', config: { output_type: 'material', name_parts: [{ type: 'variable', variable_id: 'product-name' }, { type: 'text', value: '_半成品' }] } }],
    edges: sources.map((source, index) => ({ id: `material-edge-${index}`, source: source.id, source_handle: 'material', target: 'bom', target_handle: 'components', kind: 'data' })),
  }

  const canvas = toCanvasGraph(workflow, [
    { kind: 'material', name: '物料', workflow_version: 3, palette_visible: true, inputs: [], outputs: [{ id: 'material', types: ['material.ref'] }] },
    { kind: 'bom', name: 'BOM组装', workflow_version: 3, palette_visible: true, inputs: [{ id: 'components', types: ['material.ref'], multiple: true }], outputs: [] },
  ])
  const bom = canvas.nodes.find((node) => node.id === 'bom')
  assert.equal(bom.data.recipeInputs.length, 5, 'four connected sources leave one add input')
  assert.deepEqual(bom.data.recipeInputs.map((port) => port.label), ['来源0', '来源1', '来源2', '来源3', '＋配方输入'])
  assert.notEqual(canvas.edges[0].targetHandle, canvas.edges[1].targetHandle)

  const saved = toWorkflowGraph(canvas.nodes, canvas.edges, 3, workflow.variables)
  assert.equal(saved.edges.length, 4)
  assert.ok(saved.edges.every((edge) => edge.target_handle === 'components'))
  assert.deepEqual(saved.variables, workflow.variables)
  assert.deepEqual(saved.nodes.find((node) => node.id === 'bom').config.name_parts, workflow.nodes.at(-1).config.name_parts)
})

test('new V3+ BOM nodes start with an empty recipe input handle', () => {
  assert.deepEqual(initialRecipeInputs('bom', 4), [{ id: 'components:add', label: '＋配方输入', add: true }])
  assert.deepEqual(initialRecipeInputs('bom', 3), [{ id: 'components:add', label: '＋配方输入', add: true }])
  assert.deepEqual(initialRecipeInputs('material', 4), [])
  assert.deepEqual(initialRecipeInputs('bom', 2), [])
})

test('V4 product BOM exposes every connected source as a main-input candidate and hides the separate route input', () => {
  const workflow = {
    version: 4,
    nodes: [
      { id: 'raw', kind: 'material', name: '原料', config: { data_role: 'input' } },
      { id: 'semi', kind: 'material', name: '半成品', config: { data_role: 'input' } },
      { id: 'bom', kind: 'bom', name: '成品包装 BOM', config: { output_type: 'product', spec_template_version_id: 91 } },
      { id: 'product', kind: 'product', name: '成品', config: { data_role: 'output' } },
    ],
    edges: [
      { id: 'raw-edge', source: 'raw', source_handle: 'material', target: 'bom', target_handle: 'components', kind: 'data' },
      { id: 'semi-edge', source: 'semi', source_handle: 'material', target: 'bom', target_handle: 'components', kind: 'data' },
      { id: 'output-edge', source: 'bom', source_handle: 'assembly', target: 'product', target_handle: 'from_bom', kind: 'data' },
    ],
  }
  const canvas = toCanvasGraph(workflow, [
    { kind: 'material', name: '物料', workflow_version: 4, inputs: [{ id: 'from_bom' }], outputs: [{ id: 'material', types: ['material.ref'] }] },
    { kind: 'bom', name: 'BOM组装', workflow_version: 4, inputs: [{ id: 'components', label: '配方输入', types: ['material.ref', 'item.specs'], multiple: true }, { id: 'route', label: '工艺路线', types: ['process.route'] }], outputs: [{ id: 'assembly', types: ['bom.output'] }], fields: [{ key: 'variants' }, { key: 'route_id' }, { key: 'spec_template_version_id' }] },
    { kind: 'product', name: '商品', workflow_version: 4, inputs: [{ id: 'from_bom' }], outputs: [{ id: 'product', types: ['product.ref'] }, { id: 'specs', types: ['item.specs'] }] },
  ])
  const bom = canvas.nodes.find((node) => node.id === 'bom')
  assert.deepEqual(bom.data.recipeInputs.map((port) => port.label), ['原料 · 主体候选', '半成品 · 主体候选', '＋配方输入'])
  assert.equal(bom.data.module.inputs.some((port) => port.id === 'route'), false)
  assert.equal(bom.data.module.inputs.find((port) => port.id === 'components').label, '规格主体候选')
  assert.equal(bom.data.module.fields.some((field) => field.key === 'variants' || field.key === 'route_id'), false)
  const saved = toWorkflowGraph(canvas.nodes, canvas.edges, 4)
  assert.ok(saved.edges.every((edge) => edge.target_handle === 'components' || edge.target_handle === 'from_bom'))
})

test('V5 product BOM keeps a distinct process input alongside its specification-template recipe inputs', () => {
  const workflow = {
    version: 5,
    nodes: [
      { id: 'source', kind: 'material', name: '半成品', config: { data_role: 'input' } },
      { id: 'route', kind: 'process', name: '包装工艺', config: { route_id: 9 } },
      { id: 'bom', kind: 'bom', name: '成品包装 BOM', config: { output_type: 'product', spec_template_version_id: 91 } },
    ],
    edges: [
      { id: 'ingredient', source: 'source', source_handle: 'material', target: 'bom', target_handle: 'components', kind: 'data' },
      { id: 'route-edge', source: 'route', source_handle: 'route', target: 'bom', target_handle: 'route', kind: 'data' },
    ],
  }
  const canvas = toCanvasGraph(workflow, [
    { kind: 'material', name: '物料', workflow_version: 5, inputs: [], outputs: [{ id: 'material', types: ['material.ref'] }] },
    { kind: 'process', name: '工艺', workflow_version: 5, inputs: [], outputs: [{ id: 'route', label: '工艺路线', types: ['process.route'] }] },
    { kind: 'bom', name: 'BOM组装', workflow_version: 5, inputs: [{ id: 'components', types: ['material.ref', 'item.specs'], multiple: true }, { id: 'route', label: '工艺路线', types: ['process.route'] }], outputs: [], fields: [{ key: 'route_id' }, { key: 'spec_template_version_id' }] },
  ])
  const bom = canvas.nodes.find((node) => node.id === 'bom')
  assert.equal(bom.data.module.inputs.some((port) => port.id === 'route'), true)
  assert.equal(bom.data.module.inputs.find((port) => port.id === 'components').label, '规格主体候选')
  assert.equal(bom.data.module.fields.some((field) => field.key === 'route_id'), false)
  assert.equal(canvas.edges.find((edge) => edge.id === 'route-edge').targetHandle, 'route')
})

test('V4 specification-template selector is required for product BOMs and absent for material BOMs', () => {
  const modules = [{
    kind: 'bom', name: 'BOM组装', workflow_version: 4, inputs: [], outputs: [],
    fields: [{ key: 'spec_template_version_id', required: false }, { key: 'output_qty', required: true }],
  }]
  const product = moduleForNode({ kind: 'bom', config: { output_type: 'product' } }, modules, 4)
  const material = moduleForNode({ kind: 'bom', config: { output_type: 'material' } }, modules, 4)
  assert.deepEqual(product.fields.find((field) => field.key === 'spec_template_version_id'), { key: 'spec_template_version_id', required: true })
  assert.equal(material.fields.some((field) => field.key === 'spec_template_version_id'), false)
  assert.equal(material.fields.some((field) => field.key === 'output_qty'), true)
})

test('replacing a connected Vue Flow edge always creates a saved-model snapshot with one stable edge', () => {
  const existing = { id: 'recipe-edge', source: 'raw', sourceHandle: 'material', target: 'bom', targetHandle: 'components:source:recipe-edge', data: { kind: 'data' } }
  const staleCollection = [existing]
  const refreshed = replaceGraphEdge(staleCollection, { ...existing, label: '物料对象 → 配方物料或商品规格' })

  assert.notEqual(refreshed, staleCollection, 'the v-model receives a new array even if Vue Flow already inserted this edge')
  assert.equal(refreshed.length, 1, 'replacing the same edge does not duplicate it')
  const saved = toWorkflowGraph([], refreshed, 3)
  assert.equal(saved.edges[0].target_handle, 'components')
  assert.equal(saved.edges[0].label, '物料对象 → 配方物料或商品规格')
})

test('V3 BOM recipe handles accept multiple connections and reject duplicate sources', () => {
  const source = { id: 'material', data: { module: { kind: 'material', outputs: [{ id: 'material', types: ['material.ref'] }] }, label: '云南豆' } }
  const bom = { id: 'bom', data: { module: { kind: 'bom', inputs: [{ id: 'components', types: ['material.ref'], multiple: true }] }, recipeInputs: [{ id: 'components:add', label: '＋配方输入' }] } }
  const nodes = [source, bom]
  const edges = []
  assert.equal(connectionIsValid({ source: 'material', sourceHandle: 'material', target: 'bom', targetHandle: 'components:add' }, nodes, modules), true)
  edges.push({ id: 'edge-1', source: 'material', sourceHandle: 'material', target: 'bom', targetHandle: 'components:source:edge-1', data: { kind: 'data' } })
  assert.equal(connectionIsValid({ source: 'material', sourceHandle: 'material', target: 'bom', targetHandle: 'components:add' }, nodes, modules, 'data', edges), false)
  assert.equal(connectionIsValid({ id: 'edge-1', source: 'material', sourceHandle: 'material', target: 'bom', targetHandle: 'components:source:edge-1' }, nodes, modules, 'data', edges), true, 'revalidating an edge must not reject the edge itself as a duplicate')
  assert.equal(connectionIsValid({ source: 'material', sourceHandle: 'material', target: 'bom', targetHandle: 'route' }, nodes, modules), false)
})

test('a BOM route input gives a precise message when a second process route is attempted', () => {
  const process = { kind: 'process', outputs: [{ id: 'route', types: ['process.route'] }] }
  const bomModule = { kind: 'bom', inputs: [{ id: 'route', label: '工艺路线', types: ['process.route'] }] }
  const nodes = [
    { id: 'first-route', data: { module: process, label: '半成品烘焙路线' } },
    { id: 'second-route', data: { module: process, label: '成品包装路线' } },
    { id: 'bom', data: { module: bomModule, label: '半成品烘焙 BOM' } },
  ]
  const edges = [{ id: 'existing-route', source: 'first-route', sourceHandle: 'route', target: 'bom', targetHandle: 'route', data: { kind: 'data' } }]
  const result = connectionValidation({ source: 'second-route', sourceHandle: 'route', target: 'bom', targetHandle: 'route' }, nodes, [], 'data', edges)
  assert.equal(result.valid, false)
  assert.equal(result.code, 'multiple_routes')
  assert.equal(result.message, '半成品烘焙 BOM 已连接“半成品烘焙路线”，一个 BOM 只能连接一个工艺。请先删除原连线再更换。')
})

test('a graph connection that closes a cycle is rejected with a cycle explanation', () => {
  const material = { kind: 'material', inputs: [{ id: 'from_bom', types: ['material.ref'] }], outputs: [{ id: 'material', types: ['material.ref'] }] }
  const bomModule = { kind: 'bom', inputs: [{ id: 'components', types: ['material.ref'], multiple: true }], outputs: [{ id: 'output', types: ['material.ref'] }] }
  const nodes = [
    { id: 'material', data: { module: material, label: '物料' } },
    { id: 'bom', data: { module: bomModule, label: 'BOM' } },
  ]
  const edges = [{ id: 'existing', source: 'bom', sourceHandle: 'output', target: 'material', targetHandle: 'from_bom', data: { kind: 'data' } }]
  const result = connectionValidation({ source: 'material', sourceHandle: 'material', target: 'bom', targetHandle: 'components:add' }, nodes, [], 'data', edges)
  assert.equal(result.valid, false)
  assert.equal(result.code, 'cycle')
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

test('product BOM components require and accept the product specification output', () => {
  const productModule = {
    kind: 'product',
    workflow_version: 7,
    outputs: [
      { id: 'product', label: '商品对象', types: ['product.ref'] },
      { id: 'specs', label: '商品规格', types: ['item.specs'] },
    ],
  }
  const bomModule = {
    kind: 'bom',
    workflow_version: 7,
    inputs: [{ id: 'components', label: '配方物料或商品规格', types: ['material.ref', 'item.specs'], multiple: true }],
  }
  const nodes = [
    { id: 'ear-drip-product', data: { module: productModule, label: '10g袋装挂耳' } },
    { id: 'boxing-bom', data: { module: bomModule, label: '挂耳包装成盒 BOM' } },
  ]

  const wrongOutput = connectionValidation({ source: 'ear-drip-product', sourceHandle: 'product', target: 'boxing-bom', targetHandle: 'components:add' }, nodes)
  assert.equal(wrongOutput.valid, false)
  assert.equal(wrongOutput.code, 'product_object_requires_specification')
  assert.match(wrongOutput.message, /商品规格.*具体规格/)

  const productSpecification = connectionValidation({ source: 'ear-drip-product', sourceHandle: 'specs', target: 'boxing-bom', targetHandle: 'components:add' }, nodes)
  assert.equal(productSpecification.valid, true)
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

test('automatic layout keeps adjacent columns far enough apart to expose connection handles', () => {
  const nodes = ['material', 'process', 'bom', 'output'].map((id) => ({
    id,
    position: { x: 0, y: 0 },
    data: { label: id },
  }))
  const edges = [
    { source: 'material', target: 'bom' },
    { source: 'process', target: 'bom' },
    { source: 'bom', target: 'output' },
  ]

  const layout = autoLayout(nodes, edges)
  const x = Object.fromEntries(layout.map((node) => [node.id, node.position.x]))
  assert.ok(x.bom - x.material >= 300, 'source and BOM cards must not overlap across the input handles')
  assert.ok(x.output - x.bom >= 300, 'BOM and output cards must not overlap across the output handles')
})

test('BOM workflow graph renders material sources before assembly and BOM output on generated objects', () => {
  const workflow = {
    version: 2,
    nodes: [
      { id: 'raw', kind: 'material', config: { data_role: 'output' } },
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
  assert.equal(canvas.nodes.find((node) => node.id === 'raw').data.module.inputs[0].id, 'from_bom')
  assert.equal(canvas.nodes.find((node) => node.id === 'semi').data.module.inputs[0].id, 'from_bom')
  assert.equal(connectionIsValid({ source: 'semi-bom', sourceHandle: 'assembly', target: 'raw', targetHandle: 'from_bom' }, canvas.nodes, catalog), true)
  const saved = toWorkflowGraph(canvas.nodes, canvas.edges, 2)
  assert.equal(saved.version, 2)
  assert.equal(saved.edges[1].source_handle, 'assembly')
})

test('data connections infer whether product and material nodes are BOM outputs or recipe inputs', () => {
  const nodes = [
    { id: 'bom', data: { module: { kind: 'bom' } } },
    { id: 'semi', data: { module: { kind: 'material' }, config: { data_role: 'output' } } },
    { id: 'component', data: { module: { kind: 'material' }, config: { data_role: 'output' } } },
    { id: 'finish', data: { module: { kind: 'bom' } } },
  ]
  assert.deepEqual(connectionRoleUpdates({ source: 'bom', sourceHandle: 'assembly', target: 'semi', targetHandle: 'from_bom' }, nodes), { semi: 'output' })
  assert.deepEqual(connectionRoleUpdates({ source: 'component', sourceHandle: 'material', target: 'finish', targetHandle: 'components' }, nodes), { component: 'input' })
  assert.deepEqual(connectionRoleUpdates(
    { source: 'semi', sourceHandle: 'material', target: 'finish', targetHandle: 'components' },
    nodes,
    'data',
    [{ source: 'bom', target: 'semi', targetHandle: 'from_bom', data: { kind: 'data' } }],
  ), {})
  assert.deepEqual(connectionRoleUpdates({ source: 'bom', sourceHandle: '__prerequisite', target: 'semi', targetHandle: '__prerequisite' }, nodes, 'prerequisite'), {})
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

test('V6 material nodes expose BOM output ports but reject purchased or multi-row output defaults', () => {
  const modulesV6 = [
    { kind: 'material', name: '物料', workflow_version: 6, inputs: [{ id: 'from_bom', types: ['bom.output'] }], outputs: [{ id: 'material', types: ['material.ref'] }] },
    { kind: 'bom', name: 'BOM组装', workflow_version: 6, inputs: [{ id: 'components', types: ['material.ref'], multiple: true }, { id: 'route', types: ['process.route'] }], outputs: [{ id: 'assembly', types: ['bom.output'] }] },
  ]
  const bom = { id: 'bom', type: 'business', data: { module: modulesV6[1], config: { output_type: 'material' } } }
  const material = (rows) => ({ id: 'raw', type: 'business', data: { module: modulesV6[0], label: '投入物料', config: { data_role: 'input', default_rows: rows } } })
  const connection = { source: 'bom', sourceHandle: 'assembly', target: 'raw', targetHandle: 'from_bom' }
  assert.equal(connectionValidation(connection, [bom, material([{ row_id: 'raw-1', supply_mode: 'purchase' }])], modulesV6).code, 'purchased_material_output')
  assert.equal(connectionValidation(connection, [bom, material([{ row_id: 'raw-1', supply_mode: 'manufacture' }, { row_id: 'raw-2', supply_mode: 'manufacture' }])], modulesV6).code, 'multirow_material_output')
  assert.equal(connectionIsValid(connection, [bom, material([{ row_id: 'semi', supply_mode: 'manufacture' }])], modulesV6), true)
})
