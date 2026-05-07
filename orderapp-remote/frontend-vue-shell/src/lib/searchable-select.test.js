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

test('filterSearchableOptions supports customer custom SKU dropdown fields', () => {
  const customers = [
    { id: 1, name: '棵凡咖啡馆', company_name: '昆明棵凡咖啡有限公司', contact: '王店长', phone: '13800138000' },
    { id: 2, name: '山城烘焙', company_name: '重庆山城食品', contact: '李经理', phone: '13900139000' },
  ]
  const products = [
    { id: 7, number: 12, name: '暖阳拼配' },
    { id: 8, number: 13, name: '日晒瑰夏' },
  ]

  assert.deepEqual(filterSearchableOptions(customers, '王店长').map((row) => row.id), [1])
  assert.deepEqual(filterSearchableOptions(customers, '重庆 食品').map((row) => row.id), [2])
  assert.deepEqual(filterSearchableOptions(customers, '1390').map((row) => row.id), [2])
  assert.deepEqual(filterSearchableOptions(products, '12').map((row) => row.id), [7])
  assert.deepEqual(filterSearchableOptions(products, '暖 拼').map((row) => row.id), [7])
})
