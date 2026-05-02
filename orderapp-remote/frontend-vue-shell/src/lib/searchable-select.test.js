import test from 'node:test'
import assert from 'node:assert/strict'

import { filterSearchableOptions, optionSearchText } from './searchable-select.js'

const rows = [
  { id: 1, code: 'RAW-ETH-001', name: '埃塞俄比亚 水洗 耶加雪菲' },
  { id: 2, code: 'RAW-COL-002', name: '哥伦比亚 蕙兰' },
  { id: 3, code: 'FP-454', name: '乌拉嘎 454g' },
]

test('optionSearchText combines labels, names, codes, and aliases for one in-dropdown search field', () => {
  const got = optionSearchText(rows[0], '耶加雪菲 (RAW-ETH-001)')

  assert.match(got, /耶加雪菲/)
  assert.match(got, /raw-eth-001/)
})

test('filterSearchableOptions matches by multiple typed terms without a separate search input', () => {
  assert.deepEqual(filterSearchableOptions(rows, 'raw eth').map((row) => row.id), [1])
  assert.deepEqual(filterSearchableOptions(rows, '乌拉 454').map((row) => row.id), [3])
  assert.deepEqual(filterSearchableOptions(rows, '不存在').map((row) => row.id), [])
})
