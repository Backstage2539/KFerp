import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildMaterialCreatePayload,
  filterMaterialsByOwnership,
  materialBelongsToCatalogContext,
  materialCustomerNames,
} from './material-customer-references.js'

test('material customer links are explicit and may include multiple customers', () => {
  const refs = [
    { material_id: 10, customer_id: 74, active: true },
    { material_id: 10, customer_id: 75, active: true },
    { material_id: 11, customer_id: 74, active: false },
  ]
  assert.equal(materialBelongsToCatalogContext({ id: 10 }, 0, refs), true)
  assert.equal(materialBelongsToCatalogContext({ id: 10 }, 74, refs), true)
  assert.equal(materialBelongsToCatalogContext({ id: 10 }, 75, refs), true)
  assert.equal(materialBelongsToCatalogContext({ id: 10 }, 76, refs), false)
  assert.equal(materialBelongsToCatalogContext({ id: 11 }, 74, refs), false)
  assert.equal(materialCustomerNames({ id: 10 }, refs, [{ id: 74, name: '芬纳' }, { id: 75, name: '客户B' }]), '芬纳、客户B')
})

test('material ownership filter supports factory and fuzzy customer names', () => {
  const rows = [{ id: 10 }, { id: 11 }, { id: 12 }]
  const references = [
    { material_id: 10, customer_id: 74, active: true },
    { material_id: 11, customer_id: 75, active: true },
  ]
  const customers = [{ id: 74, name: '芬纳咖啡' }, { id: 75, name: '另一客户' }]

  assert.deepEqual(filterMaterialsByOwnership(rows, 'factory', references, customers).map((row) => row.id), [12])
  assert.deepEqual(filterMaterialsByOwnership(rows, 'customer:74', references, customers).map((row) => row.id), [10])
  assert.deepEqual(filterMaterialsByOwnership(rows, '芬纳', references, customers).map((row) => row.id), [10])
})

test('materials view uses ownership wording and copy-to-customer link', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/MaterialsView.vue', import.meta.url), 'utf8')
  const template = source.split('<script setup>')[0] || source
  assert.match(template, /<span>物料归属<\/span>/)
  assert.match(template, /<th>物料归属<\/th>/)
  assert.match(template, />复制到客户<\/button>/)
  assert.doesNotMatch(template, /<span>客户关联<\/span>|<th>客户关联<\/th>|>客户关联<\/button>/)
})

test('material create payload sends selected customer ids independently of view context', () => {
  assert.deepEqual(buildMaterialCreatePayload({ code: 'MAT-A', name: '物料A' }, 'factory', [74]), {
    code: 'MAT-A',
    name: '物料A',
    customer_ids: [],
  })
  assert.deepEqual(buildMaterialCreatePayload({ code: 'MAT-B', name: '物料B' }, 'customers', [74, 75, 74]), {
    code: 'MAT-B',
    name: '物料B',
    customer_ids: [74, 75],
  })
})
