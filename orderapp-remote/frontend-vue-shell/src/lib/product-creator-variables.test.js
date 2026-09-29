import test from 'node:test'
import assert from 'node:assert/strict'
import { filterWorkflowVariables, renderNameParts, upgradeWorkflowToV3 } from './product-creator-variables.js'

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
