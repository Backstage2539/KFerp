import test from 'node:test'
import assert from 'node:assert/strict'
import { staffPriceTableGroups, createOrderCatalogSession } from './order-catalog.js'

test('staff catalog groups both price styles as manual alternatives within one category', () => {
  const options = [
    { id: 10, list_type: 'commercial', classification_template_id: 56, classification_template_name: '新商品类型', is_default: true },
    { id: 11, list_type: 'retail', classification_template_id: 56 },
  ]
  const groups = staffPriceTableGroups(options, [11])
  assert.equal(groups.length, 1)
  assert.equal(groups[0].label, '新商品类型')
  assert.equal(groups[0].listType, 'retail')
  assert.deepEqual(groups[0].options.map(row => row.id), [10, 11])
})

test('catalog refresh ignores an old customer response and fails closed after a read error', async () => {
  const pending = []
  const state = {}
  const session = createOrderCatalogSession({ state, load: (customerID, ids) => new Promise((resolve,reject) => pending.push({ customerID,ids,resolve,reject })) })
  const first = session.refresh(487, [10])
  const second = session.refresh(363, [11])
  pending[0].resolve({ customer_id: 487, product_families: [{ name: '错误客户商品' }] })
  assert.equal(await first, null)
  assert.deepEqual(state.families, [])
  pending[1].resolve({ customer_id: 363, product_families: [{ name: '当前商品' }], selected_price_table_ids: [11], price_table_options: [] })
  assert.equal((await second).customer_id, 363)
  assert.equal(state.families[0].name, '当前商品')
  const failed = session.refresh(363, [11])
  pending[2].reject(Error('目录读取失败'))
  assert.equal(await failed, null)
  assert.deepEqual(state.families, [])
  assert.equal(state.error, '目录读取失败')
  assert.equal(state.loading, false)
})
