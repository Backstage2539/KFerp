import test from 'node:test'
import assert from 'node:assert/strict'
import { filterWorkflowVariables, renderNameParts, renderNamePreview, upgradeWorkflowToV3, upgradeWorkflowToV4, upgradeWorkflowToV5, upgradeWorkflowToV6 } from './product-creator-variables.js'

test('variable search matches substrings and ordered fuzzy terms and permits adding the missing value', () => {
  const variables = [
    { id: 'product-name', name: '商品名称', default_value: '' },
    { id: 'batch-code', name: '批次编号', default_value: '' },
  ]
  assert.deepEqual(filterWorkflowVariables('商品名', variables).map((row) => row.id), ['product-name'])
  assert.deepEqual(filterWorkflowVariables('批编', variables).map((row) => row.id), ['batch-code'])
  assert.deepEqual(filterWorkflowVariables('  ', variables), variables)
})

test('name parts combine literal text and one run-time variable value', () => {
  const result = renderNameParts([
    { type: 'variable', variable_id: 'product-name' },
    { type: 'text', value: '_半成品' },
  ], [{ id: 'product-name', name: '商品名称' }], { 'product-name': '云南日晒豆' })
  assert.deepEqual(result, { value: '云南日晒豆_半成品', missing: [] })
  assert.deepEqual(renderNameParts([{ type: 'variable', variable_id: 'product-name' }], [{ id: 'product-name' }], {}), {
    value: '', missing: ['product-name'],
  })
})

test('template naming preview combines literals with preview samples, prefers defaults, and marks missing variables', () => {
  const parts = [
    { type: 'text', value: '半成品-烘焙豆_' },
    { type: 'variable', variable_id: 'product-name' },
    { type: 'text', value: '_' },
    { type: 'variable', variable_id: 'batch' },
  ]
  const variables = [
    { id: 'product-name', name: '商品名', default_value: '默认商品' },
    { id: 'batch', name: '批次', default_value: '' },
  ]

  assert.deepEqual(renderNamePreview(parts, variables), {
    value: '半成品-烘焙豆_默认商品_【批次待填写】', missing: ['batch'],
  })
  assert.deepEqual(renderNamePreview(parts, variables, { 'product-name': '春季新品', batch: 'A01' }), {
    value: '半成品-烘焙豆_春季新品_A01', missing: [],
  })
})

test('V2 workflow upgrade preserves literals and converts old product-name placeholders to stable shared variables', () => {
  const upgraded = upgradeWorkflowToV3({
    version: 2,
    nodes: [
      { id: 'semi', kind: 'material', config: { data_role: 'output', name_pattern: '{{商品名称}}_半成品', fixed_fields: ['name', 'output_qty'], kind: 'bean' } },
      { id: 'finished', kind: 'product', config: { data_role: 'output', name_pattern: '{{商品名称}}_成品', fixed_fields: ['product_kind'], product_kind: 'roasted' } },
    ],
    edges: [{ source: 'semi', target: 'finished', source_handle: 'material', target_handle: 'components' }],
  })
  assert.equal(upgraded.version, 3)
  assert.equal(upgraded.variables.length, 1)
  assert.equal(upgraded.variables[0].name, '商品名称')
  assert.deepEqual(upgraded.nodes[0].config.name_parts, [
    { type: 'variable', variable_id: upgraded.variables[0].id }, { type: 'text', value: '_半成品' },
  ])
  assert.ok(upgraded.nodes.every((node) => !node.config.fixed_fields && !node.config.name_pattern))
  assert.equal(upgraded.nodes[0].config.kind, undefined)
  assert.equal(upgraded.nodes[1].config.product_kind, undefined)
  assert.equal(upgraded.edges[0].id, 'legacy-edge-1')
})

test('V3 product BOM migration preserves hand-maintained settings for explicit review and V4 snapshots stay immutable', () => {
  const legacy = {
    version: 3,
    variables: [{ id: 'product-name', name: '商品名称', default_value: '' }],
    nodes: [
      { id: 'semi-bom', kind: 'bom', config: { output_type: 'material', output_qty: 5, route_id: 8, variants: [], components: [{ row_id: 'raw-1', quantity: 2 }] } },
      { id: 'product-bom', kind: 'bom', config: { output_type: 'product', output_qty: 1, output_unit: 'bag', route_id: 9, material_loss_rate: 0.12, variants: [{ row_id: 'spec-a', name: '250g', unit: 'bag' }], components: [{ row_id: 'pack-a', source_node_id: 'pack', quantity: 1, unit: 'bag' }] } },
      { id: 'product', kind: 'product', config: { data_role: 'output', name_parts: [{ type: 'variable', variable_id: 'product-name' }] } },
    ],
    edges: [{ id: 'route-edge', source: 'route', target: 'product-bom', target_handle: 'route' }],
  }
  const upgraded = upgradeWorkflowToV4(legacy)
  const productBOM = upgraded.nodes.find((node) => node.id === 'product-bom')
  const materialBOM = upgraded.nodes.find((node) => node.id === 'semi-bom')

  assert.equal(upgraded.version, 4)
  assert.equal(productBOM.config.spec_template_version_id, 0)
  assert.equal(productBOM.config.legacy_spec_configuration_pending, true)
  assert.deepEqual(productBOM.config.legacy_spec_configuration.variants, legacy.nodes[1].config.variants)
  assert.deepEqual(productBOM.config.legacy_spec_configuration.components, legacy.nodes[1].config.components)
  assert.equal(productBOM.config.variants, undefined)
  assert.equal(productBOM.config.components, undefined)
  assert.equal(materialBOM.config.output_qty, 5)
  assert.deepEqual(upgraded.edges, legacy.edges)
  assert.deepEqual(upgradeWorkflowToV4(upgraded), upgraded)
  assert.equal(legacy.nodes[1].config.output_unit, 'bag')
})

test('V5 migration preserves all V4 graph connections for review under the new process-route rules', () => {
  const v4 = {
    version: 4,
    variables: [],
    nodes: [{ id: 'bom', kind: 'bom', name: '成品包装 BOM', config: { output_type: 'product', spec_template_version_id: 91 } }],
    edges: [{ id: 'route-edge', source: 'route', source_handle: 'route', target: 'bom', target_handle: 'route', kind: 'data' }],
  }
  const upgraded = upgradeWorkflowToV5(v4)
  assert.equal(upgraded.version, 5)
  assert.deepEqual(upgraded.edges, v4.edges)
  assert.equal(upgraded.nodes[0].config.spec_template_version_id, 91)
  assert.deepEqual(upgradeWorkflowToV5(upgraded), upgraded)
})

test('V6 migration moves input material defaults into stable rows and preserves legacy purchase nodes for review', () => {
  const legacy = {
    version: 5,
    variables: [],
    nodes: [
      { id: 'raw', kind: 'material', config: { data_role: 'input', rows: [{ row_id: 'raw-1', name: '生豆', supply_mode: 'purchase', unit: 'kg' }] } },
      { id: 'purchase', kind: 'purchase', config: { supplier_id: 1 } },
    ],
    edges: [{ id: 'edge-purchase', source: 'raw', target: 'purchase', target_handle: 'material' }],
  }
  const upgraded = upgradeWorkflowToV6(legacy)
  assert.equal(upgraded.version, 6)
  assert.deepEqual(upgraded.nodes[0].config.default_rows, legacy.nodes[0].config.rows)
  assert.equal(upgraded.nodes[0].config.rows, undefined)
  assert.deepEqual(upgraded.nodes[1], legacy.nodes[1])
  assert.deepEqual(upgraded.edges, legacy.edges)
  assert.deepEqual(upgradeWorkflowToV6(upgraded), upgraded)
})
