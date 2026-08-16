import test from 'node:test'
import assert from 'node:assert/strict'

import {
  filterReceiptMaterials,
  isSemiFinishedMaterial,
  selectableReceiptMaterials,
  selectableStockEntryMaterials,
} from './material-receipts.js'

const materials = [
  { id: 1, code: 'RAW-ETH-001', name: '埃塞俄比亚 水洗 耶加雪菲', kind: 'bean' },
  { id: 2, code: 'RAW-COL-002', name: '哥伦比亚 蕙兰', Kind: 'bean' },
  { id: 3, code: 'BAG-454', name: '454g 豆袋', kind: 'pack' },
  { id: 4, code: 'WIP-ROASTED', name: '烘焙熟豆', kind: 'bean', is_semi_finished: true },
  { id: 5, code: 'WIP-COUNT', name: '计数半成品', kind: 'other', IsSemiFinished: true },
]

test('selectableReceiptMaterials excludes packaging and semi-finished materials from raw receipt choices', () => {
  const got = selectableReceiptMaterials(materials)

  assert.deepEqual(got.map((row) => row.id), [1, 2])
  assert.equal(isSemiFinishedMaterial(materials[3]), true)
  assert.equal(isSemiFinishedMaterial(materials[4]), true)
})

test('filterReceiptMaterials fuzzy matches by material name and code', () => {
  assert.deepEqual(filterReceiptMaterials(materials, '哥伦').map((row) => row.id), [2])
  assert.deepEqual(filterReceiptMaterials(materials, 'raw eth').map((row) => row.id), [1])
  assert.deepEqual(filterReceiptMaterials(materials, '454').map((row) => row.id), [])
  assert.deepEqual(filterReceiptMaterials(materials, '熟豆').map((row) => row.id), [])
})

test('ordinary stock receipt excludes semi-finished materials without hiding them from manufacturing movements', () => {
  assert.deepEqual(selectableStockEntryMaterials(materials, 'material_receipt').map((row) => row.id), [1, 2, 3])
  assert.deepEqual(selectableStockEntryMaterials(materials, 'material_transfer_for_manufacture').map((row) => row.id), [1, 2, 3, 4, 5])
})
