import test from 'node:test'
import assert from 'node:assert/strict'

import { filterReceiptMaterials, selectableReceiptMaterials } from './material-receipts.js'

const materials = [
  { id: 1, code: 'RAW-ETH-001', name: '埃塞俄比亚 水洗 耶加雪菲', kind: 'bean' },
  { id: 2, code: 'RAW-COL-002', name: '哥伦比亚 蕙兰', Kind: 'bean' },
  { id: 3, code: 'BAG-454', name: '454g 豆袋', kind: 'pack' },
]

test('selectableReceiptMaterials excludes packaging materials from raw receipt choices', () => {
  const got = selectableReceiptMaterials(materials)

  assert.deepEqual(got.map((row) => row.id), [1, 2])
})

test('filterReceiptMaterials fuzzy matches by material name and code', () => {
  assert.deepEqual(filterReceiptMaterials(materials, '哥伦').map((row) => row.id), [2])
  assert.deepEqual(filterReceiptMaterials(materials, 'raw eth').map((row) => row.id), [1])
  assert.deepEqual(filterReceiptMaterials(materials, '454').map((row) => row.id), [])
})
