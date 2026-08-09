import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

import { filterSearchableOptions, optionSearchText } from './searchable-select.js'

const here = dirname(fileURLToPath(import.meta.url))

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

test('filterSearchableOptions supports customer fulfillment picker fields', () => {
  const rows = [
    { product_id: 88, product_name: '誉观山冷萃豆', sku_code: 'YGS-LC-100', spec: '100g' },
    { item_id: 19, item_name: '埃塞花魁', item_type: 'raw_bean' },
    { employee_id: 23, name: '誉观山客户', phone: '13800138000', department: '代加工' },
    { receiver_name: '张三', receiver_phone: '13900139000', receiver_address: '浙江省杭州市西湖区文三路 10 号' },
  ]

  assert.deepEqual(filterSearchableOptions(rows, 'YGS').map((row) => row.product_id), [88])
  assert.deepEqual(filterSearchableOptions(rows, '花魁').map((row) => row.item_id), [19])
  assert.deepEqual(filterSearchableOptions(rows, '23').map((row) => row.employee_id), [23])
  assert.deepEqual(filterSearchableOptions(rows, '西湖 文三').map((row) => row.receiver_name), ['张三'])
})

test('workspace customer selector keeps clear and dropdown controls in separate hit targets', () => {
  const searchableSelect = readFileSync(resolve(here, '../components/SearchableSelect.vue'), 'utf8')
  const app = readFileSync(resolve(here, '../App.vue'), 'utf8')

  assert.match(searchableSelect, /class="select-clear"/)
  assert.match(searchableSelect, /aria-label="清除选择"/)
  assert.match(searchableSelect, /type="text"/)
  assert.match(searchableSelect, /\.select-clear[\s\S]*right:\s*36px/)
  assert.match(searchableSelect, /padding:\s*7px\s+70px\s+7px\s+9px/)
  assert.match(app, /workspace-customer[\s\S]*select-control input[\s\S]*padding:\s*6px\s+70px\s+6px\s+8px/)
})

test('searchable select supports optional menu header and option slots without removing default rendering', () => {
  const source = readFileSync(resolve(here, '../components/SearchableSelect.vue'), 'utf8')

  assert.match(source, /<slot\s+name="menu-header"\s*\/>/)
  assert.match(source, /<slot\s+name="option"[^>]*:option="option"[^>]*:label="labelOf\(option\)"[^>]*:meta="metaOf\(option\)"[^>]*>/)
  assert.match(source, /<strong>\{\{ labelOf\(option\) \}\}<\/strong>/)
  assert.match(source, /<small v-if="metaOf\(option\)">\{\{ metaOf\(option\) \}\}<\/small>/)
})
