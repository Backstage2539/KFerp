import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { appendDuplicatedBOMComponentRow } from './product-creator-component-rows.js'

const runtime = readFileSync(new URL('../views/ProductCreatorRunView.vue', import.meta.url), 'utf8')

test('duplicating a BOM component keeps its source and recipe values but gives the row a new identity', () => {
  const source = {
    row_id: 'component-200g',
    source_node_id: 'semi-material',
    source_row_id: 'output',
    quantity: 200,
    unit: 'g',
    variant_row_id: 'spec-200g',
    loss_rate: 0,
  }

  const rows = [source]
  const duplicatedRows = appendDuplicatedBOMComponentRow(rows, source, 'component-500g')

  assert.deepEqual(duplicatedRows, [source, { ...source, row_id: 'component-500g' }])
  assert.notEqual(duplicatedRows[1].row_id, source.row_id)
  assert.equal(rows.length, 1)
  assert.equal(source.row_id, 'component-200g')
})

test('the BOM run form exposes duplication on each recipe row', () => {
  assert.match(runtime, /aria-label="复制配方行"[^>]*@click="duplicateComponentRow\(node\.id, component\)"/)
  assert.match(runtime, /function duplicateComponentRow\(nodeID, component\)[\s\S]*?valuesFor\(nodeID\)\.components = appendDuplicatedBOMComponentRow\(ensureComponents\(nodeID\), component, makeNodeId\(\)\)/)
})
