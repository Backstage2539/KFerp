import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildMaterialCreatePayload,
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
